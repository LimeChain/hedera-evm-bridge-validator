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

package message

import (
	"encoding/base64"
	"github.com/golang/protobuf/proto"
	"github.com/limechain/hedera-eth-bridge-validator/app/helper/timestamp"
	model "github.com/limechain/hedera-eth-bridge-validator/proto"
)

// Message serves as a model between Topic Message Watcher and Handler
type Message struct {
	*model.TopicMessage
}

// FromBytes instantiates new TopicMessage protobuf used internally by the Watchers/Handlers
func FromBytes(data []byte) (*Message, error) {
	msg := &model.TopicMessage{}
	err := proto.Unmarshal(data, msg)
	if err != nil {
		return nil, err
	}
	return &Message{msg}, nil
}

// FromBytesWithTS instantiates new TopicMessage protobuf used internally by the Watchers/Handlers
func FromBytesWithTS(data []byte, ts int64) (*Message, error) {
	msg, err := FromBytes(data)
	if err != nil {
		return nil, err
	}
	msg.TransactionTimestamp = ts
	return msg, nil
}

// FromString instantiates new Topic Message protobuf from string `content` and `timestamp`
func FromString(data, ts string) (*Message, error) {
	t, err := timestamp.FromString(ts)
	if err != nil {
		return nil, err
	}

	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	return FromBytesWithTS(bytes, t)
}

// NewSignatureMessage instantiates Signature Message struct ready for submission to the Bridge Topic
func NewSignature(transferID, receiver, amount, txReimbursement, gasPrice, signature, wrappedToken string) *Message {
	topicMsg := &model.TopicMessage{
		Type: model.TopicMessageType_EthSignature,
		Message: &model.TopicMessage_TopicSignatureMessage{
			TopicSignatureMessage: &model.TopicEthSignatureMessage{
				TransferID:      transferID,
				Receiver:        receiver,
				Amount:          amount,
				TxReimbursement: txReimbursement,
				GasPrice:        gasPrice,
				Signature:       signature,
				WrappedToken:    wrappedToken,
			},
		},
	}
	return &Message{topicMsg}
}

// NewEthereumHash instantiates Ethereum Transaction Hash Message struct ready for submission to the Bridge Topic
func NewEthereumHash(transferID, messageHash, ethereumTxHash string) *Message {
	topicMsg := &model.TopicMessage{
		Type: model.TopicMessageType_EthTransaction,
		Message: &model.TopicMessage_TopicEthTransactionMessage{
			TopicEthTransactionMessage: &model.TopicEthTransactionMessage{
				TransferID: transferID,
				Hash:       messageHash,
				EthTxHash:  ethereumTxHash,
			},
		},
	}
	return &Message{topicMsg}
}

// ToBytes marshals the underlying protobuf Message into bytes
func (tm *Message) ToBytes() ([]byte, error) {
	return proto.Marshal(tm.TopicMessage)
}