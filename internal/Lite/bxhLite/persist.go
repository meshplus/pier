package bxhLite

import (
	"fmt"
	"strconv"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
)

func (lite *BxhLite) persist(w *pb.MerkleWrapper) error {
	batch := lite.storage.NewBatch()

	data, err := w.Marshal()
	if err != nil {
		return fmt.Errorf("marshal wrapper: %w", err)
	}

	batch.Put(model.WrapperKey(w.BlockHeader.Number), data)

	for _, tx := range w.Transactions {
		ibtp, err := tx.GetIBTP()
		if err != nil {
			return fmt.Errorf("get ibtp from tx: %w", err)
		}

		data, err := ibtp.Marshal()
		if err != nil {
			return fmt.Errorf("ibtp marshal: %w", err)
		}

		batch.Put(model.IBTPKey(ibtp.ID()), data)
	}

	batch.Put(syncHeightKey(), []byte(strconv.FormatUint(lite.height, 10)))

	err = batch.Commit()
	if err != nil {
		return fmt.Errorf("batch commit: %w", err)
	}

	return nil
}
