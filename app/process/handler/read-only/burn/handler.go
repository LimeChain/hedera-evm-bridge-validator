/*
 * Copyright 2021 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package burn

import (
	"database/sql"
	"github.com/hashgraph/hedera-sdk-go/v2"
	mirror_node "github.com/limechain/hedera-eth-bridge-validator/app/clients/hedera/mirror-node"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/client"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/repository"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/service"
	model "github.com/limechain/hedera-eth-bridge-validator/app/model/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/fee"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/schedule"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	log "github.com/sirupsen/logrus"
	"strconv"
)

type Handler struct {
	bridgeAccount      hedera.AccountID
	transfersService   service.Transfers
	scheduleRepository repository.Schedule
	transferRepository repository.Transfer
	mirrorNode         client.MirrorNode
	logger             *log.Entry
}

func NewHandler(
	bridgeAccount string,
	mirrorNode client.MirrorNode,
	scheduleRepository repository.Schedule,
	transferRepository repository.Transfer,
	transferService service.Transfers) *Handler {
	bridgeAcc, err := hedera.AccountIDFromString(bridgeAccount)
	if err != nil {
		log.Fatalf("Invalid account id [%s]. Error: [%s]", bridgeAccount, err)
	}
	return &Handler{
		bridgeAccount:      bridgeAcc,
		mirrorNode:         mirrorNode,
		transfersService:   transferService,
		scheduleRepository: scheduleRepository,
		transferRepository: transferRepository,
		logger:             config.GetLoggerFor("Hedera Burn and Topic Message Read-only Handler"),
	}
}

func (mhh Handler) Handle(payload interface{}) {
	transferMsg, ok := payload.(*model.Transfer)
	if !ok {
		mhh.logger.Errorf("Could not cast payload [%s]", payload)
		return
	}

	transactionRecord, err := mhh.transfersService.InitiateNewTransfer(*transferMsg)
	if err != nil {
		mhh.logger.Errorf("[%s] - Error occurred while initiating processing. Error: [%s]", transferMsg.TransactionId, err)
		return
	}

	if transactionRecord.Status != transfer.StatusInitial {
		mhh.logger.Debugf("[%s] - Previously added with status [%s]. Skipping further execution.", transactionRecord.TransactionID, transactionRecord.Status)
		return
	}

	amount, err := strconv.ParseInt(transferMsg.Amount, 10, 64)
	if err != nil {
		mhh.logger.Errorf("[%s] - Failed to parse string amount. Error [%s]", transferMsg.TransactionId, err)
		return
	}

	burnTransfer := []mirror_node.Transfer{
		{
			Account: mhh.bridgeAccount.String(),
			Amount:  -amount,
			Token:   transferMsg.SourceAsset,
		},
	}
	for {
		response, err := mhh.mirrorNode.GetAccountTokenBurnTransactionsAfterTimestampString(mhh.bridgeAccount, transferMsg.Timestamp)
		if err != nil {
			mhh.logger.Errorf("[%s] - Failed to get token burn transactions after timestamp. Error: [%s]", transactionRecord.TransactionID, err)
		}

		finished := false
		for _, transaction := range response.Transactions {
			found := false
			for _, expectedTransfer := range burnTransfer {
				contained := false
				for _, t := range transaction.TokenTransfers {
					if expectedTransfer == t {
						contained = true
						break
					}
				}

				if !contained {
					found = false
					break
				} else {
					found = true
				}
			}
			if found {
				isFound := false
				scheduledTx, err := mhh.mirrorNode.GetScheduledTransaction(transaction.TransactionID)
				if err != nil {
					mhh.logger.Errorf("[%s] - Failed to retrieve scheduled transaction [%s]. Error: [%s]", transferMsg.TransactionId, transaction.TransactionID, err)
					continue
				}
				for _, tx := range scheduledTx.Transactions {
					if tx.Result == hedera.StatusSuccess.String() {
						scheduleID, err := mhh.mirrorNode.GetSchedule(tx.EntityId)
						if err != nil {
							mhh.logger.Errorf("[%s] - Failed to get scheduled entity [%s]. Error: [%s]", transferMsg.TransactionId, scheduleID, err)
							break
						}
						if scheduleID.Memo == transferMsg.TransactionId {
							err := mhh.scheduleRepository.Create(&entity.Schedule{
								TransactionID: transaction.TransactionID,
								ScheduleID:    tx.EntityId,
								Operation:     schedule.BURN,
								Status:        fee.StatusCompleted, // TODO: not fee
								TransferID: sql.NullString{
									String: transferMsg.TransactionId,
									Valid:  true,
								},
							})
							if err != nil {
								mhh.logger.Errorf("[%s] - Failed to save scheduled entity [%s]. Error: [%s]", transferMsg.TransactionId, tx.EntityId, err)
								break
							}

							err = mhh.transferRepository.UpdateStatusCompleted(transferMsg.TransactionId)
							if err != nil {
								mhh.logger.Errorf("[%s] - Failed to update completed. Error: [%s]", transferMsg.TransactionId, err)
							}
							isFound = true
						}
					}
				}
				if isFound {
					finished = true
					break
				}
			}
		}
		if finished {
			break
		}
	}
}