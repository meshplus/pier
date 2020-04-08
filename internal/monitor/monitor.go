package monitor

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/pkg/plugins/client"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("monitor")

// Monitor receives event from blockchain and sends it to bitxhub
type AppchainMonitor struct {
	agent     agent.Agent
	client    client.Client
	meta      *rpcx.Appchain
	suspended uint64
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates monitor instance given agent of bitxhub, client interacting with appchain
// and meta about appchain.
func New(agent agent.Agent, client client.Client, meta *rpcx.Appchain) (*AppchainMonitor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	if meta.InterchainCounter == nil {
		meta.InterchainCounter = make(map[string]uint64)
	}
	return &AppchainMonitor{
		agent:  agent,
		client: client,
		meta:   meta,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start implements Monitor
func (m *AppchainMonitor) Start() error {
	err := m.recovery()
	if err != nil {
		return fmt.Errorf("recover monitor :%w", err)
	}

	go func() {
		for {
			select {
			case e := <-m.client.GetIBTP():
				for atomic.LoadUint64(&m.suspended) == 1 {
					time.Sleep(1 * time.Second)
				}

				m.handleIBTP(e)
			case <-m.ctx.Done():
				return
			}
		}
	}()

	if err := m.client.Start(); err != nil {
		return err
	}

	logger.Info("Monitor started")

	return nil
}

// recovery will recover interchain info from both bitxhub side and contract
// on appchain side in case pier is halted accidentally
func (m *AppchainMonitor) recovery() error {
	meta, err := m.client.GetOutMeta()
	if err != nil {
		return fmt.Errorf("get out meta from broker contract :%w", err)
	}
	for addr, index := range meta {
		beginIndex, ok := m.meta.InterchainCounter[addr]
		if !ok {
			beginIndex = 0
		}

		if err = m.handleMissingIBTP(addr, beginIndex+1, index+1); err != nil {
			logger.WithFields(logrus.Fields{
				"address": addr,
				"error":   err.Error(),
			}).Error("Handle missing ibtp")
		}
	}
	return nil
}

// Stop implements Monitor
func (m *AppchainMonitor) Stop() error {
	m.cancel()

	logger.Info("Monitor stopped")

	return m.client.Stop()
}

// handleIBTP handle the ibtp package captured by monitor.
// it will check the index of this package to filter duplicated packages and
// fetch unfinished ones
func (m *AppchainMonitor) handleIBTP(ibtp *pb.IBTP) {
	// m.meta.InterchainCounter[ibtp.To] is the index of top handled tx
	if m.meta.InterchainCounter[ibtp.To] >= ibtp.Index {
		logger.WithFields(logrus.Fields{
			"index":   ibtp.Index,
			"to":      ibtp.To,
			"ibtp_id": ibtp.ID(),
		}).Info("Ignore ibtp")
		return
	}

	if m.meta.InterchainCounter[ibtp.To]+1 < ibtp.Index {
		logger.WithFields(logrus.Fields{
			"index": ibtp.Index,
			"to":    types.String2Address(ibtp.To).ShortString(),
		}).Info("Get missing ibtp")

		if err := m.handleMissingIBTP(ibtp.To, m.meta.InterchainCounter[ibtp.To]+1, ibtp.Index); err != nil {
			logger.WithFields(logrus.Fields{
				"index": ibtp.Index,
				"to":    types.String2Address(ibtp.To).ShortString(),
			}).Error("Handle missing ibtp")
		}
	}

	m.send(ibtp)
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

		m.send(ev)
	}

	return nil
}

// send will send the ibtp package to bitxhub
func (m *AppchainMonitor) send(ibtp *pb.IBTP) {
	entry := logger.WithFields(logrus.Fields{
		"index": ibtp.Index,
		"type":  ibtp.Type,
		"to":    types.String2Address(ibtp.To).ShortString(),
		"id":    ibtp.ID(),
	})

	send := func() error {
		receipt, err := m.agent.SendIBTP(ibtp)
		if err != nil {
			return fmt.Errorf("send ibtp : %w", err)
		}

		if !receipt.IsSuccess() {
			entry.WithField("error", string(receipt.Ret)).Error("Send ibtp")
			return nil
		}

		if err := m.client.CommitCallback(ibtp); err != nil {
			return err
		}

		entry.WithFields(logrus.Fields{
			"hash": receipt.TxHash.Hex(),
		}).Info("Send ibtp")

		m.meta.InterchainCounter[ibtp.To] = ibtp.Index

		return nil
	}

	if err := retry.Retry(func(attempt uint) error {
		if err := send(); err != nil {
			entry.WithFields(logrus.Fields{
				"error": err,
			}).Error("Send IBTP")

			return err
		}

		return nil
	}, strategy.Wait(1*time.Second)); err != nil {
		panic(err.Error())
	}
}
