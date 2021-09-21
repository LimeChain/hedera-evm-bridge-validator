package service

import (
	"github.com/limechain/hedera-eth-bridge-validator/app/model/message"
	"github.com/limechain/hedera-eth-bridge-validator/app/model/transfer"
	"github.com/stretchr/testify/mock"
)

type MockMessageService struct {
	mock.Mock
}

func (m *MockMessageService) SignMessage(transfer transfer.Transfer) (*message.Message, error) {
	args := m.Called(transfer)
	if args[1] == nil {
		return args[0].(*message.Message), nil
	}
	return args[0].(*message.Message), args[1].(error)
}

// SanityCheckSignature performs any validation required prior handling the topic message
// (verifies metadata against the corresponding Transaction record)
func (m *MockMessageService) SanityCheckSignature(tm message.Message) (bool, error) {
	args := m.Called(tm)
	if args[1] == nil {
		return args[0].(bool), nil
	}
	return args[0].(bool), args[1].(error)
}

// ProcessSignature processes the signature message, verifying and updating all necessary fields in the DB
func (m *MockMessageService) ProcessSignature(tm message.Message) error {
	args := m.Called(tm)
	if args[0] == nil {
		return nil
	}
	return args[0].(error)
}