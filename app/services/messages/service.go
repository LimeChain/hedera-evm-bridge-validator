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

package messages

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/client"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/repository"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/service"
	ethhelper "github.com/limechain/hedera-eth-bridge-validator/app/helper/ethereum"
	"github.com/limechain/hedera-eth-bridge-validator/app/model/auth-message"
	"github.com/limechain/hedera-eth-bridge-validator/app/model/message"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type Service struct {
	ethSigner          service.Signer
	contractsService   service.Contracts
	transferRepository repository.Transfer
	messageRepository  repository.Message
	topicID            hedera.TopicID
	hederaClient       client.HederaNode
	mirrorClient       client.MirrorNode
	ethClient          client.Ethereum
	logger             *log.Entry
}

func NewService(
	ethSigner service.Signer,
	contractsService service.Contracts,
	transferRepository repository.Transfer,
	messageRepository repository.Message,
	hederaClient client.HederaNode,
	mirrorClient client.MirrorNode,
	ethClient client.Ethereum,
	topicID string,
) *Service {
	tID, e := hedera.TopicIDFromString(topicID)
	if e != nil {
		panic(fmt.Sprintf("Invalid monitoring Topic ID [%s] - Error: [%s]", topicID, e))
	}

	return &Service{
		ethSigner:          ethSigner,
		contractsService:   contractsService,
		messageRepository:  messageRepository,
		transferRepository: transferRepository,
		logger:             config.GetLoggerFor(fmt.Sprintf("Messages Service")),
		topicID:            tID,
		hederaClient:       hederaClient,
		mirrorClient:       mirrorClient,
		ethClient:          ethClient,
	}
}

// SanityCheckSignature performs validation on the topic message metadata.
// Validates it against the Transaction Record metadata from DB
func (ss *Service) SanityCheckSignature(tm message.Message) (bool, error) {
	topicMessage := tm.GetTopicSignatureMessage()

	// In case a topic message for given transfer is being processed before the actual transfer
	t, err := ss.awaitTransfer(topicMessage.TransferID)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to await incoming transfer. Error: [%s]", topicMessage.TransferID, err)
		return false, err
	}

	wrappedToken, err := ss.contractsService.ParseToken(t.NativeToken)
	if err != nil {
		ss.logger.Errorf("[%s] - Could not parse nativeToken [%s] - Error: [%s]", t.TransactionID, t.NativeToken, err)
		return false, err
	}

	match := t.Receiver == topicMessage.Receiver &&
		t.Amount == topicMessage.Amount &&
		t.TxReimbursement == topicMessage.TxReimbursement &&
		topicMessage.WrappedToken == wrappedToken
	return match, nil
}

// ProcessSignature processes the signature message, verifying and updating all necessary fields in the DB
func (ss *Service) ProcessSignature(tm message.Message) error {
	// Parse incoming message
	tsm := tm.GetTopicSignatureMessage()
	authMsgBytes, err := auth_message.EncodeBytesFrom(tsm.TransferID, tsm.WrappedToken, tsm.Receiver, tsm.Amount, tsm.TxReimbursement)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to encode the authorisation signature. Error: [%s]", tsm.TransferID, err)
		return err
	}

	// Prepare Signature
	signatureBytes, signatureHex, err := ethhelper.DecodeSignature(tsm.GetSignature())
	if err != nil {
		ss.logger.Errorf("[%s] - Decoding Signature [%s] for TX failed. Error: [%s]", tsm.TransferID, tsm.GetSignature(), err)
		return err
	}
	authMessageStr := hex.EncodeToString(authMsgBytes)

	// Check for duplicated signature
	exists, err := ss.messageRepository.Exist(tsm.TransferID, signatureHex, authMessageStr)
	if err != nil {
		ss.logger.Errorf("[%s] - An error occurred while checking existence from DB. Error: [%s]", tsm.TransferID, err)
		return err
	}
	if exists {
		ss.logger.Errorf("[%s] - Signature already received", tsm.TransferID)
		return err
	}

	// Verify Signature
	address, err := ss.verifySignature(err, authMsgBytes, signatureBytes, tsm.TransferID, authMessageStr)
	if err != nil {
		return err
	}

	ss.logger.Debugf("[%s] - Successfully verified new Signature from [%s]", tsm.TransferID, address.String())

	// Persist in DB
	err = ss.messageRepository.Create(&entity.Message{
		TransferID:           tsm.TransferID,
		Signature:            signatureHex,
		Hash:                 authMessageStr,
		Signer:               address.String(),
		TransactionTimestamp: tm.TransactionTimestamp,
	})
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to save Transaction Message in DB with Signature [%s]. Error: [%s]", tsm.TransferID, signatureHex, err)
		return err
	}

	ss.logger.Infof("[%s] - Successfully processed Signature Message from [%s]", tsm.TransferID, address.String())
	return nil
}

// prepareEthereumMintTask returns the function to be executed for processing the
// Ethereum Mint transaction and HCS topic message with the ethereum TX hash after that
func (ss *Service) prepareEthereumMintTask(transferID, wrappedToken, ethAddress, amount, txReimbursement string, signatures [][]byte, messageHash string) func() {
	ethereumMintTask := func() {
		// Submit and monitor Ethereum TX
		ethTransactor, err := ss.ethSigner.NewKeyTransactor(ss.ethClient.ChainID())
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to establish key transactor. Error: [%s].", transferID, err)
			return
		}

		ethTx, err := ss.contractsService.SubmitSignatures(ethTransactor, transferID, wrappedToken, ethAddress, amount, txReimbursement, signatures)
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to Submit Signatures. Error: [%s]", transferID, err)
			return
		}
		err = ss.transferRepository.UpdateEthTxSubmitted(transferID, ethTx.Hash().String())
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to update status. Error: [%s].", transferID, err)
			return
		}
		ss.logger.Infof("[%s] - Submitted Ethereum Mint TX [%s]", transferID, ethTx.Hash().String())

		onEthTxSuccess, onEthTxRevert := ss.ethTxCallbacks(transferID, ethTx.Hash().String())
		ss.ethClient.WaitForTransaction(ethTx.Hash().String(), onEthTxSuccess, onEthTxRevert, func(err error) {})

		// Submit and monitor HCS Message for Ethereum TX Hash
		hcsTx, err := ss.submitEthTxTopicMessage(transferID, messageHash, ethTx.Hash().String())
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to submit Ethereum TX Hash to Bridge Topic. Error: [%s].", transferID, err)
			return
		}
		err = ss.transferRepository.UpdateStatusEthTxMsgSubmitted(transferID)
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to update status for. Error: [%s].", transferID, err)
			return
		}
		ss.logger.Infof("[%s] - Submitted Ethereum TX Hash [%s] to HCS. Transaction ID [%s].", transferID, ethTx.Hash().String(), hcsTx.String())

		onHcsMessageSuccess, onHcsMessageFail := ss.hcsTxCallbacks(transferID)
		ss.mirrorClient.WaitForTransaction(hcsTx.String(), onHcsMessageSuccess, onHcsMessageFail)

		ss.logger.Infof("[%s] - Successfully processed Ethereum Minting", transferID)
	}
	return ethereumMintTask
}

func getSignatures(messages []entity.Message) ([][]byte, error) {
	var signatures [][]byte

	for _, msg := range messages {
		signature, err := hex.DecodeString(msg.Signature)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, signature)
	}

	return signatures, nil
}

func (ss *Service) verifySignature(err error, authMsgBytes []byte, signatureBytes []byte, transferID, authMessageStr string) (common.Address, error) {
	publicKey, err := crypto.Ecrecover(authMsgBytes, signatureBytes)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to recover public key. Hash [%s]. Error: [%s]", transferID, authMessageStr, err)
		return common.Address{}, err
	}
	unmarshalledPublicKey, err := crypto.UnmarshalPubkey(publicKey)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to unmarshall public key. Error: [%s]", transferID, err)
		return common.Address{}, err
	}
	address := crypto.PubkeyToAddress(*unmarshalledPublicKey)
	if !ss.contractsService.IsMember(address.String()) {
		ss.logger.Errorf("[%s] - Received Signature [%s] is not signed by Bridge member", transferID, authMessageStr)
		return common.Address{}, errors.New(fmt.Sprintf("signer is not signatures member"))
	}
	return address, nil
}

func (ss *Service) submitEthTxTopicMessage(transferID, messageHash, ethereumTxHash string) (*hedera.TransactionID, error) {
	ethTxHashMessage := message.NewEthereumHash(transferID, messageHash, ethereumTxHash)
	ethTxHashBytes, err := ethTxHashMessage.ToBytes()
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to encode Eth TX Hash Message to bytes. Error: [%s]", transferID, err)
		return nil, err
	}

	return ss.hederaClient.SubmitTopicConsensusMessage(ss.topicID, ethTxHashBytes)
}

// awaitTransfer checks until given transfer is found
func (ss *Service) awaitTransfer(transferID string) (*entity.Transfer, error) {
	for {
		t, err := ss.transferRepository.GetByTransactionId(transferID)
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to retrieve Transaction Record. Error: [%s]", transferID, err)
			return nil, err
		}

		if t != nil {
			return t, nil
		}
		ss.logger.Debugf("[%s] - Transfer not yet added. Querying after 5 seconds", transferID)
		time.Sleep(5 * time.Second)
	}
}

func (ss *Service) ethTxCallbacks(transferID, hash string) (onSuccess, onRevert func()) {
	onSuccess = func() {
		ss.logger.Infof("[%s] - Ethereum TX [%s] was successfully mined", transferID, hash)
		err := ss.transferRepository.UpdateEthTxMined(transferID)
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to update status. Error [%s].", transferID, err)
			return
		}
	}

	onRevert = func() {
		ss.logger.Infof("[%s] - Ethereum TX [%s] reverted", transferID, hash)
		err := ss.transferRepository.UpdateEthTxReverted(transferID)
		if err != nil {
			ss.logger.Errorf("[%s] - Failed to update status. Error [%s].", transferID, err)
			return
		}
	}
	return onSuccess, onRevert
}

func (ss *Service) hcsTxCallbacks(txId string) (onSuccess, onFailure func()) {
	onSuccess = func() {
		ss.logger.Infof("[%s] - Ethereum TX Hash message was successfully mined", txId)
		err := ss.transferRepository.UpdateStatusEthTxMsgMined(txId)
		if err != nil {
			ss.logger.Errorf("Failed to update status for TX [%s]. Error [%s].", txId, err)
			return
		}
	}

	onFailure = func() {
		ss.logger.Infof("[%s] - Ethereum TX Hash message failed", txId)
		err := ss.transferRepository.UpdateStatusEthTxMsgFailed(txId)
		if err != nil {
			ss.logger.Errorf("Failed to update status for TX [%s]. Error [%s].", txId, err)
			return
		}
	}
	return onSuccess, onFailure
}

// VerifyEthereumTxAuthenticity performs the validation required prior handling the topic message
// (verifies the submitted TX against the required target contract and arguments passed)
func (ss *Service) VerifyEthereumTxAuthenticity(tm message.Message) (bool, error) {
	ethTxMessage := tm.GetTopicEthTransactionMessage()
	tx, _, err := ss.ethClient.GetClient().TransactionByHash(context.Background(), common.HexToHash(ethTxMessage.EthTxHash))
	if err != nil {
		ss.logger.Warnf("[%s] - Failed to get eth transaction by hash [%s]. Error [%s].", ethTxMessage.TransferID, ethTxMessage.EthTxHash, err)
		return false, err
	}

	// Verify Ethereum TX `to` property
	if strings.ToLower(tx.To().String()) != strings.ToLower(ss.contractsService.GetBridgeContractAddress().String()) {
		ss.logger.Debugf("[%s] - ETH TX [%s] - Failed authenticity - Different To Address [%s].", ethTxMessage.TransferID, ethTxMessage.EthTxHash, tx.To().String())
		return false, nil
	}
	// Verify Ethereum TX `call data`
	txId, ethAddress, wrappedToken, amount, txReimbursement, signatures, err := ethhelper.DecodeBridgeMintFunction(tx.Data())
	if err != nil {
		if errors.Is(err, ethhelper.ErrorInvalidMintFunctionParameters) {
			ss.logger.Debugf("[%s] - ETH TX [%s] - Invalid Mint parameters provided", ethTxMessage.TransferID, ethTxMessage.EthTxHash)
			return false, nil
		}
		return false, err
	}

	if txId != ethTxMessage.TransferID {
		ss.logger.Debugf("[%s] - ETH TX [%s] - Different txn id [%s].", ethTxMessage.TransferID, ethTxMessage.EthTxHash, txId)
		return false, nil
	}

	dbTx, err := ss.transferRepository.GetByTransactionId(ethTxMessage.TransferID)
	if err != nil {
		return false, err
	}
	if dbTx == nil {
		ss.logger.Debugf("[%s] - ETH TX [%s] - Transaction not found in database.", ethTxMessage.TransferID, ethTxMessage.EthTxHash)
		return false, nil
	}

	if dbTx.Amount != amount ||
		dbTx.Receiver != ethAddress ||
		dbTx.TxReimbursement != txReimbursement ||
		wrappedToken != dbTx.WrappedToken {
		ss.logger.Errorf("[%s] - ETH TX [%s] - Invalid arguments.", ethTxMessage.TransferID, ethTxMessage.EthTxHash)
		return false, nil
	}

	// Verify Ethereum TX provided `signatures` authenticity
	messageHash, err := auth_message.EncodeBytesFrom(txId, wrappedToken, ethAddress, amount, txReimbursement)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to encode the authorisation signature to reconstruct required Signature. Error: [%s]", txId, err)
		return false, err
	}

	checkedAddresses := make(map[string]bool)
	for _, signature := range signatures {
		address, err := ethhelper.RecoverSignerFromBytes(messageHash, signature)
		if err != nil {
			return false, err
		}
		if checkedAddresses[address] {
			return false, err
		}

		if !ss.contractsService.IsMember(address) {
			ss.logger.Debugf("[%s] - ETH TX [%s] - Invalid operator process - [%s].", txId, ethTxMessage.EthTxHash, address)
			return false, nil
		}
		checkedAddresses[address] = true
	}

	return true, nil
}

func (ss *Service) ProcessEthereumTxMessage(tm message.Message) error {
	etm := tm.GetTopicEthTransactionMessage()
	err := ss.transferRepository.UpdateEthTxSubmitted(etm.TransferID, etm.EthTxHash)
	if err != nil {
		ss.logger.Errorf("[%s] - Failed to update status to [%s]. Error [%s].", etm.TransferID, transfer.StatusEthTxSubmitted, err)
		return err
	}

	onEthTxSuccess, onEthTxRevert := ss.ethTxCallbacks(etm.TransferID, etm.EthTxHash)
	ss.ethClient.WaitForTransaction(etm.EthTxHash, onEthTxSuccess, onEthTxRevert, func(err error) {})

	return nil
}
