package monitor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/txcrypto"
	"github.com/meshplus/pier/pkg/plugins"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("monitor")

var _ Monitor = (*AppchainMonitor)(nil)

// Monitor receives event from blockchain and sends it to network
type AppchainMonitor struct {
	client    plugins.Client
	recvCh    chan *pb.IBTP
	suspended uint64
	ctx       context.Context
	cancel    context.CancelFunc
	cryptor   txcrypto.Cryptor
}

// New creates monitor instance given client interacting with appchain and interchainCounter about appchain.
func New(client plugins.Client, cryptor txcrypto.Cryptor) (*AppchainMonitor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &AppchainMonitor{
		client:  client,
		cryptor: cryptor,
		recvCh:  make(chan *pb.IBTP, 1024),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start implements Monitor
func (m *AppchainMonitor) Start() error {
	if err := m.client.Start(); err != nil {
		return err
	}

	ch := m.client.GetIBTP()
	go func() {
		for {
			select {
			case e := <-ch:
				m.handleIBTP(e)
			case <-m.ctx.Done():
				return
			}
		}
	}()
	logger.Info("Monitor started")
	return nil
}

// Stop implements Monitor
func (m *AppchainMonitor) Stop() error {
	m.cancel()
	logger.Info("Monitor stopped")
	return m.client.Stop()
}

func (m *AppchainMonitor) ListenIBTP() <-chan *pb.IBTP {
	return m.recvCh
}

// QueryIBTP queries interchain tx recorded in appchain given ibtp id
func (m *AppchainMonitor) QueryIBTP(id string) (*pb.IBTP, error) {
	// TODO(xcc): Encapsulate as a function
	args := strings.Split(id, "-")
	if len(args) != 3 {
		return nil, fmt.Errorf("invalid ibtp id %s", id)
	}

	idx, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ibtp index")
	}

	c := make(chan *pb.IBTP, 1)
	if err := retry.Retry(func(attempt uint) error {
		// TODO(xcc): Need to distinguish error types
		e, err := m.client.GetOutMessage(args[1], idx)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"error":   err,
				"ibtp_id": id,
			}).Error("Query ibtp")
			return err
		}
		c <- e
		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	ibtp := <-c

	if err := m.encryption(ibtp); err != nil {
		return nil, err
	}

	return ibtp, nil
}

// QueryOuterMeta queries outer meta from appchain.
// It will loop until the result is returned or panic.
func (m *AppchainMonitor) QueryOuterMeta() map[string]uint64 {
	ret := make(map[string]uint64)
	if err := retry.Retry(func(attempt uint) error {
		meta, err := m.client.GetOutMeta()
		if err != nil {
			logger.WithField("error", err).Error("Get outer meta from appchain")
			return err
		}
		ret = meta
		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		panic(err)
	}

	return ret
}

// handleIBTP handle the ibtp package captured by monitor.
func (m *AppchainMonitor) handleIBTP(ibtp *pb.IBTP) {
	if err := m.encryption(ibtp); err != nil {
		logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"to":    ibtp.To,
		}).Error("check encryption")
		return
	}

	m.recvCh <- ibtp
}

func (m *AppchainMonitor) encryption(ibtp *pb.IBTP) error {
	pld := &pb.Payload{}
	if err := pld.Unmarshal(ibtp.Payload); err != nil {
		return err
	}
	if !pld.Encrypted {
		return nil
	}

	ctb, err := m.cryptor.Encrypt(pld.Content, ibtp.To)
	if err != nil {
		return err
	}
	pld.Content = ctb
	payload, err := pld.Marshal()
	if err != nil {
		return err
	}
	ibtp.Payload = payload

	return nil
}
