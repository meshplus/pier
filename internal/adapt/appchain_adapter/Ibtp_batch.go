package appchain_adapter

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

const MaxSize = 10

func (a *AppchainAdapter) listenIBTPBatch() {
	iBTPBatch := make([]*pb.IBTP, 0)
	for {
		select {
		case ibtp := <-a.recvIbtpC:
			iBTPBatch = append(iBTPBatch, ibtp)

		default:
			// if receive ibtp, start handle and submit
			if len(iBTPBatch) > 0 {
				dst := make([]*pb.IBTP, len(iBTPBatch))
				copy(dst, iBTPBatch)
				req := a.requestPool.Get().(*pb.BatchRequest)
				var err error

				// control batch size
				for len(dst) > 0 {
					batchSize := a.config.Batch.BatchSize
					if a.config.Batch.BatchSize > MaxSize {
						batchSize = MaxSize
					}
					if len(dst) <= batchSize {
						err = a.handleIBTPBatch(dst[:], req)
						dst = make([]*pb.IBTP, 0)
					} else {
						err = a.handleIBTPBatch(dst[:batchSize], req)
						if err != nil {
							a.logger.Errorf("handleIBTPBatch err：%s", err)
							dst = dst[batchSize:]
							continue
						}
						dst = dst[batchSize:]
					}
					a.logger.WithFields(logrus.Fields{
						//"Froms":      req.Froms,
						"indexs":     req.Indexs,
						"serviceIDs": req.ServiceIDs,
						"ibtpTypes":  req.IbtpTypes,
						//"Contents":   req.Contents,
						//"proofs":     req.Proofs,
						//"isEncrypt":  req.IsEncrypted,
						"batchSize": len(req.Froms),
					}).Info("start submit ibtpBatch")

					now := time.Now()
					_, err = a.client.SubmitIBTPBatch(
						req.Froms, req.Indexs, req.ServiceIDs, req.IbtpTypes, req.Contents, req.Proofs, req.IsEncrypted)
					if err != nil {
						a.logger.WithFields(logrus.Fields{"Froms": req.Froms, "indexs": req.Indexs,
							"serviceIDs": req.ServiceIDs, "proofs": req.Proofs, "err": err}).Errorf("SubmitIBTPBatch err")
						continue
					}

					a.logger.WithFields(logrus.Fields{
						"batchSize": len(req.Froms),
						"indexs":    req.Indexs,
						"ibtpTypes": req.IbtpTypes,
						"time":      time.Since(now),
					}).Info("appchain adapter submit ibtp batch success")

					// release instance after clean it
					req.Reset()
					a.requestPool.Put(req)
				}

				// clean batch
				iBTPBatch = make([]*pb.IBTP, 0)
			}
		}
	}
}

func (a *AppchainAdapter) handleIBTPBatch(batch []*pb.IBTP, req *pb.BatchRequest) error {
	for _, ibtp := range batch {
		content := &pb.Content{}
		proof := &pb.BxhProof{}
		_, _, serviceID := ibtp.ParseTo()
		ibtp, pd, err := a.handlePayload(ibtp, false)
		if err != nil {
			return err
		}

		if err := content.Unmarshal(pd.Content); err != nil {
			return fmt.Errorf("unmarshal content of ibtp %s: %w", ibtp.ID(), err)
		}

		if err := proof.Unmarshal(ibtp.Proof); err != nil {
			return fmt.Errorf("fail to unmarshal proof of ibtp %s: %w", ibtp.ID(), err)
		}

		req.Froms = append(req.Froms, ibtp.From)
		req.Indexs = append(req.Indexs, ibtp.Index)
		req.ServiceIDs = append(req.ServiceIDs, serviceID)
		req.IbtpTypes = append(req.IbtpTypes, ibtp.Type)
		req.Contents = append(req.Contents, content)
		req.Proofs = append(req.Proofs, proof)
		// to appchain, encrypt is false, bxh adapter handle encrypt and decrypt payload
		req.IsEncrypted = append(req.IsEncrypted, false)
	}
	return nil
}

func (a *AppchainAdapter) listenReceiptBatch() {
	receiptBatch := make([]*pb.IBTP, 0)
	batchSize := a.config.Batch.BatchSize
	// control batch size
	if batchSize > MaxSize {
		batchSize = MaxSize
	}

	for {
		select {
		case receipt := <-a.recvReceiptC:
			receiptBatch = append(receiptBatch, receipt)

		default:
			// if receive receipt, start handle and submit
			if len(receiptBatch) > 0 {
				dst := make([]*pb.IBTP, len(receiptBatch))
				copy(dst, receiptBatch)
				receipts := a.receiptPool.Get().(*pb.BatchReceipt)
				var err error
				for len(dst) > 0 {
					if len(dst) <= batchSize {
						err = a.handleReceiptBatch(dst[:], receipts)
						dst = make([]*pb.IBTP, 0)
					} else {
						err = a.handleReceiptBatch(dst[:batchSize], receipts)
						if err != nil {
							a.logger.Errorf("handleReceiptBatch err：%s", err)
							dst = dst[batchSize:]
							continue
						}
						dst = dst[batchSize:]
					}

					a.logger.WithFields(logrus.Fields{
						//"Tos":        receipts.Tos,
						"indexs":     receipts.Indexs,
						"serviceIDs": receipts.ServiceIDs,
						"type":       receipts.IbtpTypes,
						"batchSize":  len(receipts.Tos),
						"maxSize":    batchSize,
						"results":    receipts.Results,
					}).Info("start submit receiptBatch")

					now := time.Now()
					_, err := a.client.SubmitReceiptBatch(
						receipts.Tos, receipts.Indexs, receipts.ServiceIDs, receipts.IbtpTypes, receipts.Results, receipts.Proofs)
					if err != nil {
						a.logger.WithFields(logrus.Fields{
							"indexs":     receipts.Indexs,
							"serviceIDs": receipts.ServiceIDs,
							"proofs":     receipts.Proofs,
							"err":        err,
						}).Errorf("SubmitReceiptBatch err")
						continue
					}

					a.logger.WithFields(logrus.Fields{
						"indexs":     receipts.Indexs,
						"serviceIDs": receipts.ServiceIDs,
						"type":       receipts.IbtpTypes,
						"batchSize":  len(receipts.Tos),
						"time":       time.Since(now),
					}).Info("appchain adapter submit receipt batch success")

					// release instance batch after clean them
					receipts.Reset()
					a.receiptPool.Put(receipts)
				}
				// clean batch
				receiptBatch = make([]*pb.IBTP, 0)
			}
		}
	}
}

func (a *AppchainAdapter) handleReceiptBatch(batch []*pb.IBTP, receipts *pb.BatchReceipt) error {
	for _, ibtp := range batch {
		result := &pb.Result{}
		proof := &pb.BxhProof{}
		_, _, FromServiceID := ibtp.ParseFrom()
		ibtp, pd, err := a.handlePayload(ibtp, false)
		if err != nil {
			return err
		}
		// in direct mode, if src pier notify src rollback, will modify ibtp type from INTERCHAIN to RECEIPT_ROLLBACK
		// so the content should be interchain type
		if a.mode == repo.DirectMode && ibtp.Type == pb.IBTP_RECEIPT_ROLLBACK {
			content := &pb.Content{}
			if err := content.Unmarshal(pd.Content); err != nil {
				return fmt.Errorf("unmarshal content of ibtp %s: %w", ibtp.ID(), err)
			}
			//Judging whether it is MultiTransfer(1) or Transfer(0)
			if binary.BigEndian.Uint64(content.Args[0]) == 0 {
				result.MultiStatus = append(result.MultiStatus, false)
			}
		} else {
			// if type is interchain( maybe status is begin_rollback or begin_fail),the content should be interchain type
			if ibtp.Type == pb.IBTP_INTERCHAIN {
				content := &pb.Content{}
				if err := content.Unmarshal(pd.Content); err != nil {
					return fmt.Errorf("unmarshal content of ibtp %s: %w", ibtp.ID(), err)
				}
				a.logger.WithFields(logrus.Fields{"type": ibtp.Type, "status": proof.TxStatus}).Info("src chain need rollback interchain")
				//Judging whether it is MultiTransfer(1) or Transfer(0)
				if binary.BigEndian.Uint64(content.Args[0]) == 0 {
					result.MultiStatus = append(result.MultiStatus, false)
				}
			} else {
				if err := result.Unmarshal(pd.Content); err != nil {
					return fmt.Errorf("unmarshal result of ibtp %s: %w", ibtp.ID(), err)
				}
			}
		}

		if err := proof.Unmarshal(ibtp.Proof); err != nil {
			return fmt.Errorf("fail to unmarshal proof of ibtp %s: %w", ibtp.ID(), err)
		}

		receipts.Tos = append(receipts.Tos, ibtp.To)
		receipts.Indexs = append(receipts.Indexs, ibtp.Index)
		receipts.ServiceIDs = append(receipts.ServiceIDs, FromServiceID)
		receipts.IbtpTypes = append(receipts.IbtpTypes, ibtp.Type)
		receipts.Results = append(receipts.Results, result)
		receipts.Proofs = append(receipts.Proofs, proof)
	}
	return nil
}
