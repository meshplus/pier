package checker

import (
	"encoding/json"
	"github.com/meshplus/bitxhub-core/governance"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-core/validator"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-kit/storage/leveldb"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/peermgr/mock_peermgr"
	"github.com/meshplus/pier/internal/rulemgr"
	"github.com/stretchr/testify/require"
)

const (
	from           = "0xe02d8fdacd59020d7f292ab3278d13674f5c404d"
	to             = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50161"
	from2          = "0x0915fdfc96232c95fb9c62d27cc9dc0f13f50162"
	rulePrefix     = "validation-rule-"
	proofPath      = "./testdata/proof_1.0.0_rc"
	proofPath2     = "./testdata/proof_1.0.0_rc_complex"
	validatorsPath = "./testdata/single_validator"
)

func TestMockChecker_Check(t *testing.T) {
	checker := &MockChecker{}
	require.Nil(t, checker.Check(nil))
}

func TestDirectChecker_Check(t *testing.T) {
	dc := New(t).(*DirectChecker)

	ibtp1 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, from, to, proofPath)
	ibtp2 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, "2", to, proofPath)
	ibtp3 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, "10", to, proofPath)
	ibtp4 := getIBTP(t, uint64(1), pb.IBTP_RECEIPT_SUCCESS, from, to, proofPath)
	ibtp5 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, from2, to, proofPath)
	ibtp6 := getIBTP(t, uint64(1), pb.IBTP_INTERCHAIN, from, to, proofPath2)

	// check with load not ok
	// not nil code
	err := dc.Check(ibtp1)
	require.NotNil(t, err)
	// nonexistent appchain
	err = dc.Check(ibtp2)
	require.NotNil(t, err)
	// appchain unmarshal error
	err = dc.Check(ibtp3)
	require.NotNil(t, err)
	// ethereum with nil code
	err = dc.Check(ibtp4)
	require.NotNil(t, err)
	// fabric with nil code
	err = dc.Check(ibtp5)
	require.Nil(t, err)

	// check with load ok
	app, err := getAppchain(from, "fabric")
	require.Nil(t, err)
	dc.appchainCache.Store(from, &appchainRule{
		appchain:    app,
		codeAddress: validator.SimFabricRuleAddr,
	})
	// check successfully
	err = dc.Check(ibtp1)
	require.Nil(t, err)
	// check unsuccessfully
	err = dc.Check(ibtp6)
	require.NotNil(t, err)
}

func New(t *testing.T) Checker {
	mockCtl := gomock.NewController(t)
	mockCtl.Finish()
	tmpDir, err := ioutil.TempDir("", "storage")
	require.Nil(t, err)
	storage, err := leveldb.New(tmpDir)
	require.Nil(t, err)
	storage.Put([]byte(rulePrefix+types.NewAddressByStr(from).String()), []byte("from"))

	pm := mock_peermgr.NewMockPeerManager(mockCtl)
	pm.EXPECT().RegisterMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	pm.EXPECT().RegisterMultiMsgHandler(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	rm, err := rulemgr.New(storage, pm, log.NewWithModule("api"))
	require.Nil(t, err)

	am, err := appchain.NewManager(from, storage, pm, log.NewWithModule("api"))
	require.Nil(t, err)
	am.Mgr = &MockAppchainMgr{}

	dc := NewDirectChecker(rm, am)
	return dc
}

func getAppchain(id, chainType string) (*appchainmgr.Appchain, error) {
	validators, err := ioutil.ReadFile(validatorsPath)
	if err != nil {
		return nil, err
	}

	app := &appchainmgr.Appchain{
		ID:            id,
		Name:          "chainA",
		Validators:    string(validators),
		ConsensusType: "rbft",
		ChainType:     chainType,
		Desc:          "appchain",
		Version:       "1.4.3",
		PublicKey:     "",
	}

	return app, nil
}

func getIBTP(t *testing.T, index uint64, typ pb.IBTP_Type, fid, tid, proofPath string) *pb.IBTP {
	ct := &pb.Content{
		Func:     "interchainCharge",
		Args:     [][]byte{[]byte("Alice"), []byte("Alice"), []byte("1")},
		Callback: "interchainConfirm",
	}
	c, err := ct.Marshal()
	require.Nil(t, err)

	pd := pb.Payload{
		Encrypted: false,
		Content:   c,
	}
	ibtppd, err := pd.Marshal()
	require.Nil(t, err)

	proof, err := ioutil.ReadFile(proofPath)
	require.Nil(t, err)

	return &pb.IBTP{
		From:    fid,
		To:      tid,
		Payload: ibtppd,
		Index:   index,
		Type:    typ,
		Proof:   proof,
	}
}

// MockAppchainMgr================================================
type MockAppchainMgr struct {
}

func (m MockAppchainMgr) ChangeStatus(id, trigger, lastStatus string, extra []byte) (bool, []byte) {
	panic("implement me")
}

func (m MockAppchainMgr) GovernancePre(id string, event governance.EventType, extra []byte) (bool, []byte) {
	panic("implement me")
}

func (m MockAppchainMgr) CountAvailable(extra []byte) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) CountAll(extra []byte) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) All(extra []byte) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) QueryById(id string, extra []byte) (bool, []byte) {
	if id == from || id == from2 {
		app, err := getAppchain(id, "fabric")
		if err != nil {
			return false, nil
		}
		data, err := json.Marshal(app)
		if err != nil {
			return false, nil
		}
		return true, data
	} else if id == to {
		app, err := getAppchain(id, "ethereum")
		data, err := json.Marshal(app)
		if err != nil {
			return false, nil
		}
		return true, data
	} else if id == "10" {
		return true, []byte("10")
	} else {
		return false, nil
	}
}

func (m MockAppchainMgr) Register(info []byte) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) Update(info []byte) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) CountAvailableAppchains() (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) UpdateAppchain(id, appchainOwner, docAddr, docHash, validators string, consensusType, chainType, name, desc, version, pubkey string) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) Audit(proposer string, isApproved int32, desc string) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) FetchAuditRecords(id string) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) CountApprovedAppchains() (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) CountAppchains() (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) Appchains() (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) DeleteAppchain(id string) (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) Appchain() (bool, []byte) {
	return true, nil
}

func (m MockAppchainMgr) GetPubKeyByChainID(id string) (bool, []byte) {
	return true, nil
}

var _ appchainmgr.AppchainMgr = &MockAppchainMgr{}
