package bxh_adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

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
)

var (
	_                adapt.Adapt = (*BxhAdapter)(nil)
	ErrIBTPNotFound              = fmt.Errorf("receipt from bitxhub failed")
	ErrMetaOutOfDate             = fmt.Errorf("interchain meta is out of date")
)

const maxChSize = 1 << 10

// BxhAdapter represents the necessary data for sync tx from bitxhub
type BxhAdapter struct {
	client    rpcx.Client
	logger    logrus.FieldLogger
	wrappersC chan *pb.InterchainTxWrappers
	ibtpC     chan *pb.IBTP

	mode       string
	pierID     string
	appchainID string
	ctx        context.Context
	cancel     context.CancelFunc
}

func (b *BxhAdapter) SendIBTP(ibtp *pb.IBTP) error {
	panic("implement me")
}

func (b *BxhAdapter) Name() string {
	bxhId, err := b.client.GetChainID()
	if err != nil {
		return ""
	}
	return strconv.Itoa(int(bxhId))
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
	proof, err := b.getMultiSign(retIBTP, isReq)
	if err != nil {
		return nil, err
	}
	// get ibtp proof from bxh
	retIBTP.Proof = proof

	return retIBTP, nil
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
	//todo how to solve timeoutIbtp
	//for _, ibtpId := range w.TimeoutIbtps {
	//	b.rollbackHandler(nil, ibtpId)
	//}

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

		proof, err := b.getMultiSign(ibtp, false)
		if err != nil {
			return false
		}
		ibtp.Proof = proof
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
	var receipt *pb.Receipt
	// if query fail from BVMContract, retry
	if err := retry.Retry(func(attempt uint) error {
		bxhReceipt, err := b.client.InvokeBVMContract(constant.TransactionMgrContractAddr.Address(),
			"GetStatus", nil, rpcx.String(id))
		if err != nil {
			return err
		}
		receipt = bxhReceipt
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		b.logger.WithFields(logrus.Fields{
			"id":    id,
			"error": err,
		}).Errorf("Retry to get tx status")
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

func (b *BxhAdapter) getMultiSign(ibtp *pb.IBTP, isReq bool) ([]byte, error) {
	var (
		err     error
		retSign *pb.SignResponse
	)
	if isReq {
		retSign, err = b.client.GetMultiSigns(ibtp.ID(), pb.GetMultiSignsRequest_IBTP_REQUEST)
		if err != nil {
			return nil, err
		}
	} else {
		retSign, err = b.client.GetMultiSigns(ibtp.ID(), pb.GetMultiSignsRequest_IBTP_RESPONSE)
		if err != nil {
			return nil, err
		}

	}
	if retSign == nil || retSign.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for ibtp %s", ibtp.ID())
	}

	var signs [][]byte
	for _, sign := range retSign.Sign {
		signs = append(signs, sign)
	}

	retStatus, err := b.getTxStatus(ibtp.ID())
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
	return retProof, nil
}
