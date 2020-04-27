package bxh_lite

import (
	"fmt"
	"strconv"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/syndtr/goleveldb/leveldb"
)

func (lite *BxhLite) persist(h *pb.BlockHeader) error {
	batch := lite.storage.NewBatch()

	data, err := h.Marshal()
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	batch.Put(headerKey(h.Number), data)
	batch.Put(headerHeightKey(), []byte(strconv.FormatUint(lite.height, 10)))

	err = batch.Commit()
	if err != nil {
		return fmt.Errorf("batch commit: %w", err)
	}

	return nil
}

// getLastHeight gets the current working height of lite
func (lite *BxhLite) getLastHeight() (uint64, error) {
	v, err := lite.storage.Get(headerHeightKey())
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}

		return 0, fmt.Errorf("get header height %w", err)
	}

	return strconv.ParseUint(string(v), 10, 64)
}
