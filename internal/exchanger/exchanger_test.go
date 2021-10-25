package exchanger

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/meshplus/bitxhub-kit/hexutil"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt/mock_adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

const (
	chain0          = "chain0"
	chain1          = "chain1"
	from            = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
	to              = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998"
	pierID          = "0x892eedc032948be00722038a29f2a90d0e05352f"
	fullFromService = "1356:fabric:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998"
	fullToService   = "1356:ether:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
)

var errorUnhappy = fmt.Errorf("nil")

func TestRelayMode(t *testing.T) {
	testNormalStartRelay(t)
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

	srcIBTPCh := make(chan *pb.IBTP)
	destIBTPCh := make(chan *pb.IBTP)
	mockAdaptRelay.EXPECT().MonitorIBTP().Return(destIBTPCh).AnyTimes()
	mockAdaptAppchain.EXPECT().MonitorIBTP().Return(srcIBTPCh).AnyTimes()
	mockAdaptRelay.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Name().Return("bitxhub").AnyTimes()
	mockAdaptAppchain.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().Name().Return("fabric").AnyTimes()

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

	mockAdaptRelay.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(&pb.IBTP{}, nil).AnyTimes()
	mockAdaptAppchain.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(&pb.IBTP{}, nil).AnyTimes()

	mockAdaptRelay.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()
	mockAdaptAppchain.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()

	mockAdaptAppchain.EXPECT().GetServiceIDList().Return([]string{fullFromService}, nil).AnyTimes()
	mockAdaptAppchain.EXPECT().QueryInterchain(gomock.Eq(fullFromService)).
		Return(&pb.Interchain{ID: fullFromService,
			SourceInterchainCounter: make(map[string]uint64),
			ReceiptCounter:          make(map[string]uint64),
			SourceReceiptCounter:    map[string]uint64{fullToService: 1},
			InterchainCounter:       map[string]uint64{fullToService: 1}}, nil).AnyTimes()
	//mockAdaptAppchain.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:data")).
	//	Return(&pb.Interchain{ID: "1356:fabric:transfer"}, nil).AnyTimes()
	mockAdaptRelay.EXPECT().QueryInterchain(gomock.Eq(fullFromService)).
		Return(&pb.Interchain{ID: fullFromService,
			SourceInterchainCounter: map[string]uint64{fullToService: 1},
			ReceiptCounter:          map[string]uint64{fullToService: 1},
			SourceReceiptCounter:    make(map[string]uint64),
			InterchainCounter:       make(map[string]uint64)}, nil).AnyTimes()
	//mockAdaptRelay.EXPECT().QueryInterchain(gomock.Eq("1356:fabric:data")).
	//	Return(&pb.Interchain{ID: "1356:fabric:transfer"}, nil).AnyTimes()

	ibtps_src, _ := genIBTPs(t, 100, pb.IBTP_INTERCHAIN, false)

	ibtps_dest, _ := genIBTPs(t, 100, pb.IBTP_INTERCHAIN, true)

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
	require.Equal(t, uint64(100), exchanger.srcServiceMeta[ibtps[0].From].InterchainCounter[ibtps[0].To])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS, false)
	for _, receipt := range ibtpReceipts {
		outCh <- receipt
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.srcServiceMeta[ibtpReceipts[0].To].SourceReceiptCounter[ibtpReceipts[0].From])
}

func testRelayIBTPFromDest(t *testing.T, inCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			inCh <- ibtp
		}
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.destServiceMeta[ibtps[0].To].SourceInterchainCounter[ibtps[0].From])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS, true)
	for _, receipt := range ibtpReceipts {
		inCh <- receipt
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, uint64(100), exchanger.destServiceMeta[ibtpReceipts[0].From].ReceiptCounter[ibtpReceipts[0].To])
}

func genIBTPs(t *testing.T, count int, typ pb.IBTP_Type, isDestToSrc bool) ([]*pb.IBTP, map[string]*pb.IBTP) {
	ibtps := make([]*pb.IBTP, 0, count)
	ibtpM := make(map[string]*pb.IBTP, count)
	for i := 1; i <= count; i++ {
		ibtp := getIBTP(t, uint64(i), typ, isDestToSrc)
		ibtps = append(ibtps, ibtp)
		ibtpM[ibtp.ID()] = ibtp
	}
	return ibtps, ibtpM
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type, isDestToSrc bool) *pb.IBTP {
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
	if strings.EqualFold(typ.String(), pb.IBTP_INTERCHAIN.String()) && !isDestToSrc {
		return &pb.IBTP{
			From:    fmt.Sprintf("1356:fabric:%s1", to),
			To:      fmt.Sprintf("1356:ether:%s", from),
			Payload: ibtppd,
			Index:   index,
			Type:    typ,
		}
	}

	if strings.EqualFold(typ.String(), pb.IBTP_RECEIPT_SUCCESS.String()) && !isDestToSrc {
		return &pb.IBTP{
			From:    fmt.Sprintf("1356:ether:%s", to),
			To:      fmt.Sprintf("1356:fabric:%s2", from),
			Payload: ibtppd,
			Index:   index,
			Type:    typ,
		}
	}

	if strings.EqualFold(typ.String(), pb.IBTP_INTERCHAIN.String()) {
		return &pb.IBTP{
			From:    fmt.Sprintf("1356:ether:%s", from),
			To:      fmt.Sprintf("1356:fabric:%s", to),
			Payload: ibtppd,
			Index:   index,
			Type:    typ,
		}
	} else {
		return &pb.IBTP{
			From:    fmt.Sprintf("1356:fabric:%s", to),
			To:      fmt.Sprintf("1356:ether:%s", from),
			Payload: ibtppd,
			Index:   index,
			Type:    typ,
		}
	}

}

func TestUnionMode(t *testing.T) {

}

func TestDirectMode(t *testing.T) {
	fmt.Println(hexutil.Encode([]byte("world")))
}

func TestGenerateConfig(t *testing.T) {
	s0 := []string{"0xdabf70772fbf4a13f3152f66eed09075e2b95f40be1f6fcf3f3a3f6b08069c705ac2ef9dbc5d50a58d5c9cef0d078e0afae8b70e3d0a313b81a5107dcc2ec27f00",
		"0xce589509b78b2121eaabf553d434903d39dc38ac7824f1fb411cc9b7bb068d23593f650b0fda688b9625b82246ac694f636c66136bb0963f24ca16e745950d7c00",
		"0x1a1e1dfbb19cd086a84fc9805076f41c2387560ff6305c6ab6f94f6e1c04d2c15d8cc5f527bd5ebde320dfe6abf6175adfadc6519aec357f24278496810b122d01"}

	var sigs []byte
	for _, s := range s0 {
		sig := hexutil.Decode(s)
		sigs = append(sigs, sig...)
	}

	fmt.Print("[")
	for i, b := range sigs {
		fmt.Printf("\"0x%02x\"", b)
		if i != len(sigs)-1 {
			fmt.Print(",")
		}
	}

	fmt.Println("]")
}
