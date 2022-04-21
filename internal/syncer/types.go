package syncer

import "fmt"

const (
	srcchainNotAvailable = "current appchain not available"
	dstchainNotAvailable = "target appchain not available"
	invalidIBTP          = "invalid ibtp"
	ibtpIndexExist       = "index already exists"
	ibtpIndexWrong       = "wrong index"
	noBindRule           = "appchain didn't register rule"
)

var (
	ErrIBTPNotFound  = fmt.Errorf("receipt from bitxhub failed")
	ErrMetaOutOfDate = fmt.Errorf("interchain meta is out of date")
)

const maxChSize = 1 << 10

type SubscriptionKey struct {
	PierID      string `json:"pier_id"`
	AppchainDID string `json:"appchain_did"`
}

func syncHeightKey() []byte {
	return []byte("sync-height")
}
