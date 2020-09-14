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

// Monitor receives event from blockchain and sends it to network
type AppchainMonitor struct {
	client            plugins.Client
	interchainCounter map[string]uint64
	recvCh            chan *pb.IBTP
	suspended         uint64
	ctx               context.Context
	cancel            context.CancelFunc
	cryptor           txcrypto.Cryptor
}

// New creates monitor instance given client interacting with appchain and interchainCounter about appchain.
func New(client plugins.Client, cryptor txcrypto.Cryptor) (*AppchainMonitor, error) {
	meta, err := client.GetOutMeta()
	if err != nil {
		return nil, fmt.Errorf("get out interchainCounter from broker contract :%w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &AppchainMonitor{
		client:            client,
		interchainCounter: meta,
		cryptor:           cryptor,
		recvCh:            make(chan *pb.IBTP, 30720),
		ctx:               ctx,
		cancel:            cancel,
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
				//for atomic.LoadUint64(&m.suspended) == 1 {
				//	time.Sleep(1 * time.Second)
				//}

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

func (m *AppchainMonitor) ListenOnIBTP() chan *pb.IBTP {
	return m.recvCh
}

func (m *AppchainMonitor) QueryIBTP(id string) (*pb.IBTP, error) {
	args := strings.Split(id, "-")
	if len(args) != 3 {
		return nil, fmt.Errorf("invalid ibtp id %s", id)
	}

	idx, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return nil, err
	}

	ibtp, err := m.getIBTP(args[1], idx)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"id": id,
		}).Error("query ibtp from contract")
	}

	if err := m.checkEnrcyption(ibtp); err != nil {
		return nil, err
	}
	return ibtp, nil
}

func (m *AppchainMonitor) QueryLatestMeta() map[string]uint64 {
	return m.interchainCounter
}

// handleIBTP handle the ibtp package captured by monitor.
// it will check the index of this package to filter duplicated packages and
// fetch unfinished ones
func (m *AppchainMonitor) handleIBTP(ibtp *pb.IBTP) {
	// m.interchainCounter.InterchainCounter[ibtp.To] is the index of top handled tx
	if m.interchainCounter[ibtp.To] >= ibtp.Index {
		logger.WithFields(logrus.Fields{
			"index":      ibtp.Index,
			"to counter": m.interchainCounter[ibtp.To],
			"ibtp_id":    ibtp.ID(),
		}).Info("Ignore ibtp")
		return
	}

	if m.interchainCounter[ibtp.To]+1 < ibtp.Index {
		logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"to":    ibtp.To,
		}).Info("Get missing ibtp")

		if err := m.handleMissingIBTP(ibtp.To, m.interchainCounter[ibtp.To]+1, ibtp.Index); err != nil {
			logger.WithFields(logrus.Fields{
				"index": ibtp.Index,
				"to":    ibtp.To,
			}).Error("Handle missing ibtp")
		}
	}

	if err := m.checkEnrcyption(ibtp); err != nil {
		logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"to":    ibtp.To,
		}).Error("check encryption")
	}

	logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"to":    ibtp.To,
	}).Info("Pass ibtp to exchanger")

	m.interchainCounter[ibtp.To]++
	m.recvCh <- ibtp
}

// getIBTP gets interchain tx recorded in appchain given destination chain and index
func (m *AppchainMonitor) getIBTP(to string, idx uint64) (*pb.IBTP, error) {
	c := make(chan *pb.IBTP, 1)
	if err := retry.Retry(func(attempt uint) error {
		e, err := m.client.GetOutMessage(to, idx)
		if err != nil {
			logger.Errorf("Get out message : %s", err.Error())
			return err
		}

		c <- e

		return nil
	}, strategy.Wait(2*time.Second)); err != nil {
		return nil, err
	}

	return <-c, nil
}

// handleMissingIBTP gets already-thrown-out interchain ibtps from appchain.
// On appchain, with destination-chain address and begin and end index
// this will get events in the range [begin, end)
func (m *AppchainMonitor) handleMissingIBTP(to string, begin, end uint64) error {
	if begin < 1 {
		return fmt.Errorf("begin index for missing ibtp is required >= 1")
	}
	for ; begin < end; begin++ {
		logger.WithFields(logrus.Fields{
			"to":    to,
			"index": begin,
		}).Info("Get missing event from contract")

		ev, err := m.getIBTP(to, begin)
		if err != nil {
			return fmt.Errorf("fetch ibtp:%w", err)
		}

		if err = m.checkEnrcyption(ev); err != nil {
			return fmt.Errorf("check enrcyption:%w", err)
		}

		m.interchainCounter[ev.To]++
		m.recvCh <- ev
	}

	return nil
}

func (m *AppchainMonitor) checkEnrcyption(ibtp *pb.IBTP) error {
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
