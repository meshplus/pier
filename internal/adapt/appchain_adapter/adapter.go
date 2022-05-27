package appchain_adapter

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/checker"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/internal/utils"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ adapt.Adapt = (*AppchainAdapter)(nil)

type AppchainAdapter struct {
	config       *repo.Config
	client       plugins.Client
	pluginClient *plugin.Client
	checker      checker.Checker
	cryptor      txcrypto.Cryptor
	logger       logrus.FieldLogger
	ibtpC        chan *pb.IBTP

	appchainID string
	bitxhubID  string
}

const IBTP_CH_SIZE = 1024

func NewAppchainAdapter(config *repo.Config, logger logrus.FieldLogger, crypto txcrypto.Cryptor) (adapt.Adapt, error) {
	adapter := &AppchainAdapter{
		config:  config,
		cryptor: crypto,
		logger:  logger,
	}

	if err := adapter.init(); err != nil {
		return nil, err
	}

	return adapter, nil
}

func (a *AppchainAdapter) Start() error {
	if a.client == nil || a.pluginClient == nil {
		if err := a.init(); err != nil {
			return err
		}
	}

	if err := a.client.Start(); err != nil {
		return err
	}

	go func() {
		ibtpC := a.client.GetIBTPCh()
		if ibtpC != nil {
			for ibtp := range ibtpC {
				ok, err := a.checkIBTPInDirectMode(ibtp)
				if err != nil {
					a.logger.Errorf("check IBTP %s in direct mode: %v", ibtp.ID(), err)
					continue
				}

				if !ok {
					a.logger.Warnf("omit invalid IBTP %s in direct mode", ibtp.ID())
					continue
				}

				ibtp, _, err := a.handlePayload(ibtp, true)
				if err != nil {
					a.logger.Warnf("fail to encrypt monitored IBTP: %v", err)
					continue
				}

				a.ibtpC <- ibtp
			}
		}
		a.logger.Info("ibtp channel of appchain plugin is closed")
		close(a.ibtpC)
	}()

	a.logger.Info("appchain adapter start")

	return nil
}

func (a *AppchainAdapter) Stop() error {
	if err := a.client.Stop(); err != nil {
		return err
	}

	a.pluginClient.Kill()
	a.client = nil
	a.pluginClient = nil
	a.logger.Info("appchain adapter stopped")
	return nil
}

func (a *AppchainAdapter) ID() string {
	return fmt.Sprintf("%s", a.appchainID)
}
func (a *AppchainAdapter) Name() string {
	return fmt.Sprintf("appchain:%s", a.appchainID)
}

func (a *AppchainAdapter) MonitorIBTP() chan *pb.IBTP {
	return a.ibtpC
}

func (a *AppchainAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	srcServiceID, dstServiceID, index, err := utils.ParseIBTPID(id)
	if err != nil {
		return nil, err
	}

	servicePair := pb.GenServicePair(srcServiceID, dstServiceID)

	if isReq {
		return a.client.GetOutMessage(servicePair, index)
	}

	return a.client.GetReceiptMessage(servicePair, index)
}

func (a *AppchainAdapter) SendIBTP(ibtp *pb.IBTP) error {
	a.logger.Warn("1111111111111111")
	var res *pb.SubmitIBTPResponse
	proof := &pb.BxhProof{}

	isReq, err := a.checker.BasicCheck(ibtp)
	if err != nil {
		return err
	}
	a.logger.Warn("2222")
	ibtp, pd, err := a.handlePayload(ibtp, false)
	if err != nil {
		return err
	}

	if err := a.checker.CheckProof(ibtp); err != nil {
		return err
	}
	a.logger.Warn("33333")
	if a.config.Mode.Type == repo.RelayMode {
		if err := proof.Unmarshal(ibtp.Proof); err != nil {
			return fmt.Errorf("fail to unmarshal proof of ibtp %s: %w", ibtp.ID(), err)
		}
	}
	a.logger.Warnf("req,%v", isReq)

	if isReq {
		content := &pb.Content{}
		if err := content.Unmarshal(pd.Content); err != nil {
			return fmt.Errorf("unmarshal content of ibtp %s: %w", ibtp.ID(), err)
		}
		_, _, serviceID := ibtp.ParseTo()
		a.logger.WithFields(logrus.Fields{
			"ibtp": ibtp.ID(),
			"typ":  ibtp.Type,
		}).Info("start submit ibtp")
		res, err = a.client.SubmitIBTP(ibtp.From, ibtp.Index, serviceID, ibtp.Type, content, proof, pd.Encrypted)
		a.logger.Info("appchain adapter submit ibtp success")
	} else {
		a.logger.Warn("4444444444444")
		a.logger.Warn("3434343," + string(pd.Content))
		result := &pb.Result{}
		if err := result.Unmarshal(pd.Content); err != nil {
			return fmt.Errorf("unmarshal result of ibtp %s: %w", ibtp.ID(), err)
		}
		_, _, serviceID := ibtp.ParseFrom()
		a.logger.WithFields(logrus.Fields{
			"ibtp": ibtp.ID(),
			"typ":  ibtp.Type,
		}).Info("start submit receipt")
		res, err = a.client.SubmitReceipt(ibtp.To, ibtp.Index, serviceID, ibtp.Type, result, proof)
		a.logger.Debug("appchain adapter submit receipt success")
	}
	a.logger.Warn("5555555555555")
	if err != nil {
		// solidity broker cannot get detailed error info
		return &adapt.SendIbtpError{
			Err:    fmt.Sprintf("fail to send ibtp %s with type %v: %v", ibtp.ID(), ibtp.Type, err),
			Status: adapt.Other_Error,
		}
	}

	if !res.Status {
		err := &adapt.SendIbtpError{Err: fmt.Sprintf("fail to send ibtp %s with type %v: %s", ibtp.ID(), ibtp.Type, res.Message)}
		if strings.Contains(res.Message, "invalid multi-signature") {
			err.Status = adapt.Proof_Invalid
		} else if a.config.Mode.Type == repo.DirectMode && (strings.Contains(res.Message, "dest address is not in local white list") ||
			strings.Contains(res.Message, "remote service is not registered") ||
			strings.Contains(res.Message, "remote service is not allowed to call dest address")) {
			ibtp.Type = pb.IBTP_RECEIPT_FAILURE
			a.ibtpC <- ibtp
			err.Status = adapt.Other_Error
		} else {
			err.Status = adapt.Other_Error
		}
		return err
	}

	return nil
}

func (a *AppchainAdapter) GetServiceIDList() ([]string, error) {
	return a.client.GetServices()
}

func (a *AppchainAdapter) QueryInterchain(serviceID string) (*pb.Interchain, error) {
	outMeta, err := a.client.GetOutMeta()
	if err != nil {
		return nil, err
	}
	callbackMeta, err := a.client.GetCallbackMeta()
	if err != nil {
		return nil, err
	}
	inMeta, err := a.client.GetInMeta()
	if err != nil {
		return nil, err
	}

	interchainCounter, err := filterMap(outMeta, serviceID, true)
	if err != nil {
		return nil, err
	}

	receiptCounter, err := filterMap(callbackMeta, serviceID, true)
	if err != nil {
		return nil, err
	}

	sourceInterchainCounter, err := filterMap(inMeta, serviceID, false)
	if err != nil {
		return nil, err
	}

	sourceReceiptCounter, err := filterMap(inMeta, serviceID, false)
	if err != nil {
		return nil, err
	}

	return &pb.Interchain{
		ID:                      serviceID,
		InterchainCounter:       interchainCounter,
		ReceiptCounter:          receiptCounter,
		SourceInterchainCounter: sourceInterchainCounter,
		SourceReceiptCounter:    sourceReceiptCounter,
	}, nil

}

func (a *AppchainAdapter) init() error {
	var err error

	if err := retry.Retry(func(attempt uint) error {
		a.client, a.pluginClient, err = plugins.CreateClient(&a.config.Appchain, nil)
		if err != nil {
			a.logger.Errorf("create client plugin", "error", err.Error())
		}
		return err
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to create plugin: %w", err)
	}

	a.ibtpC = make(chan *pb.IBTP, IBTP_CH_SIZE)

	a.bitxhubID, a.appchainID, err = a.client.GetChainID()
	if err != nil {
		return err
	}

	if a.config.Mode.Type == repo.DirectMode {
		a.checker = checker.NewDirectChecker(a.client, a.appchainID, a.logger, a.config.Mode.Direct.GasLimit)
	} else {
		a.checker = checker.NewRelayChecker(a.client, a.appchainID, a.bitxhubID, a.logger)
	}

	return nil
}

func (a *AppchainAdapter) GetChainID() string {
	return a.appchainID
}

func (a *AppchainAdapter) MonitorUpdatedMeta() chan *[]byte {
	panic("implement me")
}

func (a *AppchainAdapter) SendUpdatedMeta(byte []byte) error {
	panic("implement me")
}

func (a *AppchainAdapter) handlePayload(ibtp *pb.IBTP, encrypt bool) (*pb.IBTP, *pb.Payload, error) {
	pd := pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		return nil, nil, fmt.Errorf("cannot unmarshal payload for monitored ibtp %s", ibtp.ID())
	}

	var (
		chainID    string
		newContent []byte
		err        error
	)
	_, srcChainID, _ := ibtp.ParseFrom()
	_, dstChainID, _ := ibtp.ParseTo()

	if pd.Encrypted {
		if encrypt {
			if ibtp.Category() == pb.IBTP_REQUEST {
				chainID = dstChainID
			} else {
				chainID = srcChainID
			}
			newContent, err = a.cryptor.Encrypt(pd.Content, chainID)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot encrypt content for monitored ibtp %s", ibtp.ID())
			}
		} else {
			if ibtp.Category() == pb.IBTP_REQUEST {
				chainID = srcChainID
			} else {
				chainID = dstChainID
			}
			newContent, err = a.cryptor.Decrypt(pd.Content, chainID)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot encrypt content for monitored ibtp %s", ibtp.ID())
			}
		}

		pd.Content = newContent
		data, err := pd.Marshal()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot marshal payload for monitored ibtp %s", ibtp.ID())
		}
		ibtp.Payload = data
	}

	return ibtp, &pd, nil
}

func filterMap(meta map[string]uint64, serviceID string, isSrc bool) (map[string]uint64, error) {
	counterM := make(map[string]uint64)
	for servicePair, idx := range meta {
		srcServiceID, dstServiceID, err := utils.ParseServicePair(servicePair)
		if err != nil {
			return nil, err
		}

		if isSrc {
			if srcServiceID == serviceID {
				counterM[dstServiceID] = idx
			}
		} else {
			if dstServiceID == serviceID {
				counterM[srcServiceID] = idx
			}
		}
	}

	return counterM, nil
}

func (a *AppchainAdapter) RollbackInDirectMode(ibtp *pb.IBTP) error {
	_, _, serviceID := ibtp.ParseFrom()
	_, err := a.client.SubmitReceipt(ibtp.To, ibtp.Index, serviceID, pb.IBTP_RECEIPT_FAILURE, &pb.Result{}, &pb.BxhProof{})

	return err
}

func (a *AppchainAdapter) checkIBTPInDirectMode(ibtp *pb.IBTP) (bool, error) {
	if a.config.Mode.Type != repo.DirectMode || ibtp.Type != pb.IBTP_INTERCHAIN {
		return true, nil
	}

	if err := ibtp.CheckServiceID(); err != nil {
		if err := a.RollbackInDirectMode(ibtp); err != nil {
			a.logger.Errorf("rollback in direct mode for IBTP %s: %v", ibtp.ID(), err)
			return false, err
		} else {
			return false, nil
		}
	}

	bxhID, chainID, serviceID := ibtp.ParseTo()
	if bxhID != "" || chainID == "" || serviceID == "" {
		if err := a.RollbackInDirectMode(ibtp); err != nil {
			a.logger.Errorf("rollback in direct mode for IBTP %s: %v", ibtp.ID(), err)
			return false, err
		} else {
			return false, nil
		}
	}

	return true, nil
}
