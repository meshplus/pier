package direct_adapter

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

var _ adapt.Adapt = (*DirectAdapter)(nil)

const (
	maxChSize    = 1 << 10
	maxCacheSize = 10000
)

type DirectAdapter struct {
	ibtpCache *lru.Cache
	maxIndex  uint64

	logger        logrus.FieldLogger
	ibtpC         chan *pb.IBTP
	peerMgr       peermgr.PeerManager
	appchainadapt adapt.Adapt
	lock          *sync.Mutex //control the concurrent count
	sendIBTPTimer atomic.Duration
	appchainID    string
}

func (d *DirectAdapter) ID() string {
	return fmt.Sprintf("%s", d.appchainID)
}

func (d *DirectAdapter) MonitorUpdatedMeta() chan *[]byte {
	panic("implement me")
}

func (d *DirectAdapter) SendUpdatedMeta(byte []byte) error {
	panic("implement me")
}

func (d *DirectAdapter) GetServiceIDList() ([]string, error) {
	panic("implement me")
}

func New(peerMgr peermgr.PeerManager, appchainAdapt adapt.Adapt, logger logrus.FieldLogger) (*DirectAdapter, error) {
	ibtpCache, err := lru.New(maxCacheSize)
	if err != nil {
		return nil, fmt.Errorf("ibtpCache initialize err: %w", err)
	}

	appchainID := appchainAdapt.ID()

	da := &DirectAdapter{
		logger:        logger,
		peerMgr:       peerMgr,
		appchainadapt: appchainAdapt,
		ibtpCache:     ibtpCache,
		maxIndex:      0,
		lock:          &sync.Mutex{},
		ibtpC:         make(chan *pb.IBTP, maxChSize),
		appchainID:    appchainID,
	}

	return da, nil
}

func (d *DirectAdapter) Start() error {
	if d.ibtpC == nil {
		d.ibtpC = make(chan *pb.IBTP, maxChSize)
	}
	if err := d.peerMgr.RegisterMsgHandler(pb.Message_INTERCHAIN_META_GET, d.handleGetInterchainMessage); err != nil {
		return fmt.Errorf("register query interchain msg handler: %w", err)
	}

	if err := d.peerMgr.RegisterMsgHandler(pb.Message_IBTP_SEND, d.handleSendIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := d.peerMgr.RegisterMsgHandler(pb.Message_ADDRESS_GET, d.handleGetAddressMessage); err != nil {
		return fmt.Errorf("register get address msg handler: %w", err)
	}

	if err := d.peerMgr.RegisterMultiMsgHandler([]pb.Message_Type{pb.Message_IBTP_RECEIPT_GET, pb.Message_IBTP_GET},
		d.handleGetIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	if err := d.peerMgr.Start(); err != nil {
		return fmt.Errorf("peerMgr start: %w", err)
	}
	return nil
}

func (d *DirectAdapter) Stop() error {
	err := d.peerMgr.Stop()
	if err != nil {
		return err
	}

	close(d.ibtpC)
	d.ibtpC = nil

	d.logger.Info("DirectAdapter stopped")
	return nil
}

func (d *DirectAdapter) Name() string {
	return fmt.Sprintf("direct:%s", d.appchainID)
}

func (d *DirectAdapter) MonitorIBTP() chan *pb.IBTP {
	return d.ibtpC
}

// QueryIBTP query ibtp from another pier
func (d *DirectAdapter) QueryIBTP(id string, isReq bool) (*pb.IBTP, error) {
	srcServiceID, destServiceID, index, err := utils.ParseIBTPID(id)
	if err != nil {
		return nil, err
	}
	if value, ok := d.ibtpCache.Get(index); ok {
		ibtp, ok := value.(*pb.IBTP)
		if !ok {
			d.ibtpCache.Remove(index)
			return nil, fmt.Errorf("get wrong type from ibtpCache")
		}
		// todo: Is it necessary to remove?
		d.ibtpCache.Remove(index)
		return ibtp, nil
	}

	var (
		serviceID string
		result    *pb.Message
	)
	// if isReq, targetPier is ibtpID filed From, the message type is Message_IBTP_GET
	if isReq {
		serviceID = srcServiceID
	} else {
		serviceID = destServiceID
	}
	pierID, err := utils.GetPierID(serviceID)
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_IBTP_GET, true, []byte(id))
	result, err = d.peerMgr.Send(pierID, msg)
	if err != nil {
		return nil, err
	}
	ibtp := &pb.IBTP{}
	if err := ibtp.Unmarshal(peermgr.DataToPayload(result).Data); err != nil {
		return nil, err
	}
	return ibtp, nil
}

// SendIBTP send ibtp to another pier
func (d *DirectAdapter) SendIBTP(ibtp *pb.IBTP) error {
	if ibtp.Type > 2 {
		return fmt.Errorf("unsupport ibtp type:%s", ibtp.Type)
	}
	data, err := ibtp.Marshal()
	if err != nil {
		panic(fmt.Sprintf("marshal ibtp: %s", err.Error()))
	}
	msg := peermgr.Message(pb.Message_IBTP_SEND, true, data)

	var serviceID string
	// if ibtp type is not interchain, target adr is From
	if ibtp.Type == pb.IBTP_INTERCHAIN {
		serviceID = ibtp.To
	} else {
		serviceID = ibtp.From
	}
	pierID, err := utils.GetPierID(serviceID)
	if err != nil {
		d.logger.Errorf("get pierID from ibtp err: %s", err.Error())
		return err
	}

	if err := d.peerMgr.AsyncSend(pierID, msg); err != nil {
		d.logger.Errorf("Send ibtp to pier %s: %s", ibtp.ID(), err.Error())
		return err
	}
	d.logger.Debugf("Send ibtp success from DirectAdapter")
	return nil
}

func (d *DirectAdapter) QueryInterchain(serviceID string) (*pb.Interchain, error) {
	pierID, err := utils.GetPierID(serviceID)
	if err != nil {
		return nil, err
	}
	msg := peermgr.Message(pb.Message_INTERCHAIN_META_GET, true, []byte(serviceID))
	result, err := d.peerMgr.Send(pierID, msg)
	if err != nil {
		return nil, err
	}

	interChain := &pb.Interchain{}
	if err := interChain.Unmarshal(peermgr.DataToPayload(result).Data); err != nil {
		return nil, err
	}
	return interChain, nil
}
