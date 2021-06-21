package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meshplus/pier/pkg/model"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/cbergoon/merkletree"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

var _ Syncer = (*WrapperSyncer)(nil)

const maxChSize = 1 << 10

// WrapperSyncer represents the necessary data for sync tx wrappers from bitxhub
type WrapperSyncer struct {
	client          rpcx.Client
	lite            lite.Lite
	storage         storage.Storage
	logger          logrus.FieldLogger
	wrappersC       chan *pb.InterchainTxWrappers
	ibtpC           chan *model.WrappedIBTP
	burnEvent       chan *pb.UnLock
	appchainHandler AppchainHandler
	recoverHandler  RecoverUnionHandler
	rollbackHandler RollbackHandler
	jsonrpcClient   *Client

	mode        string
	isRecover   bool
	height      uint64
	pierID      string
	appchainDID string
	ctx         context.Context
	cancel      context.CancelFunc
}

func (syncer *WrapperSyncer) QueryBurnEventByIndex(index int64) *pb.UnLock {
	var unLock *pb.UnLock
	syncer.retryFunc(func(attempt uint) error {
		unLock = syncer.jsonrpcClient.QueryBurnEventByIndex(index)
		signs, err := syncer.GetEVMSigns(unLock.TxId)
		if err != nil {
			syncer.logger.WithFields(logrus.Fields{
				"burn txid": unLock.TxId,
			}).Error("Syncer GetEVMSigns")
			return err
		}
		unLock.MultiSigns = signs
		return nil
	})
	return unLock
}

func (syncer *WrapperSyncer) JsonrpcClient() *Client {
	return syncer.jsonrpcClient
}

type SubscriptionKey struct {
	PierID      string `json:"pier_id"`
	AppchainDID string `json:"appchain_did"`
}

// New creates instance of WrapperSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(pierID, appchainDID string, mode string, opts ...Option) (*WrapperSyncer, error) {
	cfg, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}

	ws := &WrapperSyncer{
		wrappersC:     make(chan *pb.InterchainTxWrappers, maxChSize),
		ibtpC:         make(chan *model.WrappedIBTP, maxChSize),
		burnEvent:     make(chan *pb.UnLock, maxChSize),
		client:        cfg.client,
		lite:          cfg.lite,
		storage:       cfg.storage,
		logger:        cfg.logger,
		jsonrpcClient: cfg.ethClient,
		mode:          mode,
		pierID:        pierID,
		appchainDID:   appchainDID,
	}

	return ws, nil
}

// Start implements Syncer
func (syncer *WrapperSyncer) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	syncer.ctx = ctx
	syncer.cancel = cancel

	meta, err := syncer.client.GetChainMeta()
	if err != nil {
		return fmt.Errorf("get chain meta from bitxhub: %w", err)
	}

	// recover the block height which has latest unfinished interchain tx
	height, err := syncer.getLastHeight()
	if err != nil {
		return fmt.Errorf("get last height: %w", err)
	}
	syncer.height = height

	if meta.Height > height {
		syncer.recover(syncer.getDemandHeight(), meta.Height)
	}

	go syncer.syncInterchainTxWrappers()
	go syncer.listenInterchainTxWrappers()
	go syncer.syncBurnEvent()

	syncer.logger.WithFields(logrus.Fields{
		"current_height": syncer.height,
		"bitxhub_height": meta.Height,
	}).Info("Syncer started")

	return nil
}

// recover will recover those missing merkle wrapper when pier is down
func (syncer *WrapperSyncer) recover(begin, end uint64) {
	syncer.isRecover = true
	defer func() {
		syncer.isRecover = false
	}()

	icm := make(map[string]*rpcx.Interchain, 0)

	syncer.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Syncer recover")

	if syncer.isUnionMode() {
		if err := syncer.appchainHandler(); err != nil {
			syncer.logger.WithField("err", err).Errorf("Router handle")
		}
	}

	ch := make(chan *pb.InterchainTxWrappers, maxChSize)
	if err := syncer.client.GetInterchainTxWrappers(syncer.ctx, syncer.appchainDID, begin, end, ch); err != nil {
		syncer.logger.WithFields(logrus.Fields{"begin": begin, "end": end, "error": err}).Warn("get interchain tx wrapper")
	}

	for wrappers := range ch {
		syncer.handleInterchainWrapperAndPersist(wrappers, icm)
	}
}

// Stop implements Syncer
func (syncer *WrapperSyncer) Stop() error {
	if syncer.cancel != nil {
		syncer.cancel()
	}

	syncer.logger.Info("Syncer stopped")

	return nil
}

// syncInterchainTxWrappers queries to bitxhub and syncs confirmed interchain txs
// whose destination is the same as AppchainDID.
// Note: only interchain txs generated after the connection to bitxhub
// being established will be sent to syncer
func (syncer *WrapperSyncer) syncInterchainTxWrappers() {
	loop := func(ch <-chan *pb.InterchainTxWrappers) {
		for {
			select {
			case wrappers, ok := <-ch:
				if !ok {
					syncer.logger.Warn("Unexpected closed channel while syncing interchain tx wrapper")
					return
				}

				syncer.wrappersC <- wrappers
			case <-syncer.ctx.Done():
				return
			}
		}

	}

	for {
		select {
		case <-syncer.ctx.Done():
			return
		default:
			ch := syncer.getWrappersChannel()

			err := retry.Retry(func(attempt uint) error {
				chainMeta, err := syncer.client.GetChainMeta()
				if err != nil {
					syncer.logger.WithField("error", err).Error("Get chain meta")
					return err
				}

				if chainMeta.Height > syncer.height {
					syncer.recover(syncer.getDemandHeight(), chainMeta.Height)
				}

				return nil
			}, strategy.Wait(1*time.Second))

			if err != nil {
				syncer.logger.Panic(err)
			}

			loop(ch)
		}
	}
}

// listen burn event from bitxhub
func (syncer *WrapperSyncer) syncBurnEvent() {
	loop := func(ch <-chan *pb.UnLock) {
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					syncer.logger.Warn("Unexpected closed channel while syncing interchain tx wrapper")
					return
				}
				syncer.burnEvent <- event
			case <-syncer.ctx.Done():
				return
			}
		}

	}
	for {
		select {
		case <-syncer.ctx.Done():
			return
		default:
			ch := syncer.jsonrpcClient.burnCh
			loop(ch)
		}
	}
}

// getWrappersChannel gets a syncing merkle wrapper channel
func (syncer *WrapperSyncer) getWrappersChannel() chan *pb.InterchainTxWrappers {
	var (
		err           error
		subscriptType pb.SubscriptionRequest_Type
		rawCh         <-chan interface{}
	)
	if syncer.mode == repo.UnionMode {
		subscriptType = pb.SubscriptionRequest_UNION_INTERCHAIN_TX_WRAPPER
	} else {
		subscriptType = pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER
	}
	// retry for network reason
	subKey := &SubscriptionKey{syncer.pierID, syncer.appchainDID}
	subKeyData, _ := json.Marshal(subKey)
	if err := retry.Retry(func(attempt uint) error {
		rawCh, err = syncer.client.Subscribe(syncer.ctx, subscriptType, subKeyData)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		panic(err)
	}

	// move interchainWrapper into buffered channel
	ch := make(chan *pb.InterchainTxWrappers, maxChSize)
	go func() {
		for {
			select {
			case <-syncer.ctx.Done():
				return
			case h, ok := <-rawCh:
				if !ok {
					close(ch)
					return
				}
				ch <- h.(*pb.InterchainTxWrappers)
			}
		}
	}()
	return ch
}

// listenInterchainTxWrappers listen on the wrapper channel for handling
func (syncer *WrapperSyncer) listenInterchainTxWrappers() {
	for {
		select {
		case wrappers := <-syncer.wrappersC:
			if syncer.isUnionMode() {
				if err := syncer.appchainHandler(); err != nil {
					syncer.logger.WithField("err", err).Errorf("Router handle")
				}
			}

			if len(wrappers.InterchainTxWrappers) == 0 {
				syncer.logger.WithField("interchain_tx_wrappers", 0).Errorf("InterchainTxWrappers")
				continue
			}
			w := wrappers.InterchainTxWrappers[0]
			if w == nil {
				syncer.logger.Errorf("InterchainTxWrapper is nil")
				continue
			}
			if w.Height < syncer.getDemandHeight() {
				syncer.logger.WithField("height", w.Height).Warn("Discard wrong wrapper")
				continue
			}

			if w.Height > syncer.getDemandHeight() {
				syncer.logger.WithFields(logrus.Fields{
					"begin": syncer.height,
					"end":   w.Height,
				}).Info("Get interchain tx wrapper")

				ch := make(chan *pb.InterchainTxWrappers, maxChSize)
				if err := syncer.client.GetInterchainTxWrappers(syncer.ctx, syncer.appchainDID, syncer.getDemandHeight(), w.Height, ch); err != nil {
					syncer.logger.WithFields(logrus.Fields{"begin": syncer.height, "end": w.Height, "error": err}).Warn("Get interchain tx wrapper")
					continue
				}

				for ws := range ch {
					syncer.handleInterchainWrapperAndPersist(ws, nil)
				}
				continue
			}

			syncer.handleInterchainWrapperAndPersist(wrappers, nil)
		case <-syncer.ctx.Done():
			return
		}
	}
}

func (syncer *WrapperSyncer) handleInterchainWrapperAndPersist(ws *pb.InterchainTxWrappers, icm map[string]*rpcx.Interchain) {
	if ws == nil || ws.InterchainTxWrappers == nil {
		return
	}
	for i, wrapper := range ws.InterchainTxWrappers {
		ok := syncer.handleInterchainTxWrapper(wrapper, i, icm)
		if !ok {
			return
		}
	}
	if err := syncer.persist(ws); err != nil {
		syncer.logger.WithFields(logrus.Fields{"height": ws.InterchainTxWrappers[0].Height, "error": err}).Error("Persist interchain tx wrapper")
	}
	syncer.updateHeight()
}

// handleInterchainTxWrapper is the handler for interchain tx wrapper
func (syncer *WrapperSyncer) handleInterchainTxWrapper(w *pb.InterchainTxWrapper, i int, icm map[string]*rpcx.Interchain) bool {
	if w == nil {
		syncer.logger.WithField("height", syncer.height).Error("empty interchain tx wrapper")
		return false
	}

	if ok, err := syncer.verifyWrapper(w); !ok {
		syncer.logger.WithFields(logrus.Fields{"height": w.Height, "error": err}).Warn("Invalid wrapper")
		return false
	}

	for _, tx := range w.Transactions {
		// if ibtp is failed
		// 1. this is interchain type of ibtp, increase inCounter index
		// 2. this is ibtp receipt type, rollback and increase callback index
		ibtp := tx.Tx.GetIBTP()
		if ibtp == nil {
			syncer.logger.Errorf("empty ibtp in tx")
			continue
		}
		wIBTP := &model.WrappedIBTP{Ibtp: ibtp, IsValid: tx.Valid}
		if syncer.isRecover && syncer.isUnionMode() {
			ic, ok := icm[ibtp.From]
			if !ok {
				recoveredIc, err := syncer.recoverHandler(ibtp)
				if err != nil {
					syncer.logger.Error(err)
					continue
				}
				icm[ibtp.From] = recoveredIc
				ic = recoveredIc
			}
			if index, ok := ic.InterchainCounter[ibtp.To]; ok {
				if ibtp.Index <= index {
					continue
				}
			}
		}
		syncer.logger.WithFields(logrus.Fields{
			"ibtp_id": ibtp.ID(),
			"type":    ibtp.Type,
		}).Debugf("Sync IBTP from bitxhub")
		syncer.ibtpC <- wIBTP
	}

	syncer.logger.WithFields(logrus.Fields{
		"height": w.Height,
		"count":  len(w.Transactions),
		"index":  i,
	}).Info("Handle interchain tx wrapper")
	return true
}

func (syncer *WrapperSyncer) RegisterRecoverHandler(handleRecover RecoverUnionHandler) error {
	if handleRecover == nil {
		return fmt.Errorf("register recover handler: empty handler")
	}
	syncer.recoverHandler = handleRecover
	return nil
}

func (syncer *WrapperSyncer) RegisterAppchainHandler(handler AppchainHandler) error {
	if handler == nil {
		return fmt.Errorf("register router handler: empty handler")
	}

	syncer.appchainHandler = handler
	return nil
}

func (syncer *WrapperSyncer) RegisterRollbackHandler(handler RollbackHandler) error {
	if handler == nil {
		return fmt.Errorf("register rollback handler: empty handler")
	}

	syncer.rollbackHandler = handler
	return nil
}

// verifyWrapper verifies the basic of merkle wrapper from bitxhub
func (syncer *WrapperSyncer) verifyWrapper(w *pb.InterchainTxWrapper) (bool, error) {
	if w.Height != syncer.getDemandHeight() {
		return false, fmt.Errorf("wrong height of wrapper from bitxhub")
	}
	if w.Height == 1 {
		return true, nil
	}

	// validate if l2roots are correct
	l2RootHashes := make([]merkletree.Content, 0, len(w.L2Roots))
	for _, root := range w.L2Roots {
		l2root := root
		l2RootHashes = append(l2RootHashes, &l2root)
	}
	l1Tree, err := merkletree.NewTree(l2RootHashes)
	if err != nil {
		return false, fmt.Errorf("init l1 merkle tree: %w", err)
	}

	var header *pb.BlockHeader
	if err := retry.Retry(func(attempt uint) error {
		header, err = syncer.lite.QueryHeader(w.Height)
		if err != nil {
			syncer.logger.Warnf("query header: %s", err.Error())
			return err
		}

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	// verify tx root
	if types.NewHash(l1Tree.MerkleRoot()).String() != header.TxRoot.String() {
		return false, fmt.Errorf("tx wrapper merkle root is wrong")
	}

	// validate if the txs is committed in bitxhub
	if len(w.Transactions) == 0 {
		return true, nil
	}

	hashes := make([]merkletree.Content, 0, len(w.Transactions))
	for _, tx := range w.Transactions {
		hashes = append(hashes, tx)
	}

	tree, err := merkletree.NewTree(hashes)
	if err != nil {
		return false, fmt.Errorf("init merkle tree: %w", err)
	}

	l2root := types.NewHash(tree.MerkleRoot())
	correctRoot := false
	for _, rootHash := range w.L2Roots {
		if rootHash.String() == l2root.String() {
			correctRoot = true
			break
		}
	}
	if !correctRoot {
		return false, fmt.Errorf("incorrect trx hashes")
	}
	return true, nil
}

// getLastHeight gets the current working height of Syncer
func (syncer *WrapperSyncer) getLastHeight() (uint64, error) {
	v := syncer.storage.Get(syncHeightKey())
	if v == nil {
		return 0, nil
	}

	return strconv.ParseUint(string(v), 10, 64)
}

func syncHeightKey() []byte {
	return []byte("sync-height")
}

func (syncer *WrapperSyncer) getDemandHeight() uint64 {
	return atomic.LoadUint64(&syncer.height) + 1
}

// updateHeight updates sync height
func (syncer *WrapperSyncer) updateHeight() {
	atomic.AddUint64(&syncer.height, 1)
}

func (syncer *WrapperSyncer) isUnionMode() bool {
	return syncer.mode == repo.UnionMode
}
