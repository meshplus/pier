package bxh_lite

import (
	"fmt"
	"strconv"

	"github.com/meshplus/bitxhub-model/pb"
)

func (lite *BxhLite) persist(h *pb.BlockHeader) error {
	batch := lite.storage.NewBatch()

	data, err := h.Marshal()
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	batch.Put(headerKey(h.Number), data)
	batch.Put(headerHeightKey(), []byte(strconv.FormatUint(lite.height, 10)))

	batch.Commit()

	return nil
}

// getLastHeight gets the current working height of lite
func (lite *BxhLite) getLastHeight() (uint64, error) {
	v := lite.storage.Get(headerHeightKey())
	if v == nil {
		// if header height is not set, return default 0
		return 0, nil
	}

	return strconv.ParseUint(string(v), 10, 64)
}
