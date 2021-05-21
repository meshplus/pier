package syncer

import (
	"fmt"
	"strconv"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/pkg/model"
)

func (syncer *WrapperSyncer) persist(ws *pb.InterchainTxWrappers) error {
	if len(ws.InterchainTxWrappers) == 0 {
		return fmt.Errorf("empty interchain wrappers")
	}
	batch := syncer.storage.NewBatch()

	data, err := ws.Marshal()
	if err != nil {
		return fmt.Errorf("marshal wrapper: %w", err)
	}

	batch.Put(model.WrapperKey(ws.InterchainTxWrappers[0].Height), data)

	for _, w := range ws.InterchainTxWrappers {
		for _, tx := range w.Transactions {
			data, err := tx.Marshal()
			if err != nil {
				return fmt.Errorf("verifiedTx marshal: %w", err)
			}

			batch.Put(model.IBTPKey(tx.GetTx().GetIBTP().ID()), data)
		}
	}
	batch.Put(syncHeightKey(), []byte(strconv.FormatUint(syncer.height, 10)))

	batch.Commit()

	return nil
}
