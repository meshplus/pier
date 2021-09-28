package appchain_adapter

import (
	"fmt"
	"github.com/meshplus/pier/internal/adapt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/utils"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ adapt.Adapt = (*Adapter)(nil)

type Adapter struct {
	config       *repo.Appchain
	client       plugins.Client
	pluginClient *plugin.Client
	logger       logrus.FieldLogger
	appchainID   string
	bitxhubID    string
}

func (a *Adapter) GetAppchainID() string {
	return a.appchainID
}

func (a *Adapter) MonitorUpdatedMeta() chan *[]byte {
	panic("implement me")
}

func (a *Adapter) SendUpdatedMeta(byte []byte) error {
	panic("implement me")
}

func NewAppchainAdapter(config *repo.Config, logger logrus.FieldLogger) (adapt.Adapt, error) {
	adapter := &Adapter{
		config: &config.Appchain,
		logger: logger,
	}

	if err := adapter.init(); err != nil {
		return nil, err
	}

	return adapter, nil
}

func (a *Adapter) Start() error {
	if a.client == nil || a.pluginClient == nil {
		if err := a.init(); err != nil {
			return err
		}
	}

	return a.client.Start()
}

func (a *Adapter) Stop() error {
	if err := a.client.Stop(); err != nil {
		return err
	}

	a.pluginClient.Kill()
	a.client = nil
	a.pluginClient = nil

	return nil
}

func (a *Adapter) Name() string {
	return a.appchainID
}

func (a *Adapter) MonitorIBTP() chan *pb.IBTP {
	return a.client.GetIBTPCh()
}

func (a *Adapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
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

func (a *Adapter) SendIBTP(ibtp *pb.IBTP) error {
	var (
		category pb.IBTP_Category
		res      *pb.SubmitIBTPResponse
		err      error
	)

	category, err = a.figureOutReceivedIBTPCategory(ibtp)
	if err != nil {
		return err
	}

	pd := &pb.Payload{}
	if err := pd.Unmarshal(ibtp.Payload); err != nil {
		return fmt.Errorf("ibtp payload unmarshal: %w", err)
	}

	_, _, serviceID, err := pb.ParseFullServiceID(ibtp.To)
	if err != nil {
		return err
	}

	proof := &pb.BxhProof{}
	if ibtp.Proof != nil {
		if err := proof.Unmarshal(ibtp.Proof); err != nil {
			return err
		}
	}

	if category == pb.IBTP_REQUEST {
		content := &pb.Content{}
		if err := content.Unmarshal(pd.Content); err != nil {
			return fmt.Errorf("ibtp content unmarshal: %w", err)
		}
		res, err = a.client.SubmitIBTP(ibtp.From, ibtp.Index, serviceID, ibtp.Type, content, proof, pd.Encrypted)
	} else {
		result := &pb.Result{}
		if err := result.Unmarshal(pd.Content); err != nil {
			return fmt.Errorf("ibtp content unmarshal: %w", err)
		}
		res, err = a.client.SubmitReceipt(ibtp.To, ibtp.Index, serviceID, ibtp.Type, result, proof)

	}
	if err != nil {
		// solidity broker cannot get detailed error info
		return &adapt.SendIbtpError{
			Err:    fmt.Sprintf("fail to send ibtp %s with type %v: %v", ibtp.ID(), ibtp.Type, err),
			Status: adapt.Other_Error,
		}
	}

	if !res.Status {
		return &adapt.SendIbtpError{
			Err:    fmt.Sprintf("fail to send ibtp %s with type %v: %s", ibtp.ID(), ibtp.Type, res.Message),
			Status: adapt.Other_Error,
		}
	}

	return nil
}

func (a *Adapter) GetServiceIDList() ([]string, error) {
	return a.client.GetServices()
}

func (a *Adapter) QueryInterchain(serviceID string) (*pb.Interchain, error) {
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

func (a *Adapter) init() error {
	var err error

	if err := retry.Retry(func(attempt uint) error {
		a.client, a.pluginClient, err = plugins.CreateClient(a.config, nil)
		if err != nil {
			a.logger.Errorf("create client plugin", "error", err.Error())
		}
		return err
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to create plugin: %w", err)
	}

	a.bitxhubID, a.appchainID, err = a.client.GetChainID()

	return err
}

func (a *Adapter) figureOutReceivedIBTPCategory(ibtp *pb.IBTP) (pb.IBTP_Category, error) {
	if err := ibtp.CheckServiceID(); err != nil {
		return 0, err
	}

	bxhID, chainID, _ := ibtp.ParseFrom()
	if bxhID == a.bitxhubID && chainID == a.appchainID {
		return pb.IBTP_RESPONSE, nil
	}

	bxhID, chainID, _ = ibtp.ParseTo()
	if bxhID == a.bitxhubID && chainID == a.appchainID {
		return pb.IBTP_REQUEST, nil
	}

	return 0, fmt.Errorf("this IBTP %s is not for current appchain", ibtp.ID())
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
