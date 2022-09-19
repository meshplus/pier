package appchain_adapter

import (
	"fmt"
	"time"

	"github.com/meshplus/bitxhub-model/pb"
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
						"Froms":      req.Froms,
						"indexs":     req.Indexs,
						"serviceIDs": req.ServiceIDs,
						"batchSize":  len(req.Froms),
						"proofs":     req.Proofs,
					}).Info("start submit ibtpBatch")

					now := time.Now()
					response, err := a.client.SubmitIBTPBatch(
						req.Froms, req.Indexs, req.ServiceIDs, req.IbtpTypes, req.Contents, req.Proofs, req.IsEncrypted)
					if err != nil {
						a.logger.WithFields(logrus.Fields{"Froms": req.Froms, "indexs": req.Indexs,
							"serviceIDs": req.ServiceIDs, "proofs": req.Proofs, "err": err}).Errorf("SubmitIBTPBatch err")
						continue
					}

					if !response.Status {
						a.logger.Errorf("fail to send ibtp: %s",
							response.Message)
					}

					a.logger.WithFields(logrus.Fields{
						"time": time.Since(now),
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

//func (a *AppchainAdapter) listenReceiptBatch() {
//	receiptBatch := make([]*pb.IBTP, 0)
//	for {
//		select {
//		case receipt := <-a.recvReceiptC:
//			receiptBatch = append(receiptBatch, receipt)
//
//		default:
//			// if receive receipt, start handle and submit
//			if len(receiptBatch) > 0 {
//				dst := make([]*pb.IBTP, len(receiptBatch))
//				copy(dst, receiptBatch)
//				receipts := a.receiptPool.Get().(*pb.BatchReceipt)
//				var err error
//
//				// control batch size
//				for len(dst) > 0 {
//					if len(dst) <= a.config.Batch.BatchSize {
//						err = a.handleReceiptBatch(dst[:], receipts)
//						dst = make([]*pb.IBTP, 0)
//					} else {
//						err = a.handleReceiptBatch(dst[:9], receipts)
//						if err != nil {
//							a.logger.Errorf("handleReceiptBatch err：%s", err)
//							dst = dst[10:]
//							continue
//						}
//						dst = dst[10:]
//					}
//					a.logger.WithFields(logrus.Fields{
//						"Tos":        receipts.Tos,
//						"indexs":     receipts.Indexs,
//						"serviceIDs": receipts.ServiceIDs,
//						"batchSize":  len(receipts.Tos),
//					}).Info("start submit receiptBatch")
//
//					now := time.Now()
//					responses, err := a.client.SubmitReceiptBatch(
//						receipts.Tos, receipts.Indexs, receipts.ServiceIDs, receipts.IbtpTypes, receipts.Results, receipts.Proofs)
//					if err != nil {
//						a.logger.WithFields(logrus.Fields{
//							"Tos":        receipts.Tos,
//							"indexs":     receipts.Indexs,
//							"serviceIDs": receipts.ServiceIDs,
//							"proofs":     receipts.Proofs,
//							"err":        err,
//						}).Errorf("SubmitReceiptBatch err")
//						continue
//					}
//
//					for i, res := range responses.Response {
//						if !res.Status {
//							a.logger.Errorf("fail to send receipt To:%s with Service:%s, index:%d: \n%s",
//								receipts.Tos[i], receipts.ServiceIDs[i], receipts.Indexs[i], res.Message)
//						}
//					}
//
//					a.logger.WithFields(logrus.Fields{
//						"time": time.Since(now),
//					}).Info("appchain adapter submit receipt batch success")
//				}
//
//				// release instance batch after clean them
//				receiptBatch = make([]*pb.IBTP, 0)
//				receipts.Reset()
//				a.receiptPool.Put(receipts)
//			}
//		}
//	}
//}
//
//func (a *AppchainAdapter) handleReceiptBatch(batch []*pb.IBTP, receipts *pb.BatchReceipt) error {
//
//	result := &pb.Result{}
//	proof := &pb.BxhProof{}
//
//	for _, ibtp := range batch {
//		_, _, serviceID := ibtp.ParseTo()
//		ibtp, pd, err := a.handlePayload(ibtp, false)
//		if err != nil {
//			return err
//		}
//
//		if err := result.Unmarshal(pd.Content); err != nil {
//			return fmt.Errorf("unmarshal result of receipt %s: %w", ibtp.ID(), err)
//		}
//
//		if err := proof.Unmarshal(ibtp.Proof); err != nil {
//			return fmt.Errorf("fail to unmarshal proof of ibtp %s: %w", ibtp.ID(), err)
//		}
//
//		receipts.Tos = append(receipts.Tos, ibtp.From)
//		receipts.Indexs = append(receipts.Indexs, ibtp.Index)
//		receipts.ServiceIDs = append(receipts.ServiceIDs, serviceID)
//		receipts.IbtpTypes = append(receipts.IbtpTypes, ibtp.Type)
//		receipts.Results = append(receipts.Results, result)
//		receipts.Proofs = append(receipts.Proofs, proof)
//	}
//	return nil
//}
