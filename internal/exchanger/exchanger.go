package exchanger

import (
	"context"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/agent"
	"github.com/meshplus/pier/internal/executor"
	"github.com/meshplus/pier/internal/monitor"
	"github.com/meshplus/pier/internal/peermgr"
	model "github.com/meshplus/pier/internal/peermgr/proto"
)

var logger = log.NewWithModule("exchanger")

type Exchanger struct {
	agent     agent.Agent
	ctx       context.Context
	mechanism int
	store     storage.Storage
	peerMgr   peermgr.PeerManager
	mnt       monitor.Monitor
	exec      executor.Executor
}

func New(ag agent.Agent, peerMgr peermgr.PeerManager,
	monitor monitor.Monitor, exec executor.Executor,
	store storage.Storage, typ int) (*Exchanger, error) {
	return &Exchanger{
		agent:     ag,
		peerMgr:   peerMgr,
		ctx:       context.Background(),
		mechanism: typ,
		store:     store,
		mnt:       monitor,
		exec:      exec,
	}, nil
}

func (ex *Exchanger) Start() error {
	// listen on monitor for ibtps
	if err := ex.peerMgr.RegisterMsgHandler(model.Message_IBTP, handleIBTPMessage); err != nil {
		return fmt.Errorf("register ibtp handler: %w", err)
	}

	go func() {
		for {
			select {
			case <-ex.ctx.Done():
				return
			case ibtp, ok := <-ex.mnt.ListenOnIBTP():
				if !ok {
					logger.Warn("Unexpected closed channel while listen on interchain ibtp")
					return
				}
				if err := ex.SendIBTP(ibtp); err != nil {
					logger.Info()
				}
			}
		}
	}()

	return nil
}

func handleIBTPMessage(network.Stream, *model.Message) {
	// put ibtp into validate engine

}

func (ex *Exchanger) Stop() error {
	return nil
}

func (ex *Exchanger) SendIBTP(ibtp *pb.IBTP) error {
	// todo: persist or not
	//if ibtp.Type == pb.IBTP_INTERCHAIN {
	//	b, err := ibtp.Marshal()
	//	if err != nil {
	//		return err
	//	}
	//	if err := ex.store.Put(ibtpKey(ibtp), b); err != nil {
	//		return err
	//	}
	//}
	switch ex.mechanism {
	case 0:
		if err := retry.Retry(func(attempt uint) error {
			receipt, err := ex.agent.SendIBTP(ibtp)
			if err != nil {
				return fmt.Errorf("send ibtp to bitxhub: %s", err.Error())
			}

			if !receipt.IsSuccess() {
				// todo: return ack
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	case 1:
		// send ibtp to another pier
		if err := retry.Retry(func(attempt uint) error {
			if err := ex.peerMgr.SendIBTP(ibtp); err != nil {
				return err
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	}
	return nil
}

func (ex *Exchanger) QueryIBTP(id string) (*pb.IBTP, error) {
	var (
		ibtp *pb.IBTP
		err  error
	)

	switch ex.mechanism {
	case 0:
		if err := retry.Retry(func(attempt uint) error {
			ibtp, err = ex.agent.GetIBTPByID(id)
			if err != nil {
				return fmt.Errorf("query ibtp from bitxhub: %s", err.Error())
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	case 1:
		// query ibtp from another pier
		if err := retry.Retry(func(attempt uint) error {
			if err := ex.peerMgr.GetIBTPByID(id); err != nil {
				return err
			}

			return nil
		}, strategy.Wait(1*time.Second)); err != nil {
			logger.Panic(err)
		}
	}
	return nil, nil
}

func (ex *Exchanger) QueryReceipt(id string) (*pb.IBTP, error) {
	return nil, nil
}

func ibtpKey(ibtp *pb.IBTP) []byte {
	return []byte(fmt.Sprintf("ibtp-%s-%s-%d", ibtp.From, ibtp.To, ibtp.Index))
}

func receiptKey(ibtp *pb.IBTP) string {
	return fmt.Sprintf("receipt-%s-%s-%d", ibtp.From, ibtp.To, ibtp.Index)
}
