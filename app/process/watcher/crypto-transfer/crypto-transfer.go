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

package cryptotransfer

import (
	"errors"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go"
	"github.com/limechain/hedera-eth-bridge-validator/app/clients/hedera/mirror-node"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/clients"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/repositories"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/services"
	"github.com/limechain/hedera-eth-bridge-validator/app/encoding"
	"github.com/limechain/hedera-eth-bridge-validator/app/helper/timestamp"
	"github.com/limechain/hedera-eth-bridge-validator/app/process"
	"github.com/limechain/hedera-eth-bridge-validator/app/process/watcher/publisher"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	"github.com/limechain/hedera-watcher-sdk/queue"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"time"
)

type Watcher struct {
	bridgeService    services.Bridge
	client           clients.MirrorNode
	accountID        hedera.AccountID
	typeMessage      string
	pollingInterval  time.Duration
	statusRepository repositories.Status
	maxRetries       int
	startTimestamp   int64
	started          bool
	logger           *log.Entry
}

func NewWatcher(
	bridgeService services.Bridge,
	client clients.MirrorNode,
	accountID hedera.AccountID,
	pollingInterval time.Duration,
	repository repositories.Status,
	maxRetries int,
	startTimestamp int64,
) *Watcher {
	return &Watcher{
		bridgeService:    bridgeService,
		client:           client,
		accountID:        accountID,
		typeMessage:      process.CryptoTransferMessageType,
		pollingInterval:  pollingInterval,
		statusRepository: repository,
		maxRetries:       maxRetries,
		startTimestamp:   startTimestamp,
		started:          false,
		logger:           config.GetLoggerFor(fmt.Sprintf("[%s] Transfer Watcher", accountID.String())),
	}
}

func (ctw Watcher) Watch(q *queue.Queue) {
	accountAddress := ctw.accountID.String()
	_, err := ctw.statusRepository.GetLastFetchedTimestamp(accountAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctw.logger.Debugf("[%s] No Transfer Watcher Timestamp found in DB", accountAddress)
			err := ctw.statusRepository.CreateTimestamp(accountAddress, ctw.startTimestamp)
			if err != nil {
				ctw.logger.Fatalf("[%s] Failed to create Transfer Watcher Status timestamp. Error %s", accountAddress, err)
			}
		} else {
			ctw.logger.Fatalf("Failed to fetch last Transfer Watcher timestamp. Err: %s", err)
		}
	}

	go ctw.beginWatching(q)
}

func (ctw Watcher) beginWatching(q *queue.Queue) {
	if !ctw.client.AccountExists(ctw.accountID) {
		ctw.logger.Errorf("Error incoming: Could not start monitoring account - Account not found.")
		return
	}

	ctw.logger.Debugf("Starting Transfer Watcher for Account [%s] after Timestamp [%d]", ctw.accountID, ctw.startTimestamp)
	milestoneTimestamp := ctw.startTimestamp
	for {
		transactions, e := ctw.client.GetAccountCreditTransactionsAfterTimestamp(ctw.accountID, milestoneTimestamp)
		if e != nil {
			ctw.logger.Errorf("Error incoming: Suddenly stopped monitoring account - [%s]", e)
			ctw.restart(q)
			return
		}

		ctw.logger.Debugf("Found [%d] TX for AccountID [%s]", len(transactions.Transactions), ctw.accountID)
		if len(transactions.Transactions) > 0 {
			for _, tx := range transactions.Transactions {
				go ctw.processTransaction(tx, q)
			}
			var err error
			milestoneTimestamp, err = timestamp.FromString(transactions.Transactions[len(transactions.Transactions)-1].ConsensusTimestamp)
			if err != nil {
				ctw.logger.Errorf("Watcher [%s] - Unable to parse latest transaction timestamp. Error - [%s].", ctw.accountID.String(), err)
				continue
			}
		}

		err := ctw.statusRepository.UpdateLastFetchedTimestamp(ctw.accountID.String(), milestoneTimestamp)
		if err != nil {
			ctw.logger.Errorf("Error incoming: Failed to update last fetched timestamp - [%s]", e)
			return
		}
		time.Sleep(ctw.pollingInterval * time.Second)
	}
}

func (ctw Watcher) processTransaction(tx mirror_node.Transaction, q *queue.Queue) {
	ctw.logger.Infof("New Transaction with ID: [%s]", tx.TransactionID)
	amount, err := tx.GetIncomingAmountFor(ctw.accountID.String())
	if err != nil {
		ctw.logger.Errorf("Could not extract incoming amount for TX [%s]. Error: [%s]", tx.TransactionID, err)
		return
	}

	m, err := ctw.bridgeService.SanityCheck(tx)
	if err != nil {
		ctw.logger.Errorf("Sanity check for TX [%s] failed. Error: [%s]", tx.TransactionID, err)
		return
	}

	transferMessage := encoding.NewTransferMessage(tx.TransactionID, m.EthereumAddress, amount, m.TxReimbursementFee, m.GasPriceGwei)
	publisher.Publish(transferMessage, ctw.typeMessage, ctw.accountID, q)
}

func (ctw Watcher) restart(q *queue.Queue) {
	if ctw.maxRetries > 0 {
		ctw.maxRetries--
		ctw.logger.Infof("Watcher is trying to reconnect")
		go ctw.Watch(q)
		return
	}
	ctw.logger.Errorf("Watcher failed: [Too many retries]")
}
