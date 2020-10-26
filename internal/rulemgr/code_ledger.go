package rulemgr

import (
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxhub-kit/types"
)

const rulePrefix = "validation-rule-"

type CodeLedger struct {
	storage storage.Storage
}

func (l *CodeLedger) GetCode(address types.Address) []byte {
	key := rulePrefix + address.String()
	code := l.storage.Get([]byte(key))
	return code
}

func (l *CodeLedger) SetCode(address *types.Address, code []byte) error {
	key := rulePrefix + address.String()
	l.storage.Put([]byte(key), code)
	return nil
}
