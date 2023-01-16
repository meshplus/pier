package exchanger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

type Exchanger struct {
	mode       string
	srcChainId string
	srcBxhId   string
	// R: appchain -- hub
	// D: appchain -- dPier
	// U: hub 	   -- uPier
	srcAdapt        adapt.Adapt
	destAdapt       adapt.Adapt
	srcAdaptName    string
	destAdaptName   string
	srcServiceMeta  map[string]*pb.Interchain
	destServiceMeta map[string]*pb.Interchain

	srcIBTPMap  map[string]chan *pb.IBTP
	destIBTPMap map[string]chan *pb.IBTP

	sendIBTPCounter atomic.Uint64
	sendIBTPTimer   atomic.Duration
	logger          logrus.FieldLogger
	ctx             context.Context
	cancel          context.CancelFunc
}

func New(typ, srcChainId, srcBxhId string, opts ...Option) (*Exchanger, error) {
	config := GenerateConfig(opts...)

	ctx, cancel := context.WithCancel(context.Background())
	exchanger := &Exchanger{
		srcChainId:      srcChainId,
		srcBxhId:        srcBxhId,
		srcAdapt:        config.srcAdapt,
		destAdapt:       config.destAdapt,
		logger:          config.logger,
		srcServiceMeta:  make(map[string]*pb.Interchain),
		destServiceMeta: make(map[string]*pb.Interchain),
		srcIBTPMap:      make(map[string]chan *pb.IBTP),
		destIBTPMap:     make(map[string]chan *pb.IBTP),
		mode:            typ,
		ctx:             ctx,
		cancel:          cancel,
	}
	return exchanger, nil
}

func ParseFullServiceID(id string) (string, string, string, error) {
	splits := strings.Split(id, ":")
	if len(splits) != 3 {
		return "", "", "", fmt.Errorf("invalid full service ID: %s", id)
	}
	return splits[0], splits[1], splits[2], nil
}

func (ex *Exchanger) checkService(appServiceList, bxhServiceList []string) error {
	appServiceM := make(map[string]struct{}, len(appServiceList))
	for _, fullServiceID := range appServiceList {
		_, _, s, err := ParseFullServiceID(fullServiceID)
		if err != nil {
			ex.logger.Errorf("ParseFullServiceID err:%s", err)
			return err
		}
		appServiceM[s] = struct{}{}
	}
	for _, serviceId := range bxhServiceList {
		if _, ok := appServiceM[serviceId]; !ok {
			return fmt.Errorf("service:[%s] has been registered in bitxhub, "+
				"but not registered in broker contract", serviceId)
		}
	}
	return nil
}

func (ex *Exchanger) Start() error {
	// init meta info
	var (
		serviceList []string
		err         error
	)

	// start get ibtp to channel
	if err := ex.srcAdapt.Start(); err != nil {
		return err
	}

	if err := ex.destAdapt.Start(); err != nil {
		return err
	}

	ex.srcAdaptName = ex.srcAdapt.Name()
	ex.destAdaptName = ex.destAdapt.Name()

	if err := retry.Retry(func(attempt uint) error {
		if serviceList, err = ex.srcAdapt.GetServiceIDList(); err != nil {
			ex.logger.Errorf("get serviceIdList from srcAdapt", "error", err.Error())
			return err
		}
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		return fmt.Errorf("retry error to get serviceIdList from srcAdapt: %w", err)
	}

	for _, serviceId := range serviceList {
		ex.srcServiceMeta[serviceId], err = ex.srcAdapt.QueryInterchain(serviceId)
		if err != nil {
			return fmt.Errorf("queryInterchain from srcAdapt: %w", err)
		}

		if err := retry.Retry(func(attempt uint) error {
			if ex.destServiceMeta[serviceId], err = ex.destAdapt.QueryInterchain(serviceId); err != nil {
				// maybe peerMgr err cause QueryInterchain err, so retry it
				ex.logger.Errorf("queryInterchain from destAdapt: %w", err)
			}
			return err
		}, strategy.Backoff(backoff.Fibonacci(1*time.Second))); err != nil {
			ex.logger.Errorf("retry err with queryInterchain: %w", err)
		}
	}

	if repo.RelayMode == ex.mode {
		bxhServiceList := make([]string, 0)
		if err = retry.Retry(func(attempt uint) error {
			bxhServiceList, err = ex.destAdapt.GetServiceIDList()
			if err != nil {
				ex.logger.Errorf("bxhAdapter GetServiceIDList err:%s", err)
				return err
			}
			return nil
		}, strategy.Wait(2*time.Second)); err != nil {
			return err
		}

		err = ex.checkService(serviceList, bxhServiceList)
		if err != nil {
			panic(err)
		}
	}

	if repo.UnionMode == ex.mode {
		ex.recoverUnion(ex.srcServiceMeta, ex.destServiceMeta)
		// add self_interchains to srcServiceMeta
		ex.fillSelfInterchain()
	} else {
		ex.recover(ex.srcServiceMeta, ex.destServiceMeta)
	}

	// start consumer
	go ex.listenIBTPFromSrcAdaptToServicePairCh()
	go ex.listenIBTPFromDestAdaptToServicePairCh()

	ex.logger.Info("Exchanger started")
	//go ex.analysisDirectTPS()
	return nil
}

func (ex *Exchanger) fillSelfInterchain() {
	result := make(map[string]*pb.Interchain)
	for _, v := range ex.srcServiceMeta {
		for s, _ := range v.InterchainCounter {
			interchain, err := ex.srcAdapt.QueryInterchain(s)
			if err != nil {
				panic(fmt.Sprintf("queryInterchain from srcAdapt: %s", err.Error()))
			}
			result[interchain.ID] = interchain
		}
		for s, _ := range v.ReceiptCounter {
			interchain, err := ex.srcAdapt.QueryInterchain(s)
			if err != nil {
				panic(fmt.Sprintf("queryInterchain from srcAdapt: %s", err.Error()))
			}
			result[interchain.ID] = interchain
		}
		for s, _ := range v.SourceInterchainCounter {
			interchain, err := ex.srcAdapt.QueryInterchain(s)
			if err != nil {
				panic(fmt.Sprintf("queryInterchain from srcAdapt: %s", err.Error()))
			}
			result[interchain.ID] = interchain
		}
		for s, _ := range v.SourceReceiptCounter {
			interchain, err := ex.srcAdapt.QueryInterchain(s)
			if err != nil {
				panic(fmt.Sprintf("queryInterchain from srcAdapt: %s", err.Error()))
			}
			result[interchain.ID] = interchain
		}
	}
	for k, v := range result {
		ex.srcServiceMeta[k] = v
		ex.destServiceMeta[k] = v
	}
}

func initInterchain(serviceMeta map[string]*pb.Interchain, fullServiceId string) *pb.Interchain {
	serviceMeta[fullServiceId] = &pb.Interchain{
		ID:                      fullServiceId,
		InterchainCounter:       make(map[string]uint64),
		ReceiptCounter:          make(map[string]uint64),
		SourceInterchainCounter: make(map[string]uint64),
		SourceReceiptCounter:    make(map[string]uint64),
	}
	return serviceMeta[fullServiceId]
}

func (ex *Exchanger) listenIBTPFromDestAdaptToServicePairCh() {
	ex.logger.Infof("listenIBTPFromDestAdaptToServicePairCh %s Start!", ex.destAdaptName)
	ch := ex.destAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("listenIBTPFromDestAdaptToServicePairCh Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			key := ibtp.From + ibtp.To
			_, ok2 := ex.destIBTPMap[key]
			if !ok2 {
				ex.destIBTPMap[key] = make(chan *pb.IBTP, 40960)
				if strings.EqualFold(repo.RelayMode, ex.mode) {
					go ex.listenIBTPFromDestAdaptForRelay(key)
				} else if strings.EqualFold(repo.DirectMode, ex.mode) {
					go ex.listenIBTPFromDestAdaptForDirect(key)
				} else {
					go ex.listenIBTPFromDestAdapt(key)
				}
			}
			ex.destIBTPMap[key] <- ibtp

		}
	}
}
func (ex *Exchanger) listenIBTPFromDestAdapt(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromDestAdapt Stop!")
			return
		case ibtp, ok := <-ex.destIBTPMap[servicePair]:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.destAdaptName)
			index := ex.getCurrentIndexFromDest(ibtp)
			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.destAdapt, ex.srcAdapt, ibtp.From, ibtp.To, !ex.isIBTPBelongSrc(ibtp))
			}
			if err := retry.Retry(func(attempt uint) error {
				ex.logger.Infof("start sendIBTP to adapter: %s", ex.srcAdaptName)
				if err := ex.srcAdapt.SendIBTP(ibtp); err != nil {
					ex.logger.Errorf("send IBTP to Adapt, from:%s, error:%s", ex.srcAdaptName, err.Error())
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.destAdapt, ibtp.ID(), !ex.isIBTPBelongSrc(ibtp))
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}
			if ex.isIBTPBelongSrc(ibtp) {
				ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To] = ibtp.Index
			} else {
				ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) listenIBTPFromSrcAdaptToServicePairCh() {
	ex.logger.Infof("listenIBTPFromSrcAdaptToServicePairCh %s Start!", ex.srcAdaptName)
	ch := ex.srcAdapt.MonitorIBTP()
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("listenIBTPFromSrcAdaptToServicePairCh Stop!")
			return
		case ibtp, ok := <-ch:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "typ": ibtp.Type, "timestamp": ibtp.Timestamp}).
				Info("[step1] Receive ibtp from plugin")
			key := ibtp.From + ibtp.To
			_, ok2 := ex.srcIBTPMap[key]
			if !ok2 {
				ex.srcIBTPMap[key] = make(chan *pb.IBTP, 40960)
				if strings.EqualFold(repo.RelayMode, ex.mode) {
					go ex.listenIBTPFromSrcAdaptForRelay(key)
				} else if strings.EqualFold(repo.DirectMode, ex.mode) {
					go ex.listenIBTPFromSrcAdaptForDirect(key)
				} else {
					go ex.listenIBTPFromSrcAdapt(key)
				}
			}
			ex.srcIBTPMap[key] <- ibtp
		}
	}
}
func (ex *Exchanger) listenIBTPFromSrcAdapt(servicePair string) {
	for {
		select {
		case <-ex.ctx.Done():
			ex.logger.Info("ListenIBTPFromSrcAdapt Stop!")
			return
		case ibtp, ok := <-ex.srcIBTPMap[servicePair]:
			if !ok {
				ex.logger.Warn("Unexpected closed channel while listening on interchain ibtp")
				return
			}
			ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "type": ibtp.Type, "ibtp_id": ibtp.ID()}).Info("Receive ibtp from :", ex.srcAdaptName)
			index := ex.getCurrentIndexFromSrc(ibtp)
			if index >= ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to_counter": index, "ibtp_id": ibtp.ID()}).Info("Ignore ibtp")
				continue
			}

			if index+1 < ibtp.Index {
				ex.logger.WithFields(logrus.Fields{"index": ibtp.Index, "to": ibtp.To}).Info("Get missing ibtp")
				ex.handleMissingIBTPByServicePair(index+1, ibtp.Index-1, ex.srcAdapt, ex.destAdapt, ibtp.From, ibtp.To, ex.isIBTPBelongSrc(ibtp))
			}
			if err := retry.Retry(func(attempt uint) error {
				if err := ex.destAdapt.SendIBTP(ibtp); err != nil {
					// if err occurs, try to get new ibtp and resend
					if err, ok := err.(*adapt.SendIbtpError); ok {
						if err.NeedRetry() {
							ex.logger.Errorf("send IBTP to Adapt:%s err:%s", ex.destAdaptName, err.Error())
							// query to new ibtp
							ibtp = ex.queryIBTP(ex.srcAdapt, ibtp.ID(), ex.isIBTPBelongSrc(ibtp))
							return fmt.Errorf("retry sending ibtp")
						}
					}
				}
				return nil
			}, strategy.Backoff(backoff.Fibonacci(500*time.Millisecond))); err != nil {
				ex.logger.Panic(err)
			}

			if ex.isIBTPBelongSrc(ibtp) {
				ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To] = ibtp.Index
			} else {
				ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From] = ibtp.Index
			}
		}
	}
}

func (ex *Exchanger) getCurrentIndexFromDest(ibtp *pb.IBTP) uint64 {
	var index uint64
	if ex.isIBTPBelongSrc(ibtp) {
		_, ok := ex.destServiceMeta[ibtp.From]
		if !ok {
			initInterchain(ex.destServiceMeta, ibtp.From)
		}
		index = ex.destServiceMeta[ibtp.From].ReceiptCounter[ibtp.To]
	} else {
		_, ok := ex.destServiceMeta[ibtp.To]
		if !ok {
			initInterchain(ex.destServiceMeta, ibtp.To)
		}
		index = ex.destServiceMeta[ibtp.To].SourceInterchainCounter[ibtp.From]
	}
	return index
}

func (ex *Exchanger) getCurrentIndexFromSrc(ibtp *pb.IBTP) uint64 {
	var index uint64
	if ex.isIBTPBelongSrc(ibtp) {
		_, ok := ex.srcServiceMeta[ibtp.From]
		if !ok {
			initInterchain(ex.srcServiceMeta, ibtp.From)
		}
		index = ex.srcServiceMeta[ibtp.From].InterchainCounter[ibtp.To]
	} else {
		_, ok := ex.srcServiceMeta[ibtp.To]
		if !ok {
			initInterchain(ex.srcServiceMeta, ibtp.To)
		}
		index = ex.srcServiceMeta[ibtp.To].SourceReceiptCounter[ibtp.From]
	}
	return index
}

func (ex *Exchanger) isIBTPBelongSrc(ibtp *pb.IBTP) bool {
	var isIBTPBelongSrc = false
	bxhID, chainID, _ := ibtp.ParseFrom()

	switch ex.mode {
	case repo.DirectMode:
		fallthrough
	case repo.RelayMode:
		if strings.EqualFold(ex.srcChainId, chainID) {
			isIBTPBelongSrc = true
		}
	case repo.UnionMode:
		if strings.EqualFold(ex.srcBxhId, bxhID) {
			isIBTPBelongSrc = true
		}
	}
	return isIBTPBelongSrc
}

func (ex *Exchanger) queryIBTP(adapt adapt.Adapt, ibtpID string, isReq bool) *pb.IBTP {
	var (
		ibtp *pb.IBTP
		err  error
	)
	if err := retry.Retry(func(attempt uint) error {
		ibtp, err = adapt.QueryIBTP(ibtpID, isReq)
		if err != nil {
			ex.logger.Errorf("queryIBTP from Adapt:%s, error: %v", adapt.Name(), err.Error())
			return err
		}
		return nil
	}, strategy.Wait(3*time.Second)); err != nil {
		ex.logger.Panic(err)
	}
	return ibtp
}

func (ex *Exchanger) Stop() error {
	ex.cancel()

	if err := ex.srcAdapt.Stop(); err != nil {
		return fmt.Errorf("srcAdapt stop: %w", err)
	}
	if err := ex.destAdapt.Stop(); err != nil {
		return fmt.Errorf("destAdapt stop: %w", err)
	}
	ex.logger.Info("Exchanger stopped")

	return nil
}

//func (ex *Exchanger) analysisDirectTPS() {
//	ticker := time.NewTicker(time.Second)
//	defer ticker.Stop()
//
//	current := time.Now()
//	counter := ex.sendIBTPCounter.Load()
//	for {
//		select {
//		case <-ticker.C:
//			tps := ex.sendIBTPCounter.Load() - counter
//			counter = ex.sendIBTPCounter.Load()
//			totalTimer := ex.sendIBTPTimer.Load()
//
//			if tps != 0 {
//				ex.logger.WithFields(logrus.Fields{
//					"tps":      tps,
//					"tps_sum":  counter,
//					"tps_time": totalTimer.Milliseconds() / int64(counter),
//					"tps_avg":  float64(counter) / time.Since(current).Seconds(),
//				}).Warn("analysis")
//			}
//
//		case <-ex.ctx.Done():
//			return
//		}
//	}
//}
//
//func (ex *Exchanger) timeCost() func() {
//	start := time.Now()
//	return func() {
//		tc := time.Since(start)
//		ex.sendIBTPTimer.Add(tc)
//	}
//}
