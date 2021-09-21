package message

import (
	"errors"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/limechain/hedera-eth-bridge-validator/app/domain/service"
	"github.com/limechain/hedera-eth-bridge-validator/app/model/message"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	"github.com/limechain/hedera-eth-bridge-validator/proto"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	h       *Handler
	topicId = hedera.TopicID{
		Shard: 0,
		Realm: 0,
		Topic: 1,
	}

	tesm = &proto.TopicEthSignatureMessage{
		SourceChainId:        0,
		TargetChainId:        1,
		TransferID:           "some-transfer-id",
		Asset:                "0.0.1",
		Recipient:            "0xsomeethaddress",
		Amount:               "100",
		Signature:            "custom-signature",
		TransactionTimestamp: 0,
	}
	tsm = message.Message{
		TopicEthSignatureMessage: tesm,
	}
)

func Test_NewHandler(t *testing.T) {
	setup()
	assert.Equal(t, h, NewHandler(topicId.String(), mocks.MTransferRepository, mocks.MMessageRepository, map[int64]service.Contracts{1: mocks.MBridgeContractService}, mocks.MMessageService))
}

func Test_Handle_Fails(t *testing.T) {
	setup()
	h.Handle("invalid-payload")
}

func Test_HandleSignatureMessage_SanityCheckFails(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(false, errors.New("some-error"))
	h.handleSignatureMessage(tsm)
	mocks.MMessageService.AssertNotCalled(t, "ProcessSignature", tsm)
}

func Test_HandleSignatureMessage_SanityCheckIsNotValid(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(false, nil)
	h.handleSignatureMessage(tsm)
	mocks.MMessageService.AssertNotCalled(t, "ProcessSignature", tsm)
}

func Test_HandleSignatureMessage_ProcessSignatureFails(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(true, nil)
	mocks.MMessageService.On("ProcessSignature", tsm).Return(errors.New("some-error"))
	h.handleSignatureMessage(tsm)
}

func Test_HandleSignatureMessage_MajorityReached(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(true, nil)
	mocks.MMessageService.On("ProcessSignature", tsm).Return(nil)
	mocks.MMessageRepository.On("Get", tsm.TransferID).Return([]entity.Message{{}, {}, {}}, nil)
	mocks.MBridgeContractService.On("GetMembers").Return([]string{"", "", ""})
	mocks.MTransferRepository.On("UpdateStatusCompleted", tsm.TransferID).Return(nil)
	h.handleSignatureMessage(tsm)
}

func Test_Handle(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(true, nil)
	mocks.MMessageService.On("ProcessSignature", tsm).Return(nil)
	mocks.MMessageRepository.On("Get", tsm.TransferID).Return([]entity.Message{{}, {}, {}}, nil)
	mocks.MBridgeContractService.On("GetMembers").Return([]string{"", "", ""})
	mocks.MTransferRepository.On("UpdateStatusCompleted", tsm.TransferID).Return(nil)
	h.Handle(&tsm)
}

func Test_HandleSignatureMessage_UpdateStatusCompleted_Fails(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(true, nil)
	mocks.MMessageService.On("ProcessSignature", tsm).Return(nil)
	mocks.MMessageRepository.On("Get", tsm.TransferID).Return([]entity.Message{{}, {}, {}}, nil)
	mocks.MBridgeContractService.On("GetMembers").Return([]string{"", "", ""})
	mocks.MTransferRepository.On("UpdateStatusCompleted", tsm.TransferID).Return(errors.New("some-error"))
	h.handleSignatureMessage(tsm)
}

func Test_HandleSignatureMessage_CheckMajority_Fails(t *testing.T) {
	setup()
	mocks.MMessageService.On("SanityCheckSignature", tsm).Return(true, nil)
	mocks.MMessageService.On("ProcessSignature", tsm).Return(nil)
	mocks.MMessageRepository.On("Get", tsm.TransferID).Return([]entity.Message{{}, {}, {}}, errors.New("some-error"))
	h.handleSignatureMessage(tsm)
	mocks.MBridgeContractService.AssertNotCalled(t, "GetMembers")
	mocks.MTransferRepository.AssertNotCalled(t, "UpdateStatusCompleted", tsm.TransferID)
}

func setup() {
	mocks.Setup()
	h = &Handler{
		transferRepository: mocks.MTransferRepository,
		messageRepository:  mocks.MMessageRepository,
		contracts:          map[int64]service.Contracts{1: mocks.MBridgeContractService},
		messages:           mocks.MMessageService,
		logger:             config.GetLoggerFor(fmt.Sprintf("Topic [%s] Handler", topicId.String())),
	}
}