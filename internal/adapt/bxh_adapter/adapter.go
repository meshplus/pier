package bxh_adapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	servicemgr "github.com/meshplus/bitxhub-core/service-mgr"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

var _ adapt.Adapt = (*BxhAdapter)(nil)

var (
	_                adapt.Adapt = (*BxhAdapter)(nil)
	ErrIBTPNotFound              = fmt.Errorf("receipt from bitxhub failed")
	ErrMetaOutOfDate             = fmt.Errorf("interchain meta is out of date")
)

const maxChSize = 1024

// BxhAdapter represents the necessary data for sync tx from bitxhub
type BxhAdapter struct {
	client    rpcx.Client
	logger    logrus.FieldLogger
	wrappersC chan *pb.InterchainTxWrappers
	ibtpC     chan *pb.IBTP

	mode       string
	appchainId string
	bxhID      uint64
	ctx        context.Context
	cancel     context.CancelFunc
	tss        *repo.TSS
}

func (b *BxhAdapter) MonitorUpdatedMeta() chan *[]byte {
	return nil
}

func (b *BxhAdapter) SendUpdatedMeta(byte []byte) error {
	return nil
}

// New creates instance of WrapperSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(mode, appchainId string, client rpcx.Client, logger logrus.FieldLogger, tss *repo.TSS) (*BxhAdapter, error) {
	bxhID, err := client.GetChainID()
	if err != nil {
		return nil, fmt.Errorf("new bxh adapter err: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	ba := &BxhAdapter{
		wrappersC:  make(chan *pb.InterchainTxWrappers, maxChSize),
		ibtpC:      make(chan *pb.IBTP, maxChSize),
		client:     client,
		appchainId: appchainId,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		mode:       mode,
		bxhID:      bxhID,
		tss:        tss,
	}

	return ba, nil
}

func (b *BxhAdapter) Start() error {
	if b.ibtpC == nil || b.wrappersC == nil {
		b.restartCh()
	}
	go b.run()
	go b.listenInterchainTxWrappers()

	b.logger.Info("BxhAdapter started")

	return nil
}

func (b *BxhAdapter) Stop() error {
	b.cancel()
	close(b.ibtpC)
	b.ibtpC = nil
	b.logger.Info("BxhAdapter stopped")
	return nil
}

func (b *BxhAdapter) ID() string {
	return fmt.Sprintf("%d", b.bxhID)
}

func (b *BxhAdapter) Name() string {
	return fmt.Sprintf("bitxhub:%d", b.bxhID)
}

func (b *BxhAdapter) MonitorIBTP() chan *pb.IBTP {
	return b.ibtpC
}

func (b *BxhAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	ibtp, err := b.getIBTPByID(id, isReq)
	if err != nil {
		if isReq == true {
			return nil, err
		}
		txStatus, err := b.getTxStatus(id)
		if err != nil {
			return nil, err
		}

		// TODO: figure out what scenes will trigger
		if txStatus != pb.TransactionStatus_BEGIN {
			ibtp, err = b.getIBTPByID(id, true)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("cannot get IBTP %s with isReq %v, txStatus %v", id, isReq, txStatus)
		}
	}

	return ibtp, nil
}

func (b *BxhAdapter) SendIBTP(ibtp *pb.IBTP) error {
	proof := ibtp.GetProof()
	proofHash := sha256.Sum256(proof)
	ibtp.Proof = proofHash[:]
	if b.mode == repo.UnionMode {
		ibtp.Extra = proof
	}

	tx, _ := b.client.GenerateIBTPTx(ibtp)
	tx.Extra = proof

	var (
		retErr error
		count  = uint64(0)
	)
	if err := retry.Retry(func(attempt uint) error {
		receipt, err := b.client.SendTransactionWithReceipt(tx, nil)
		if err != nil {
			b.logger.Errorf("Send ibtp with receipt error: %s", err.Error())
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
			return err
		}

		// most err occur in bxh's handleIBTP
		if !receipt.IsSuccess() {
			b.logger.WithFields(logrus.Fields{
				"ibtp_id":   ibtp.ID(),
				"ibtp_type": ibtp.Type,
				"msg":       string(receipt.Ret),
			}).Error("Receipt result for ibtp")
			// if no rule bind for this appchain or appchain not available, exit pier
			errMsg := string(receipt.Ret)

			// service have no bind rule when bxh verifyProofs
			if strings.Contains(errMsg, noBindRule) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.ValidationRules_Unregister,
				}

				return nil
			}

			if strings.Contains(errMsg, SrcServiceNotAvailable) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.SrcChainService_Unavailable,
				}

				return nil
			}

			/* redundant code:
			** if target err occur, bxh will actively notify src and dest appchain rollback. bxh receipt type is success,
			** There is no need for pier actively handle error.
			 */

			//// if target service is unavailable(maybe enter wrong format serviceID), just throw error
			//if strings.Contains(errMsg, InvalidTargetService) {
			//	b.logger.Errorf("ignore invalid IBTP ID: invalid target service: %s", errMsg)
			//	return nil
			//}

			//// if target chain is not available, this ibtp should be rollback
			//if strings.Contains(errMsg, TargetAppchainNotAvailable) ||
			//	strings.Contains(errMsg, TargetServiceNotAvailable) ||
			//	strings.Contains(errMsg, TargetBitXHubNotAvailable) {
			//	var isReq bool
			//	switch ibtp.Category() {
			//	case pb.IBTP_REQUEST:
			//		isReq = true
			//	case pb.IBTP_RESPONSE:
			//		isReq = false
			//	default:
			//		b.logger.Warnf("ignore invalid IBTP ID %s", ibtp.ID())
			//		return nil
			//	}
			//
			//	// get multiSign
			//	if err := retry.Retry(func(attempt uint) error {
			//		proof, err := b.getMultiSign(ibtp, isReq)
			//		if err != nil {
			//			return fmt.Errorf("multiSign %w", err)
			//		}
			//		ibtp.Proof = proof
			//		return nil
			//	}, strategy.Wait(1*time.Second)); err != nil {
			//		b.logger.Errorf("get ibtp multiSign from bxh err: %w", err)
			//	}
			//
			//	b.ibtpC <- ibtp
			//	return nil
			//}

			// check index
			if strings.Contains(errMsg, ibtpIndexExist) {
				// if ibtp index is lower than index recorded on bitxhub, then ignore this ibtp
				return nil
			}

			if strings.Contains(errMsg, ibtpIndexWrong) {
				// if index is wrong , means appchain's ibtp_index > bxh's ibtp_index+1
				// TODO: redundant code? maybe it's impossible to occur？
				b.logger.Error("invalid ibtp %s", ibtp.ID())
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.Index_Wrong,
				}

				return nil
			}
			if strings.Contains(errMsg, invalidIBTP) {
				// if this ibtp structure is not compatible
				// try to get new ibtp and resend
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.InvalidIBTP,
				}
				return nil
			}

			if strings.Contains(errMsg, proofFailed) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.Proof_Invalid,
				}
				return nil
			}

			// when bxh trigger timeout rollback, destPier's bxhAdapter want to send receipt to bxh.
			// bxhAdapter will receive ibtpRollback error from bxh, bxhAdapter processing steps are as follows:
			// step1: bxhAdapter QueryIBTP() from bxh, return the ibtp which receive same ibtp sending from srcAppchain to bxh.
			// step2: bxh give a multiSign field with txStatus.
			// if the ibtp will rollback, the proof filed txStatus wii be modified to TransactionStatus_BEGIN_ROLLBACK.
			// step3: bxh send ibtp with TransactionStatus_BEGIN_ROLLBACK to bxhAdapter ibtpCh, then send to destAppchain.
			// step4: destAppchain receive the ibtp, will rollback according to the txStatus(TransactionStatus_BEGIN_ROLLBACK).
			// step5: desAppchain rollback finished, send receipt(type is pb.IBTP_RECEIPT_ROLLBACK) to bxh.
			// step6: bxh change status to TransactionStatus_ROLLBACK, it's done.
			if strings.Contains(errMsg, ibtpRollback) {
				b.logger.WithFields(logrus.Fields{
					"id":  ibtp.ID(),
					"err": errMsg,
				}).Warn("IBTP has rollback")
				if err := retry.Retry(func(attempt uint) error {
					ibtp, err = b.QueryIBTP(ibtp.ID(), true)
					if err != nil {
						b.logger.Warnf("query IBTP %s with isReq true", ibtp.ID())
					} else {
						b.ibtpC <- ibtp
					}
					return err
				}, strategy.Wait(time.Second*3)); err != nil {
					b.logger.Panicf("retry query IBTP %s with isReq true", ibtp.ID())
				}
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
	if b.mode == repo.RelayMode {
		tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.ServiceMgrContractAddr.Address(),
			"GetServicesByAppchainID", rpcx.String(b.appchainId))
		if err != nil {
			panic(fmt.Errorf("GetServiceIDList GenerateContractTx err:%s", err))
		}

		ret := getTxView(b.client, tx)
		service := make([]*servicemgr.Service, 0)
		if ret == nil {
			return nil, fmt.Errorf("appchain[id:%s] info is not exit in bitxhub", b.appchainId)
		}
		if err = json.Unmarshal(ret, &service); err != nil {
			panic(fmt.Errorf("GetServiceIDList unmarshal err:%s", err))
		}
		serviceIDList := make([]string, 0)
		for _, s := range service {
			serviceIDList = append(serviceIDList, s.ServiceID)
		}
		return serviceIDList, nil
	}

	// union model
	tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetAllServiceIDs")
	if err != nil {
		panic(err)
	}

	ret := getTxView(b.client, tx)

	services := make([]string, 0)
	if err := json.Unmarshal(ret, &services); err != nil {
		panic(err)
	}

	ids := make([]string, 0)

	bitXHubChainIDs, err := b.getBitXHubChainIDs()
	if err != nil {
		return nil, err
	}
	bitXHubChainIDsMap := make(map[string]interface{})
	for _, v := range bitXHubChainIDs {
		bitXHubChainIDsMap[v] = ""
	}
	for _, service := range services {
		var bxh string
		var err error
		if bxh, _, _, err = pb.ParseFullServiceID(service); err != nil {
			b.logger.WithField("service", service).Warnf("ParseFullServiceID err:%s", err.Error())
			continue
		}
		if bxh != b.ID() {
			_, ok := bitXHubChainIDsMap[bxh]
			if ok {
				ids = append(ids, service)
			}
		}
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

func (b *BxhAdapter) getBitXHubChainIDs() ([]string, error) {
	tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(),
		"GetBitXHubChainIDs")
	if err != nil {
		return nil, err
	}

	ret := getTxView(b.client, tx)

	bitXHubChainIDs := make([]string, 0)
	if err := json.Unmarshal(ret, &bitXHubChainIDs); err != nil {
		return nil, err
	}
	return bitXHubChainIDs, nil
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
		rawCh, err = b.client.Subscribe(b.ctx, subscriptType, []byte(b.appchainId))
		if err != nil {
			b.logger.Errorf("subscribe err %s", err)
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		panic(err)
	}

	for {
		select {
		case <-b.ctx.Done():
			close(b.wrappersC)
			b.wrappersC = nil
			b.logger.Info("stop bxh adapter running")
			return
		case h, ok := <-rawCh:
			if !ok {
				b.logger.Warn("Unexpected closed channel while syncing interchain tx wrapper")
				return
			}

			b.wrappersC <- h.(*pb.InterchainTxWrappers)
		}
	}

}

// listenInterchainTxWrappers listen on the wrapper channel for handling
func (b *BxhAdapter) listenInterchainTxWrappers() {
	for {
		select {
		case wrappers := <-b.wrappersC:
			if nil == wrappers {
				continue
			}
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

		var isReq bool
		switch ibtp.Category() {
		case pb.IBTP_REQUEST:
			isReq = true
		case pb.IBTP_RESPONSE:
			isReq = false
		default:
			b.logger.Errorf("unknow ibtp Category")
			return false
		}

		proof, err := b.getSign(ibtp, isReq)
		if err != nil {
			return false
		}
		ibtp.Proof = proof
		if tx.IsBatch && b.mode == repo.RelayMode {
			ibtp.Extra = []byte("1")
			b.logger.Info("get batch ibtp")
		}
		b.ibtpC <- ibtp
	}

	for _, id := range w.TimeoutIbtps {
		if err := retry.Retry(func(attempt uint) error {
			ibtp, err := b.QueryIBTP(id, true)
			if err != nil {
				b.logger.Warnf("query timeout ibtp %s: %v", id, err)
			} else {
				b.ibtpC <- ibtp
			}
			return err
		}, strategy.Wait(time.Second*3)); err != nil {
			b.logger.Panicf("retry query timeout ibtp %s failed: %v", id, err)
		}
	}

	for _, id := range w.MultiTxIbtps {
		if err := retry.Retry(func(attempt uint) error {
			ibtp, err := b.QueryIBTP(id, true)
			if err != nil {
				b.logger.Warnf("query multitx ibtp %s: %v", ibtp, err)
			} else {
				b.ibtpC <- ibtp
			}
			return err
		}, strategy.Wait(time.Second*3)); err != nil {
			b.logger.Panicf("retry query multitx ibtp %s failed: %v", id, err)
		}
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

		tx, err := b.client.GenerateContractTx(pb.TransactionData_BVM, constant.TransactionMgrContractAddr.Address(), "GetStatus", rpcx.String(id))
		if err != nil {
			b.logger.Errorf("generateContractTx err: %s", err)
			return err
		}
		receipt, err = b.client.SendView(tx)
		if err != nil {
			return err
		}
		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		b.logger.Errorf("Retry to get tx status")
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

func (b *BxhAdapter) getSign(ibtp *pb.IBTP, isReq bool) ([]byte, error) {
	var (
		err     error
		retSign *pb.SignResponse
		reqTyp  pb.GetSignsRequest_Type
	)

	if b.tss.EnableTSS {
		if isReq {
			reqTyp = pb.GetSignsRequest_TSS_IBTP_REQUEST
		} else {
			reqTyp = pb.GetSignsRequest_TSS_IBTP_RESPONSE
		}
		retSign, err = b.client.GetTssSigns(ibtp.ID(), reqTyp, nil)
		if err != nil {
			return nil, err
		}
	} else {
		if isReq {
			reqTyp = pb.GetSignsRequest_MULTI_IBTP_REQUEST
		} else {
			reqTyp = pb.GetSignsRequest_MULTI_IBTP_RESPONSE
		}

		retSign, err = b.client.GetMultiSigns(ibtp.ID(), reqTyp)
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

func (b *BxhAdapter) restartCh() {
	if b.ibtpC == nil {
		b.ibtpC = make(chan *pb.IBTP, maxChSize)
	}
	if b.wrappersC == nil {
		b.wrappersC = make(chan *pb.InterchainTxWrappers, maxChSize)
	}
}

func (b *BxhAdapter) getIBTPByID(id string, isReq bool) (*pb.IBTP, error) {
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

	if !receipt.IsSuccess() {
		return nil, fmt.Errorf("get IBTP by ID %s, isReq %v failed", id, isReq)
	}

	hash := types.NewHash(receipt.Ret)
	response, err := b.client.GetTransaction(hash.String())
	if err != nil {
		return nil, err
	}

	retIBTP := response.Tx.GetIBTP()
	proof, err := b.getSign(retIBTP, isReq)
	if err != nil {
		return nil, err
	}
	// get ibtp proof from bxh
	retIBTP.Proof = proof

	return retIBTP, nil
}
