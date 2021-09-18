package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	service_mgr "github.com/meshplus/bitxhub-core/service-mgr"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

var _ adapt.Adapt = (*BxhAdapter)(nil)

// BxhAdapter represents the necessary data for sync tx from bitxhub
type BxhAdapter struct {
	client          rpcx.Client
	logger          logrus.FieldLogger
	wrappersC       chan *pb.InterchainTxWrappers
	ibtpC           chan *pb.IBTP
	recoverHandler  RecoverUnionHandler
	rollbackHandler RollbackHandler

	mode       string
	isRecover  bool
	pierID     string
	appchainID string
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates instance of WrapperSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(pierID, appchainID string, mode string, opts ...Option) (*BxhAdapter, error) {
	cfg, err := GenerateConfig(opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ba := &BxhAdapter{
		wrappersC:  make(chan *pb.InterchainTxWrappers, maxChSize),
		ibtpC:      make(chan *pb.IBTP, maxChSize),
		client:     cfg.client,
		logger:     cfg.logger,
		ctx:        ctx,
		cancel:     cancel,
		mode:       mode,
		pierID:     pierID,
		appchainID: appchainID,
	}

	return ba, nil
}

func (b *BxhAdapter) Start() error {
	go b.run()
	go b.listenInterchainTxWrappers()

	b.logger.Info("BxhAdapter started")

	return nil
}

func (b *BxhAdapter) Stop() error {
	b.cancel()
	b.logger.Info("BxhAdapter stopped")
	return nil
}

func (b *BxhAdapter) MonitorIBTP() chan *pb.IBTP {
	return b.ibtpC
}

func (b *BxhAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	queryTx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetIBTPByID", rpcx.String(id), rpcx.Bool(isReq))
	if err != nil {
		return nil, err
	}
	queryTx.Nonce = 1
	receipt, err := b.client.SendView(queryTx)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	if !receipt.IsSuccess() {
		return nil, err
	}

	hash := types.NewHash(receipt.Ret)
	response, err := b.client.GetTransaction(hash.String())
	if err != nil {
		return nil, err
	}

	retIBTP := response.Tx.GetIBTP()
	var retSign *pb.SignResponse
	if isReq {
		retSign, err = b.client.GetMultiSigns(retIBTP.ID(), pb.GetMultiSignsRequest_IBTP_REQUEST)
		if err != nil {
			return nil, err
		}

	} else {
		retSign, err = b.client.GetMultiSigns(retIBTP.ID(), pb.GetMultiSignsRequest_IBTP_RESPONSE)
		if err != nil {
			return nil, err
		}

	}
	if retSign == nil || retSign.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for ibtp %s", retIBTP.ID())
	}

	var signs [][]byte
	for _, sign := range retSign.Sign {
		signs = append(signs, sign)
	}

	retStatus, err := b.getTxStatus(retIBTP.ID())
	if err != nil {
		return nil, err
	}

	proof := &pb.BxhProof{
		TxStatus:  retStatus,
		MultiSign: signs,
	}

	retProof, err := proof.Marshal()
	if err != nil {
		return nil, err
	}
	// get ibtp proof from bxh
	retIBTP.Proof = retProof

	return retIBTP, nil
}

func (b *BxhAdapter) SendIBTP(ibtp *pb.IBTP) error {
	proof := ibtp.GetProof()
	proofHash := sha256.Sum256(proof)
	ibtp.Proof = proofHash[:]

	tx, _ := b.client.GenerateIBTPTx(ibtp)
	tx.Extra = proof

	var (
		receipt *pb.Receipt
		retErr  error
		count   = uint64(0)
	)
	if err := retry.Retry(func(attempt uint) error {
		hash, err := b.client.SendTransaction(tx, nil)
		if err != nil {
			b.logger.Errorf("Send ibtp error: %s", err.Error())
			if errors.Is(err, rpcx.ErrRecoverable) {
				count++
				if count == 5 && ibtp.Type == pb.IBTP_INTERCHAIN {
					retErr = fmt.Errorf("rollback ibtp %s: %v", ibtp.ID(), err)
					return nil
				}
				return err
			}
			if errors.Is(err, rpcx.ErrReconstruct) {
				tx, _ = b.client.GenerateIBTPTx(ibtp)
				return err
			}
		}
		receipt, err = b.client.GetReceipt(hash)
		if err != nil {
			return fmt.Errorf("get tx receipt by hash %s: %w", hash, err)
		}
		if !receipt.IsSuccess() {
			b.logger.WithFields(logrus.Fields{
				"ibtp_id":   ibtp.ID(),
				"ibtp_type": ibtp.Type,
				"msg":       string(receipt.Ret),
			}).Error("Receipt result for ibtp")
			// if no rule bind for this appchain or appchain not available, exit pier
			errMsg := string(receipt.Ret)
			if strings.Contains(errMsg, noBindRule) ||
				strings.Contains(errMsg, CurAppchainNotAvailable) ||
				strings.Contains(errMsg, CurServiceNotAvailable) {
				return fmt.Errorf("retry sending IBTP: %s", errMsg)
			}

			// if target chain is not available, this ibtp should be rollback
			if strings.Contains(errMsg, InvalidTargetService) {
				b.logger.Errorf("%s, try to rollback in source appchain...", errMsg)
				b.rollbackHandler(ibtp, "")
				return nil
			}

			// if target chain is not available, this ibtp should be rollback
			if strings.Contains(errMsg, TargetAppchainNotAvailable) ||
				strings.Contains(errMsg, TargetServiceNotAvailable) ||
				strings.Contains(errMsg, TargetBitXHubNotAvailable) {
				retErr = fmt.Errorf("rollback ibtp %s: %s", ibtp.ID(), errMsg)
				return nil
			}
			if strings.Contains(errMsg, ibtpIndexExist) {
				// if ibtp index is lower than index recorded on bitxhub, then ignore this ibtp
				return nil
			}
			if strings.Contains(errMsg, ibtpIndexWrong) {
				// if index is wrong ,notify exchanger to update its meta from bitxhub
				retErr = ErrMetaOutOfDate
				return nil
			}
			if strings.Contains(errMsg, invalidIBTP) {
				// if this ibtp structure is not compatible or verify failed
				// try to get new ibtp and resend
				return fmt.Errorf("invalid ibtp %s", ibtp.ID())
			}
			if strings.Contains(errMsg, "has been rollback") {
				b.logger.WithField("id", ibtp.ID()).Warnf("Tx has been rollback")
				retErr = fmt.Errorf("rollback ibtp %s", ibtp.ID())
				return nil
			}
			return fmt.Errorf("unknown error, retry for %s anyway", ibtp.ID())
		}

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		return err
	}
	return retErr
}

func (b *BxhAdapter) GetServiceIDList() ([]string, error) {
	tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.ServiceMgrContractAddr.Address(),
		"GetAllServices")
	if err != nil {
		panic(err)
	}

	ret := getTxView(b.client, tx)

	services := make([]*service_mgr.Service, 0)
	if err := json.Unmarshal(ret, &services); err != nil {
		panic(err)
	}

	ids := make([]string, 0)

	bxhID, err := b.client.GetChainID()
	if err != nil {
		return nil, err
	}

	for _, service := range services {
		ids = append(ids, fmt.Sprintf("%d:%s:%s", bxhID, service.ChainID, service.ServiceID))
	}

	return ids, nil
}

func (b *BxhAdapter) QueryInterchain(serviceID string) (*pb.Interchain, error) {
	tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetInterchain", rpcx.String(serviceID))
	if err != nil {
		return nil, err
	}

	ret := getTxView(b.client, tx)
	if len(ret) == 0 {
		return &pb.Interchain{
			ID:                      serviceID,
			InterchainCounter:       make(map[string]uint64),
			ReceiptCounter:          make(map[string]uint64),
			SourceInterchainCounter: make(map[string]uint64),
			SourceReceiptCounter:    make(map[string]uint64),
		}, nil
	}

	interchain := &pb.Interchain{}
	if err := interchain.Unmarshal(ret); err != nil {
		return nil, err
	}

	return interchain, nil
}

//  move interchainWrapper into wrappers channel
func (b *BxhAdapter) run() {
	var (
		err           error
		subscriptType pb.SubscriptionRequest_Type
		rawCh         <-chan interface{}
	)
	if b.mode == repo.UnionMode {
		subscriptType = pb.SubscriptionRequest_UNION_INTERCHAIN_TX_WRAPPER
	} else {
		subscriptType = pb.SubscriptionRequest_INTERCHAIN_TX_WRAPPER
	}
	// retry for network reason
	if err := retry.Retry(func(attempt uint) error {
		rawCh, err = b.client.Subscribe(b.ctx, subscriptType, []byte(b.appchainID))
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case <-b.ctx.Done():
				return
			case h, ok := <-rawCh:
				if !ok {
					b.logger.Warn("Unexpected closed channel while syncing interchain tx wrapper")
					return
				}
				b.wrappersC <- h.(*pb.InterchainTxWrappers)
			}
		}
	}()
}

// listenInterchainTxWrappers listen on the wrapper channel for handling
func (b *BxhAdapter) listenInterchainTxWrappers() {
	for {
		select {
		case wrappers := <-b.wrappersC:
			if len(wrappers.InterchainTxWrappers) == 0 {
				b.logger.WithField("interchain_tx_wrappers", 0).Errorf("InterchainTxWrappers")
				continue
			}
			w := wrappers.InterchainTxWrappers[0]
			if w == nil {
				b.logger.Errorf("InterchainTxWrapper is nil")
				continue
			}
			for i, wrapper := range wrappers.GetInterchainTxWrappers() {
				ok := b.handleInterchainTxWrapper(wrapper, i)
				if !ok {
					return
				}
			}
		case <-b.ctx.Done():
			return
		}
	}
}

// handleInterchainTxWrapper is the handler for interchain tx wrapper
func (b *BxhAdapter) handleInterchainTxWrapper(w *pb.InterchainTxWrapper, i int) bool {
	if w == nil {
		b.logger.Error("empty interchain tx wrapper")
		return false
	}

	for _, ibtpId := range w.TimeoutIbtps {
		b.rollbackHandler(nil, ibtpId)
	}

	for _, tx := range w.Transactions {
		// if ibtp is failed
		// 1. this is interchain type of ibtp, increase inCounter index
		// 2. this is ibtp receipt type, rollback and increase callback index
		ibtp := tx.Tx.GetIBTP()
		if ibtp == nil {
			b.logger.Errorf("empty ibtp in tx")
			continue
		}
		b.logger.WithFields(logrus.Fields{
			"ibtp_id": ibtp.ID(),
			"type":    ibtp.Type,
		}).Debugf("Sync IBTP from bitxhub")
		b.ibtpC <- ibtp
	}

	b.logger.WithFields(logrus.Fields{
		"height":      w.Height,
		"count":       len(w.Transactions),
		"index":       i,
		"timeout IDs": w.TimeoutIbtps,
	}).Info("Handle interchain tx wrapper")
	return true
}

func (b *BxhAdapter) getTxStatus(id string) (pb.TransactionStatus, error) {
	receipt, err := b.client.InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, rpcx.String(id))
	if err != nil {
		return pb.TransactionStatus_BEGIN, err
	}

	if !receipt.IsSuccess() {
		return pb.TransactionStatus_BEGIN, fmt.Errorf("receipt: %s", receipt.Ret)
	}

	status, err := strconv.Atoi(string(receipt.Ret))
	if err != nil {
		return pb.TransactionStatus_BEGIN, err
	}

	return pb.TransactionStatus(status), nil
}
