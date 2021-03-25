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

package service

import (
	"github.com/limechain/hedera-eth-bridge-validator/app/clients/hedera/mirror-node"
	"github.com/limechain/hedera-eth-bridge-validator/app/encoding"
	"github.com/limechain/hedera-eth-bridge-validator/app/encoding/memo"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/transfer"
)

// Transfers is the major service used for processing Transfers operations
type Transfers interface {
	// SanityCheckTransfer performs any validation required prior to handling the transaction
	// (memo, state proof verification)
	SanityCheckTransfer(tx mirror_node.Transaction) (*memo.Memo, error)
	// SaveRecoveredTxn creates new Transaction record persisting the recovered Transfer TXn
	SaveRecoveredTxn(txId, amount, sourceAsset, targetAsset string, m memo.Memo) error
	// InitiateNewTransfer Stores the incoming transfer message into the Database
	// aware of already processed transfers
	InitiateNewTransfer(tm encoding.TransferMessage) (*transfer.Transfer, error)
	// VerifyFee verifies that the provided TX reimbursement fee is enough. Returns error if TX processing must be stopped
	// If no error is returned the TX can be processed
	VerifyFee(tm encoding.TransferMessage) error
	// ProcessTransfer processes the transfer message by signing the required
	// authorisation signature submitting it into the required HCS Topic
	ProcessTransfer(tm encoding.TransferMessage) error
}
