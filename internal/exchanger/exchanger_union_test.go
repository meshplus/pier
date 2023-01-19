package exchanger

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/adapt/mock_adapt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

//const (
//	chain0          = "chain0"
//	chain1          = "chain1"
//	from            = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
//	to              = "0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998"
//	pierID          = "0x892eedc032948be00722038a29f2a90d0e05352f"
//	fullFromService = "1356:fabric:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998"
//	fullToService   = "1356:ether:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b997"
//)

func TestUnionMode(t *testing.T) {
	testNormalStartUnion(t)
}

func prepareUnion(t *testing.T) (
	*mock_adapt.MockAdapt, *mock_adapt.MockAdapt, *Exchanger,
) {
	mode := repo.UnionMode
	mockCtl := gomock.NewController(t)
	mockAdaptRelay := mock_adapt.NewMockAdapt(mockCtl)
	mockAdaptUnion := mock_adapt.NewMockAdapt(mockCtl)
	mockExchanger, err := New(mode, "", "1356", WithSrcAdapt(mockAdaptRelay), WithDestAdapt(mockAdaptUnion), WithLogger(log.NewWithModule("exchanger")))
	require.Nil(t, err)
	return mockAdaptRelay, mockAdaptUnion, mockExchanger
}

func testNormalStartUnion(t *testing.T) {
	mockAdaptRelay, mockAdaptUnion, mockExchanger := prepareUnion(t)

	srcIBTPCh := make(chan *pb.IBTP)
	destIBTPCh := make(chan *pb.IBTP)
	mockAdaptRelay.EXPECT().MonitorIBTP().Return(destIBTPCh).AnyTimes()
	mockAdaptUnion.EXPECT().MonitorIBTP().Return(srcIBTPCh).AnyTimes()
	mockAdaptRelay.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptRelay.EXPECT().Name().Return("bitxhub:1356").AnyTimes()
	mockAdaptUnion.EXPECT().Start().Return(nil).AnyTimes()
	mockAdaptUnion.EXPECT().Stop().Return(nil).AnyTimes()
	mockAdaptUnion.EXPECT().Name().Return("fabric:0x3f9d18f7c3a6e5e4c0b877fe3e688ab08840b998").AnyTimes()

	mockAdaptRelay.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(&pb.IBTP{}, nil).AnyTimes()
	mockAdaptUnion.EXPECT().QueryIBTP(gomock.Any(), gomock.Any()).Return(&pb.IBTP{}, nil).AnyTimes()

	mockAdaptRelay.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()
	mockAdaptUnion.EXPECT().SendIBTP(gomock.Any()).Return(nil).AnyTimes()

	mockAdaptRelay.EXPECT().GetLocalServiceIDList().Return([]string{fullFromService}, nil).AnyTimes()
	mockAdaptUnion.EXPECT().QueryInterchain(gomock.Eq(fullFromService)).
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
	mockAdaptRelay.EXPECT().QueryInterchain(gomock.Eq(fullToService)).
		Return(&pb.Interchain{ID: fullToService,
			SourceInterchainCounter: map[string]uint64{fullFromService: 1},
			ReceiptCounter:          map[string]uint64{fullFromService: 1},
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

	testUnionIBTPFromDest(t, destIBTPCh, ibtps_dest, mockExchanger)
	testUnionIBTPFromSrc(t, srcIBTPCh, ibtps_src, mockExchanger)

	// test for asset exchange ibtp
	time.Sleep(500 * time.Microsecond)
	close(destIBTPCh)
	close(srcIBTPCh)
	require.Nil(t, mockExchanger.Stop())
}

func testUnionIBTPFromSrc(t *testing.T, outCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			outCh <- ibtp
		}
	}
	time.Sleep(100 * time.Millisecond)
	//require.Equal(t, uint64(100), exchanger.srcServiceMeta[ibtps[0].From].InterchainCounter[ibtps[0].To])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS, false)
	for _, receipt := range ibtpReceipts {
		outCh <- receipt
	}

	time.Sleep(100 * time.Millisecond)
	//require.Equal(t, uint64(100), exchanger.srcServiceMeta[ibtpReceipts[0].To].SourceReceiptCounter[ibtpReceipts[0].From])
}

func testUnionIBTPFromDest(t *testing.T, inCh chan *pb.IBTP, ibtps []*pb.IBTP, exchanger *Exchanger) {
	for i, ibtp := range ibtps {
		if (i+1)%2 == 0 {
			inCh <- ibtp
		}
	}

	time.Sleep(100 * time.Millisecond)
	//require.Equal(t, uint64(100), exchanger.destServiceMeta[ibtps[0].To].SourceInterchainCounter[ibtps[0].From])

	ibtpReceipts, _ := genIBTPs(t, 100, pb.IBTP_RECEIPT_SUCCESS, true)
	for _, receipt := range ibtpReceipts {
		inCh <- receipt
	}

	time.Sleep(100 * time.Millisecond)
	//require.Equal(t, uint64(100), exchanger.destServiceMeta[ibtpReceipts[0].From].ReceiptCounter[ibtpReceipts[0].To])
}
