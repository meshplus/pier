package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/cbergoon/merkletree"
	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/lite"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/utils"
	"github.com/meshplus/pier/pkg/model"
	"github.com/sirupsen/logrus"
)

var _ Syncer = (*WrapperSyncer2)(nil)

// WrapperSyncer2 represents the necessary data for sync tx wrappers from bitxhub
type WrapperSyncer2 struct {
	lite            lite.Lite
	storage         storage.Storage
	logger          logrus.FieldLogger
	wrappersC       chan *pb.InterchainTxWrappers
	ibtpC           chan *model.WrappedIBTP
	appchainHandler AppchainHandler
	recoverHandler  RecoverUnionHandler
	rollbackHandler RollbackHandler
	peerMgr         peermgr.PeerManager
	pool            *Pool
	priv            crypto.PrivateKey

	mode        string
	isRecover   bool
	height      uint64
	pierID      string
	appchainDID string
	ctx         context.Context
	cancel      context.CancelFunc
}

// New2 creates instance of WrapperSyncer2 given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New2(priv crypto.PrivateKey, pierID, appchainDID string, mode string, peerMgr peermgr.PeerManager, opts ...Option) (*WrapperSyncer2, error) {
	cfg, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}

	ws := &WrapperSyncer2{
		wrappersC:   make(chan *pb.InterchainTxWrappers, maxChSize),
		ibtpC:       make(chan *model.WrappedIBTP, maxChSize),
		peerMgr:     peerMgr,
		lite:        cfg.lite,
		storage:     cfg.storage,
		logger:      cfg.logger,
		mode:        mode,
		pierID:      pierID,
		appchainDID: appchainDID,
		priv:        priv,
	}

	return ws, nil
}

// Start implements Syncer
func (syncer *WrapperSyncer2) Start() error {
	err := syncer.peerMgr.RegisterMsgHandler(pb.Message_PIER_SUBSCRIBE_INTERCHAIN_TX_WRAPPERS_ACK, syncer.handleMessage)
	if err != nil {
		return fmt.Errorf("register subscribe interchainTxWrappers ack msg handler: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	syncer.ctx = ctx
	syncer.cancel = cancel

	msg := peermgr.Message(pb.Message_PIER_GET_CHAIN_META, true, nil)
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("syncer GetChainMeta marshal err: %w", err)
	}
	meta, err := syncer.peerMgr.SendByMultiAddr(data)
	if err != nil {
		return fmt.Errorf("%s, %w", err.Error(), utils.ErrBrokenNetwork)
	}
	chainMeta := &pb.ChainMeta{}
	err = chainMeta.Unmarshal(meta)
	if err != nil {
		return fmt.Errorf("umarshal chain meta from bitxhub: %w\n, err msg is:%s", err, string(meta))

	}
	// recover the block height which has latest unfinished interchain tx
	height, err := syncer.getLastHeight()
	if err != nil {
		return fmt.Errorf("get last height: %w", err)
	}
	syncer.height = height

	if chainMeta.Height > height {
		syncer.recover(syncer.getDemandHeight(), chainMeta.Height)
	}
	syncer.pool = NewPool(chainMeta.Height)

	go syncer.listenInterchainTxWrappers()

	syncer.logger.WithFields(logrus.Fields{
		"current_height": syncer.height,
		"bitxhub_height": chainMeta.Height,
	}).Info("Syncer started")

	return nil
}

// recover will recover those missing merkle wrapper when pier is down
func (syncer *WrapperSyncer2) recover(begin, end uint64) {
	syncer.isRecover = true
	defer func() {
		syncer.isRecover = false
	}()

	icm := make(map[string]*rpcx.Interchain, 0)

	syncer.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Start Syncer recover")

	if syncer.isUnionMode() {
		if err := syncer.appchainHandler(); err != nil {
			syncer.logger.WithField("err", err).Errorf("Router handle")
		}
	}

	requests := utils.GenerateBatchWrappersRequest(begin, end, syncer.appchainDID)
	responses := make([]*pb.MultiInterchainTxWrappers, len(requests))
	var wg sync.WaitGroup
	wg.Add(len(requests))
	for i, req := range requests {
		go func(i int, req *pb.GetInterchainTxWrappersRequest) {
			defer wg.Done()
			reqData, err := req.Marshal()
			if err != nil {
				syncer.logger.Errorf("GetInterchainTxWrappersRequest marshal err:%s", err)
			}
			msg := peermgr.Message(pb.Message_PIER_GET_INTERCHAIN_TX_WRAPPERS, true, reqData)
			data, err := msg.Marshal()
			if err != nil {
				syncer.logger.Errorf("syncer GetInterchainTxWrappers marshal err: %w", err)
			}
			resp, err := syncer.peerMgr.SendByMultiAddr(data)
			if err != nil {
				syncer.logger.WithFields(logrus.Fields{
					"begin": req.GetBegin(),
					"end":   req.GetEnd(),
					"error": err,
				}).Error("Get InterchainTxWrappers ")
			}
			multiWrappers := &pb.MultiInterchainTxWrappers{}
			err = multiWrappers.Unmarshal(resp)
			if err != nil {
				syncer.logger.Errorf("unmarshal syncer GetInterchainTxWrappers err: %w\n err msg is: %s", err, string(resp))
			}
			responses[i] = multiWrappers
		}(i, req)
	}
	wg.Wait()
	for _, multiWrappers := range responses {
		for _, wrappers := range multiWrappers.MultiWrappers {
			syncer.handleInterchainWrapperAndPersist(wrappers, icm)
		}
	}
	syncer.logger.WithFields(logrus.Fields{
		"begin": begin,
		"end":   end,
	}).Info("Finish Syncer recover")
}

// Stop implements Syncer
func (syncer *WrapperSyncer2) Stop() error {
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
func (syncer *WrapperSyncer2) syncInterchainTxWrappers() {
	var subscriptType pb.SubscriptionRequest_Type
	if syncer.mode == repo.UnionMode {
		subscriptType = pb.SubscriptionRequest_UNION_INTERCHAIN_TX_WRAPPER
	} else {
		subscriptType = pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER
	}
	// retry for network reason
	subKey := &SubscriptionKey{syncer.pierID, syncer.appchainDID}
	subKeyData, _ := json.Marshal(subKey)
	req := &pb.SubscriptionRequest{
		Type:  subscriptType,
		Extra: subKeyData,
	}
	if err := retry.Retry(func(attempt uint) error {
		reqData, err := req.Marshal()
		if err != nil {
			syncer.logger.Errorf("SubscriptionRequest marshal err: %w", err)
			return err
		}
		msg := peermgr.Message(pb.Message_PIER_SUBSCRIBE_INTERCHAIN_TX_WRAPPERS, true, reqData)
		msg.From = fmt.Sprintf("%s, %s", syncer.peerMgr.GetLocalAddr(), syncer.peerMgr.GetPangolinAddr())
		data, err := msg.Marshal()
		if err != nil {
			syncer.logger.Errorf("lite subscrbe marshal err: %w", err)
			return err
		}
		resp, err := syncer.peerMgr.SendByMultiAddr(data)
		if err != nil {
			syncer.logger.Errorf("syncer AsyncSendByMultiAddr subscrbe tx wrapper err: %v", err)
			return err
		}
		if !strings.Contains(string(resp), peermgr.SubscribeResponse) {
			err = fmt.Errorf("bitxhub is not recieve interchainWrappers subscribe request：%s", resp)
			syncer.logger.Errorf("syncer AsyncSendByMultiAddr subscrbe tx wrapper err: %v", err)
			return err
		}
		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}
}

//// getWrappersChannel gets a syncing merkle wrapper channel
//func (syncer *WrapperSyncer2) getWrappersChannel() chan *pb.InterchainTxWrappers {
//	var (
//		err           error
//		subscriptType pb.SubscriptionRequest_Type
//		rawCh         <-chan interface{}
//	)
//	if syncer.mode == repo.UnionMode {
//		subscriptType = pb.SubscriptionRequest_UNION_INTERCHAIN_TX_WRAPPER
//	} else {
//		subscriptType = pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER
//	}
//	// retry for network reason
//	subKey := &SubscriptionKey{syncer.pierID, syncer.appchainDID}
//	subKeyData, _ := json.Marshal(subKey)
//	if err := retry.Retry(func(attempt uint) error {
//		rawCh, err = syncer.client.Subscribe(syncer.ctx, subscriptType, subKeyData)
//		if err != nil {
//			return err
//		}
//		return nil
//	}, strategy.Wait(1*time.Second)); err != nil {
//		panic(err)
//	}
//
//	// move interchainWrapper into buffered channel
//	ch := make(chan *pb.InterchainTxWrappers, maxChSize)
//	go func() {
//		for {
//			select {
//			case <-syncer.ctx.Done():
//				return
//			case h, ok := <-rawCh:
//				if !ok {
//					close(ch)
//					return
//				}
//				ch <- h.(*pb.InterchainTxWrappers)
//			}
//		}
//	}()
//	return ch
//}

// listenInterchainTxWrappers listen on the wrapper channel for handling
func (syncer *WrapperSyncer2) listenInterchainTxWrappers() {
	syncer.syncInterchainTxWrappers()
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
			syncer.handleInterchainWrapperAndPersist(wrappers, nil)
		case <-syncer.ctx.Done():
			close(syncer.wrappersC)
			return
		}
	}
}

func (syncer *WrapperSyncer2) handleInterchainWrapperAndPersist(ws *pb.InterchainTxWrappers, icm map[string]*rpcx.Interchain) {
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
func (syncer *WrapperSyncer2) handleInterchainTxWrapper(w *pb.InterchainTxWrapper, i int, icm map[string]*rpcx.Interchain) bool {
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

func (syncer *WrapperSyncer2) RegisterRecoverHandler(handleRecover RecoverUnionHandler) error {
	if handleRecover == nil {
		return fmt.Errorf("register recover handler: empty handler")
	}
	syncer.recoverHandler = handleRecover
	return nil
}

func (syncer *WrapperSyncer2) RegisterAppchainHandler(handler AppchainHandler) error {
	if handler == nil {
		return fmt.Errorf("register router handler: empty handler")
	}

	syncer.appchainHandler = handler
	return nil
}

func (syncer *WrapperSyncer2) RegisterRollbackHandler(handler RollbackHandler) error {
	if handler == nil {
		return fmt.Errorf("register rollback handler: empty handler")
	}

	syncer.rollbackHandler = handler
	return nil
}

// verifyWrapper verifies the basic of merkle wrapper from bitxhub
func (syncer *WrapperSyncer2) verifyWrapper(w *pb.InterchainTxWrapper) (bool, error) {
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
	}, strategy.Wait(5*time.Second)); err != nil {
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
func (syncer *WrapperSyncer2) getLastHeight() (uint64, error) {
	v := syncer.storage.Get(syncHeightKey())
	if v == nil {
		return 0, nil
	}

	return strconv.ParseUint(string(v), 10, 64)
}

func (syncer *WrapperSyncer2) getDemandHeight() uint64 {
	return atomic.LoadUint64(&syncer.height) + 1
}

// updateHeight updates sync height
func (syncer *WrapperSyncer2) updateHeight() {
	atomic.AddUint64(&syncer.height, 1)
}

func (syncer *WrapperSyncer2) isUnionMode() bool {
	return syncer.mode == repo.UnionMode
}
