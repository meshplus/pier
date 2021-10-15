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

const maxChSize = 1 << 10

// BxhAdapter represents the necessary data for sync tx from bitxhub
type BxhAdapter struct {
	client    rpcx.Client
	logger    logrus.FieldLogger
	wrappersC chan *pb.InterchainTxWrappers
	ibtpC     chan *pb.IBTP

	mode   string
	pierID string
	bxhID  uint64
	ctx    context.Context
	cancel context.CancelFunc
}

// TODO: return bitxhub ID
func (b *BxhAdapter) GetChainID() string {
	return ""
}

func (b *BxhAdapter) MonitorUpdatedMeta() chan *[]byte {
	panic("implement me")
}

func (b *BxhAdapter) SendUpdatedMeta(byte []byte) error {
	panic("implement me")
}

// New creates instance of WrapperSyncer given agent interacting with bitxhub,
// validators addresses of bitxhub and local storage
func New(mode string, client rpcx.Client, logger logrus.FieldLogger) (*BxhAdapter, error) {
	bxhID, err := client.GetChainID()
	if err != nil {
		return nil, fmt.Errorf("new bxh adapter err: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	ba := &BxhAdapter{
		// todo: not close the channel
		wrappersC: make(chan *pb.InterchainTxWrappers, maxChSize),
		ibtpC:     make(chan *pb.IBTP, maxChSize),
		client:    client,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mode:      mode,
		bxhID:     bxhID,
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
		return nil, fmt.Errorf("query IBTP %s, isReq %v failed", id, isReq)
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
		// ensure getting receipt successful
		if err := retry.Retry(func(attempt uint) error {
			receipt, err = b.client.GetReceipt(hash)
			if err != nil {
				return fmt.Errorf("get tx receipt by hash %s: %w", hash, err)
			}
			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			b.logger.Errorf("retry error to get receipt: %w", err)
		}

		if !receipt.IsSuccess() {
			b.logger.WithFields(logrus.Fields{
				"ibtp_id":   ibtp.ID(),
				"ibtp_type": ibtp.Type,
				"msg":       string(receipt.Ret),
			}).Error("Receipt result for ibtp")
			// if no rule bind for this appchain or appchain not available, exit pier
			errMsg := string(receipt.Ret)

			if strings.Contains(errMsg, noBindRule) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.ValidationRules_Unregister,
				}

				return nil
			}

			if strings.Contains(errMsg, CurAppchainNotAvailable) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.SrcChain_Unavailable,
				}

				return nil
			}

			if strings.Contains(errMsg, CurServiceNotAvailable) {
				retErr = &adapt.SendIbtpError{
					Err:    errMsg,
					Status: adapt.SrcChainService_Unavailable,
				}

				return nil
			}

			// if target service is unavailable(maybe enter wrong format serviceID), just throw error
			if strings.Contains(errMsg, InvalidTargetService) {
				b.logger.Errorf("ignore invalid IBTP ID: invalid target service: %s", errMsg)
				return nil
			}

			// if target chain is not available, this ibtp should be rollback
			if strings.Contains(errMsg, TargetAppchainNotAvailable) ||
				strings.Contains(errMsg, TargetServiceNotAvailable) ||
				strings.Contains(errMsg, TargetBitXHubNotAvailable) {
				var isReq bool
				switch ibtp.Category() {
				case pb.IBTP_REQUEST:
					isReq = true
				case pb.IBTP_RESPONSE:
					isReq = false
				default:
					b.logger.Warnf("ignore invalid IBTP ID %s", ibtp.ID())
					return nil
				}

				// get multiSign
				if err := retry.Retry(func(attempt uint) error {
					proof, err := b.getMultiSign(ibtp, isReq)
					if err != nil {
						return fmt.Errorf("multiSign %w", err)
					}
					ibtp.Proof = proof
					return nil
				}, strategy.Wait(1*time.Second)); err != nil {
					b.logger.Errorf("get ibtp multiSign from bxh err: %w", err)
				}

				b.ibtpC <- ibtp
				return nil
			}
			if strings.Contains(errMsg, ibtpIndexExist) {
				// if ibtp index is lower than index recorded on bitxhub, then ignore this ibtp
				return nil
			}

			if strings.Contains(errMsg, ibtpIndexWrong) {
				// if index is wrong ,notify exchanger to update its meta from bitxhub
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
					Status: adapt.Other_Error,
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

			if strings.Contains(errMsg, "has been rollback") {
				b.logger.WithField("id", ibtp.ID()).Warnf("Tx has been rollback")
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

	bitXHubChainIDs, err := b.GetBitXHubChainIDs()
	if err != nil {
		b.logger.Panic(err)
	}
	bitXHubChainIDsMap := make(map[string]interface{})
	for _, v := range bitXHubChainIDs {
		bitXHubChainIDsMap[v] = ""
	}
	for _, service := range services {
		var bxh string
		var err error
		if bxh, _, _, err = pb.ParseFullServiceID(service); err != nil {
			panic(err)
		}
		if b.mode == repo.UnionMode && bxh != b.ID() {
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

func (b *BxhAdapter) GetBitXHubChainIDs() ([]string, error) {
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
		rawCh, err = b.client.Subscribe(b.ctx, subscriptType, []byte(strconv.Itoa(int(b.bxhID))))
		if err != nil {
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

		proof, err := b.getMultiSign(ibtp, isReq)
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

func (b *BxhAdapter) restartCh() {
	if b.ibtpC == nil {
		b.ibtpC = make(chan *pb.IBTP, maxChSize)
	}
	if b.wrappersC == nil {
		b.wrappersC = make(chan *pb.InterchainTxWrappers, maxChSize)
	}
}
