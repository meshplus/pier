package bxhLite

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/syndtr/goleveldb/leveldb"
)

// getLastHeight gets the current working height of Syncer
func (lite *BxhLite) getLastHeight() (uint64, error) {
	v, err := lite.storage.Get(syncHeightKey())
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}

		return 0, fmt.Errorf("get bxhLite height %w", err)
	}

	return strconv.ParseUint(string(v), 10, 64)
}

func syncHeightKey() []byte {
	return []byte("sync-height")
}

func (lite *BxhLite) getDemandHeight() uint64 {
	return atomic.LoadUint64(&lite.height) + 1
}

// updateHeight updates sync height and
func (lite *BxhLite) updateHeight() {
	atomic.AddUint64(&lite.height, 1)
}
