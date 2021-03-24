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

package main

import (
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/repository"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/message"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/transaction"
	"github.com/limechain/hedera-eth-bridge-validator/app/process"

	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/status"
	"github.com/limechain/hedera-eth-bridge-validator/config"
)

// Repositories struct holding the referenced repositories
type Repositories struct {
	transferStatus repository.Status
	messageStatus  repository.Status
	transaction    repository.Transaction
	message        repository.Message
}

// PrepareRepositories initialises connection to the Database and instantiates the repositories
func PrepareRepositories(config config.Db) *Repositories {
	db := persistence.RunDb(config)
	return &Repositories{
		transferStatus: status.NewRepositoryForStatus(db, process.CryptoTransferMessageType),
		messageStatus:  status.NewRepositoryForStatus(db, process.HCSMessageType),
		transaction:    transaction.NewRepository(db),
		message:        message.NewRepository(db),
	}
}
