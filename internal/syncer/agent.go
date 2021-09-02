package syncer

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/pkg/model"
)

const (
	srcchainNotAvailable      = "current appchain not available"
	dstchainNotAvailable      = "target appchain not available"
	invalidIBTP               = "invalid ibtp"
	ibtpIndexExist            = "index already exists"
	ibtpIndexWrong            = "wrong index"
	noBindRule                = "appchain didn't register rule"
	InvalidSourceService      = "invalid source service"
	InvalidTargetService      = "invalid target service"
	TargetServiceNotAvailable = "target service not available"
)

var (
	ErrIBTPNotFound  = fmt.Errorf("receipt from bitxhub failed")
	ErrMetaOutOfDate = fmt.Errorf("interchain meta is out of date")
)

func (syncer *WrapperSyncer) GetAssetExchangeSigns(id string) ([]byte, error) {
	resp, err := syncer.client.GetMultiSigns(id, pb.GetMultiSignsRequest_ASSET_EXCHANGE)
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for asset exchange id %s", id)
	}

	var signs []byte
	for _, sign := range resp.Sign {
		signs = append(signs, sign...)
	}

	return signs, nil
}

func (syncer *WrapperSyncer) GetIBTPSigns(ibtp *pb.IBTP) ([]byte, error) {
	hash := ibtp.Hash()
	resp, err := syncer.client.GetMultiSigns(hash.String(), pb.GetMultiSignsRequest_IBTP)
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Sign == nil {
		return nil, fmt.Errorf("get empty signatures for ibtp %s", ibtp.ID())
	}
	signs, err := resp.Marshal()
	if err != nil {
		return nil, err
	}

	return signs, nil
}

func (syncer *WrapperSyncer) GetAppchains() ([]*appchainmgr.Appchain, error) {
	tx, err := syncer.client.GenerateContractTx(pb.TransactionData_BVM, constant.AppchainMgrContractAddr.Address(), "Appchains")
	if err != nil {
		return nil, err
	}
	tx.Nonce = 1
	var receipt *pb.Receipt
	if err := syncer.retryFunc(func(attempt uint) error {
		receipt, err = syncer.client.SendView(tx)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		panic(err)
	}

	ret := make([]*appchainmgr.Appchain, 0)
	if receipt == nil || receipt.Ret == nil {
		return ret, nil
	}
	if err := json.Unmarshal(receipt.Ret, &ret); err != nil {
		return nil, err
	}
	appchains := make([]*appchainmgr.Appchain, 0)
	for _, appchain := range ret {
		// todo :ChainType has been del
		if appchain.ID != repo.BitxhubType {
			appchains = append(appchains, appchain)
		}
	}
	return appchains, nil
}

func (syncer *WrapperSyncer) GetInterchainById(from string) *pb.Interchain {
	ic := &pb.Interchain{}
	tx, err := syncer.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(), "GetInterchain", rpcx.String(from))
	if err != nil {
		return ic
	}
	tx.Nonce = 1
	receipt, err := syncer.client.SendView(tx)
	if err != nil {
		return ic
	}
	var interchain pb.Interchain
	if err := interchain.Unmarshal(receipt.Ret); err != nil {
		return ic
	}
	return &interchain
}

func (syncer *WrapperSyncer) QueryInterchainMeta(serviceID string) *pb.Interchain {
	return GetInterchainByServiceID(syncer.client, serviceID)
}

func (syncer *WrapperSyncer) QueryIBTP(ibtpID string, isReq bool) (*pb.IBTP, bool, error) {
	queryTx, err := syncer.client.GenerateContractTx(pb.TransactionData_BVM, constant.InterchainContractAddr.Address(),
		"GetIBTPByID", rpcx.String(ibtpID), rpcx.Bool(isReq))
	if err != nil {
		return nil, false, err
	}
	queryTx.Nonce = 1
	receipt, err := syncer.client.SendView(queryTx)
	if err != nil {
		return nil, false, err
	}

	if !receipt.IsSuccess() {
		return nil, false, fmt.Errorf("%w: %s", ErrIBTPNotFound, string(receipt.Ret))
	}

	hash := types.NewHash(receipt.Ret)
	response, err := syncer.client.GetTransaction(hash.String())
	if err != nil {
		return nil, false, err
	}
	receipt, err = syncer.client.GetReceipt(hash.String())
	if err != nil {
		return nil, false, err
	}
	return response.Tx.GetIBTP(), receipt.Status == pb.Receipt_SUCCESS, nil
}

func (syncer *WrapperSyncer) ListenIBTP() <-chan *model.WrappedIBTP {
	return syncer.ibtpC
}

func (syncer *WrapperSyncer) SendIBTP(ibtp *pb.IBTP) error {
	proof := ibtp.GetProof()
	proofHash := sha256.Sum256(proof)
	ibtp.Proof = proofHash[:]

	tx, _ := syncer.client.GenerateIBTPTx(ibtp)
	tx.Extra = proof

	var (
		receipt *pb.Receipt
		retErr  error
		count   = uint64(0)
	)
	if err := retry.Retry(func(attempt uint) error {
		hash, err := syncer.client.SendTransaction(tx, nil)
		if err != nil {
			syncer.logger.Errorf("Send ibtp error: %s", err.Error())
			if errors.Is(err, rpcx.ErrRecoverable) {
				count++
				if count == 5 && ibtp.Type == pb.IBTP_INTERCHAIN {
					retErr = fmt.Errorf("rollback ibtp %s: %v", ibtp.ID(), err)
					return nil
				}
				return err
			}
			if errors.Is(err, rpcx.ErrReconstruct) {
				tx, _ = syncer.client.GenerateIBTPTx(ibtp)
				return err
			}
		}
		receipt, err = syncer.client.GetReceipt(hash)
		if err != nil {
			return fmt.Errorf("get tx receipt by hash %s: %w", hash, err)
		}
		if !receipt.IsSuccess() {
			syncer.logger.WithFields(logrus.Fields{
				"ibtp_id":   ibtp.ID(),
				"ibtp_type": ibtp.Type,
				"msg":       string(receipt.Ret),
			}).Error("Receipt result for ibtp")
			// if no rule bind for this appchain or appchain not available, exit pier
			errMsg := string(receipt.Ret)
			if strings.Contains(errMsg, noBindRule) || strings.Contains(errMsg, srcchainNotAvailable) {
				return fmt.Errorf("appchain not valid: %s", errMsg)
			}

			// if target chain is not available, this ibtp should be rollback
			if strings.Contains(errMsg, InvalidTargetService) {
				syncer.logger.Errorf("%s, try to rollback in source appchain...", errMsg)
				syncer.rollbackHandler(ibtp, "")
				return nil
			}

			// if target chain is not available, this ibtp should be rollback
			if strings.Contains(errMsg, dstchainNotAvailable) || strings.Contains(errMsg, TargetServiceNotAvailable) {
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
				syncer.logger.WithField("id", ibtp.ID()).Warnf("Tx has been rollback")
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

func (syncer *WrapperSyncer) retryFunc(handle func(uint) error) error {
	return retry.Retry(func(attempt uint) error {
		if err := handle(attempt); err != nil {
			syncer.logger.Errorf("retry failed for reason: %s", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(500*time.Millisecond))
}

func (syncer *WrapperSyncer) GetTxStatus(id string) (pb.TransactionStatus, error) {
	receipt, err := syncer.client.InvokeBVMContract(constant.TransactionMgrContractAddr.Address(), "GetStatus", nil, rpcx.String(id))
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
