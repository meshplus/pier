package exchanger

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt/mock_adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	chain0 = "chain0"
	chain1 = "chain1"
	from   = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to     = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998"
	pierID = "0x892eedc032948be00722038a29f2a90d0e05352f"
)

var errorUnhappy = fmt.Errorf("nil")

func TestRelayMode(t *testing.T) {
	//testNormalStartRelay(t)
	//testRecoverErrorStartRelay(t)
}

func prepareRelay(t *testing.T) (
	*mock_adapt.MockAdapt, *mock_adapt.MockAdapt, *Exchanger,
) {
	mode := repo.RelayMode
	mockCtl := gomock.NewController(t)
	mockAdaptRelay := mock_adapt.NewMockAdapt(mockCtl)
	mockAdaptAppchain := mock_adapt.NewMockAdapt(mockCtl)
	mockExchanger, err := New(mode, "fabric", "1356", WithSrcAdapt(mockAdaptAppchain), WithDestAdapt(mockAdaptRelay), WithLogger(log.NewWithModule("exchanger")))
	require.Nil(t, err)
	return mockAdaptRelay, mockAdaptAppchain, mockExchanger
}

func testNormalStartRelay(t *testing.T) {
	mockAdaptRelay, mockAdaptAppchain, mockExchanger := prepareRelay(t)
	//meta := &pb.Interchain{}

	srcIBTPCh := make(chan *pb.IBTP)
	destIBTPCh := make(chan *pb.IBTP)
	mockAdaptRelay.EXPECT().MonitorIBTP().Return(destIBTPCh).AnyTimes()
	mockAdaptAppchain.EXPECT().MonitorIBTP().Return(srcIBTPCh).AnyTimes()
	mockAdaptRelay.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Name().Return("fabric").AnyTimes()
	mockAdaptAppchain.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().Name().Return("bitxhub").AnyTimes()

	//adapt0ServiceID_1 := fmt.Sprintf("1356:%s:%s", chain0, from)
	//adapt0ServiceID_2 := fmt.Sprintf("1356:%s:%s", chain0, from)
	//dstServiceID := fmt.Sprintf("1356:%s:%s", chain1, to)
	//servicePair := fmt.Sprintf("%s-%s", srcServiceID, dstServiceID)
	//
	//outMeta := make(map[string]uint64)
	//outMeta[servicePair] = 1
	//inMeta := make(map[string]uint64)
	//inMeta[servicePair] = 1
	//meta.InterchainCounter = map[string]uint64{srcServiceID: 1}

	mockAdaptRelay.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()

	mockAdaptAppchain.EXPECT().GetServiceIDList().Return([]string{"1356:fabric:transfer", "1356:fabric:data"}).AnyTimes()
	mockAdaptAppchain.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:transfer")).
		Return(&pb.Interchain{ID: "1356:fabric:transfer"}).AnyTimes()
	mockAdaptAppchain.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:data")).
		Return(&pb.Interchain{ID: "1356:fabric:transfer"}).AnyTimes()
	mockAdaptRelay.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:transfer")).
		Return(&pb.Interchain{ID: "1356:fabric:transfer"}).AnyTimes()
	mockAdaptRelay.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:data")).
		Return(&pb.Interchain{ID: "1356:fabric:transfer"}).AnyTimes()

	ibtps_src, _ := genIBTPs(t, 100, pb.IBTP_INTERCHAIN)

	ibtps_dest, _ := genIBTPs(t, 100, pb.IBTP_INTERCHAIN)

	//srcServiceMeta := make(map[string]*pb.Interchain)
	//destServiceMeta := make(map[string]*pb.Interchain)

	// test recover
	require.Nil(t, mockExchanger.Start())

	testRelayIBTPFromDest(t, destIBTPCh, ibtps_dest, mockExchanger)

	testRelayIBTPFromSrc(t, srcIBTPCh, ibtps_src, mockExchanger)

	// test for asset exchange ibtp
	time.Sleep(500 * time.Microsecond)
	close(destIBTPCh)
	close(srcIBTPCh)
	require.Nil(t, mockExchanger.Stop())
}

//func testRecoverErrorStartRelay(t *testing.T) {
//	mode := repo.RelayMode
//	mockAdaptRelay, mockAdaptAppchain, mockExchanger := prepareRelay(t)
//	meta := &pb.Interchain{}
//
//	outCh := make(chan *pb.IBTP)
//	inCh := make(chan *model.WrappedIBTP)
//
//	outMeta := make(map[string]uint64)
//	outMeta[to] = 1
//	inMeta := make(map[string]uint64)
//	inMeta[to] = 1
//	callbackMeta := make(map[string]uint64)
//	meta.InterchainCounter = map[string]uint64{to: 1}
//
//	mockMonitor.EXPECT().QueryOuterMeta().Return(outMeta).AnyTimes()
//	mockExecutor.EXPECT().QueryInterchainMeta().Return(inMeta).AnyTimes()
//	mockExecutor.EXPECT().QueryCallbackMeta().Return(callbackMeta).AnyTimes()
//	mockSyncer.EXPECT().ListenIBTP().Return(inCh).AnyTimes()
//	mockMonitor.EXPECT().ListenIBTP().Return(outCh).AnyTimes()
//
//	serviceMeta := make(map[string]*pb.Interchain)
//	serviceMeta[from] = meta
//	mockExchanger, err := New(mode, from, serviceMeta,
//		WithMonitor(mockMonitor), WithExecutor(mockExecutor),
//		WithSyncer(mockSyncer), WithLogger(log.NewWithModule("exchanger")),
//		WithChecker(mockChecker), WithStorage(store),
//	)
//	require.Nil(t, err)
//
//	mockSyncer.EXPECT().RegisterRollbackHandler(gomock.Any()).Return(nil).AnyTimes()
//	syncerError := fmt.Errorf("start syncer failed")
//	mockSyncer.EXPECT().Start().Return(syncerError)
//	require.Equal(t, true, errors.Is(mockExchanger.Start(), syncerError))
//
//	mockSyncer.EXPECT().Start().Return(nil).AnyTimes()
//	// mock recover error
//	mockSyncer.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(nil, false, syncer.ErrIBTPNotFound).MaxTimes(1)
//	mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(fmt.Errorf("send ibtp receipt error")).MaxTimes(2)
//	require.Panics(t, func() { mockExchanger.Start() })
//
//	// test for recovering ibtp
//	ibtp := getIBTP(t, 1, pb.IBTP_RECEIPT_SUCCESS)
//	mockMonitor.EXPECT().QueryIBTP(gomock.Any()).Return(nil, fmt.Errorf("query ibtp error")).MaxTimes(10)
//	mockMonitor.EXPECT().QueryIBTP(gomock.Any()).Return(ibtp, nil).MaxTimes(2)
//
//	//outMeta[to] = 0
//
//	//require.Panics(t, func() { mockExchanger.Start() })
//
//	// test for recovering ibtp receipt
//	//mockSyncer.EXPECT().QueryIBTP(gomock.Any()).Return(ibtp, nil).AnyTimes()
//	//mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).
//	//	Return(nil, fmt.Errorf("query ibtp receipt error"))
//	//require.Panics(t, func() { mockExchanger.Start() })
//
//	//mockExecutor.EXPECT().QueryIBTPReceipt(gomock.Any()).
//	//	Return(&pb.IBTP{}, nil).AnyTimes()
//	//mockSyncer.EXPECT().SendIBTP(gomock.Any()).Return(fmt.Errorf("send ibtp receipt error"))
//	//require.Panics(t, func() { mockExchanger.Start() })
//}

func testRelayIBTPFromSrc(t *testing.T, outCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			outCh <- ibtp
		}
	}
	time.Sleep(100 * time.Millisecond)
	exchanger.listenIBTPFromSrcAdapt()
	require.Equal(t, uint64(100), exchanger.srcServiceMeta[from].InterchainCounter[ibtps[0].To])
}

func testRelayIBTPFromDest(t *testing.T, inCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			inCh <- ibtp
		}
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.destServiceMeta[to].SourceInterchainCounter[ibtps[0].From])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS)
	for _, receipt := range ibtpReceipts {
		inCh <- receipt
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.destServiceMeta[from].ReceiptCounter[ibtps[0].To])
}

func genIBTPs(t *testing.T, count int, typ pb.IBTP_Type) ([]*pb.IBTP, map[string]*pb.IBTP) {
	ibtps := make([]*pb.IBTP, 0, count)
	ibtpM := make(map[string]*pb.IBTP, count)
	for i := 1; i <= count; i++ {
		ibtp := getIBTP(t, uint64(i), typ)
		ibtps = append(ibtps, ibtp)
		ibtpM[ibtp.ID()] = ibtp
	}
	return ibtps, ibtpM
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type) *pb.IBTP {
	ct := &pb.Content{
		Func: "set",
		Args: [][]byte{[]byte("Alice")},
	}
	c, err := ct.Marshal()
	require.Nil(t, err)

	pd := pb.Payload{
		Encrypted: false,
		Content:   c,
	}
	ibtppd, err := pd.Marshal()
	require.Nil(t, err)

	return &pb.IBTP{
		From:    fmt.Sprintf("1356:chain0:%s", from),
		To:      fmt.Sprintf("1356:chain1:%s", to),
		Payload: ibtppd,
		Index:   index,
		Type:    typ,
	}
}

func TestUnionMode(t *testing.T) {

}

func TestDirectMode(t *testing.T) {

}
