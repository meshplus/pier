package adapt

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/hashicorp/go-plugin"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var _ Adapt = (*AppchainAdapter)(nil)

type AppchainAdapter struct {
	config       *repo.Appchain
	client       plugins.Client
	pluginClient *plugin.Client
	logger       logrus.FieldLogger
	appchainID   string
	bitxhubID    string
}

func NewAppchainAdapter(config *repo.Config, logger logrus.FieldLogger) (Adapt, error) {
	adapter := &AppchainAdapter{
		config: &config.Appchain,
		logger: logger,
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

	return a.client.Start()
}

func (a *AppchainAdapter) Stop() error {
	if err := a.client.Stop(); err != nil {
		return err
	}

	a.pluginClient.Kill()
	a.client = nil
	a.pluginClient = nil

	return nil
}

func (a *AppchainAdapter) Name() string {
	return a.appchainID
}

func (a *AppchainAdapter) MonitorIBTP() chan *pb.IBTP {
	return a.client.GetIBTP()
}

func (a *AppchainAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	srcServiceID, dstServiceID, idx, err := pb.ParseFullServiceID(id)
	if err != nil {
		return nil, err
	}

	servicePair := pb.GenServicePair(srcServiceID, dstServiceID)
	index, err := strconv.Atoi(idx)
	if err != nil {
		return nil, err
	}

	if isReq {
		return a.client.GetOutMessage(servicePair, uint64(index))
	}

	return a.client.GetReceiptMessage(servicePair, uint64(index))
}

func (a *AppchainAdapter) SendIBTP(ibtp *pb.IBTP) error {
	var (
		category pb.IBTP_Category
		res      *pb.SubmitIBTPResponse
		err      error
	)

	category, err = a.figureOutReceivedIBTPCategory(ibtp)
	if err != nil {
		return err
	}

	if category == pb.IBTP_REQUEST {
		res, err = a.client.SubmitIBTP(ibtp)
	} else {
		res, err = a.client.SubmitReceipt(ibtp)

	}
	if err != nil {
		return err
	}

	if !res.Status {
		return fmt.Errorf("fail to send ibtp %s with type %v: %w", res.Message, ibtp.ID(), ibtp.Type)
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

func (a *AppchainAdapter) figureOutReceivedIBTPCategory(ibtp *pb.IBTP) (pb.IBTP_Category, error) {
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
		srcServiceID, dstServiceID, err := pb.ParseServicePair(servicePair)
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
