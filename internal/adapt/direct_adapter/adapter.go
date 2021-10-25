package direct_adapter

import (
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/peermgr"
	"github.com/meshplus/pier/internal/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"sync"
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
	remotePierID  string
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

	// todo: support multi peers
	connectedNum := d.peerMgr.CountConnectedPeers()
	if connectedNum != 1 {
		return fmt.Errorf("direct adapter connect just 1 remote pier, the actual remote num is : %d",
			connectedNum)
	}
	for pierID, _ := range d.peerMgr.Peers() {
		d.remotePierID = pierID
	}

	d.logger.Info("direct adapter start")

	return nil
}

func (d *DirectAdapter) Stop() error {
	err := d.peerMgr.Stop()
	if err != nil {
		return err
	}

	close(d.ibtpC)
	d.ibtpC = nil

	d.logger.Info("direct adapter stopped")
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
	_, _, index, err := utils.ParseIBTPID(id)
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

	var result *pb.Message
	msg := peermgr.Message(pb.Message_IBTP_GET, true, []byte(id))
	result, err = d.peerMgr.Send(d.remotePierID, msg)
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
	if ibtp.Type != pb.IBTP_INTERCHAIN && ibtp.Type != pb.IBTP_RECEIPT_SUCCESS && ibtp.Type != pb.IBTP_RECEIPT_FAILURE {
		return fmt.Errorf("unsupport ibtp type:%s", ibtp.Type)
	}
	data, err := ibtp.Marshal()
	if err != nil {
		panic(fmt.Sprintf("marshal ibtp: %s", err.Error()))
	}
	msg := peermgr.Message(pb.Message_IBTP_SEND, true, data)

	if err := d.peerMgr.AsyncSend(d.remotePierID, msg); err != nil {
		d.logger.WithFields(logrus.Fields{
			"ibtpID": ibtp.ID(),
			"error":  err.Error(),
		}).Errorf("Direct adapter peerMgr send ibtp to remote pier err")
		return err
	}
	d.logger.WithFields(logrus.Fields{
		"ibypID": ibtp.ID(),
	}).Info("Direct adapter Send ibtp success")
	return nil
}

func (d *DirectAdapter) QueryInterchain(serviceID string) (*pb.Interchain, error) {
	msg := peermgr.Message(pb.Message_INTERCHAIN_META_GET, true, []byte(serviceID))
	result, err := d.peerMgr.Send(d.remotePierID, msg)
	if err != nil {
		return nil, err
	}

	interChain := &pb.Interchain{}
	if err := interChain.Unmarshal(peermgr.DataToPayload(result).Data); err != nil {
		return nil, err
	}
	return interChain, nil
}
