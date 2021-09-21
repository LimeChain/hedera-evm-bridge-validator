package mint_hts

import (
	"errors"
	"github.com/hashgraph/hedera-sdk-go/v2"
	model "github.com/limechain/hedera-eth-bridge-validator/app/model/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/status"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	"github.com/limechain/hedera-eth-bridge-validator/constants"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

var (
	h  *Handler
	tr = &model.Transfer{
		TransactionId: "some-tx-id",
		SourceChainId: 0,
		TargetChainId: 1,
		NativeChainId: 0,
		SourceAsset:   constants.Hbar,
		TargetAsset:   "0xsomeethaddress",
		NativeAsset:   constants.Hbar,
		Receiver:      "0xsomeotherethaddress",
		Amount:        "100",
		Timestamp:     "1",
	}
	accountId = hedera.AccountID{
		Shard:   0,
		Realm:   0,
		Account: 1,
	}
)

func Test_NewHandler(t *testing.T) {
	setup()
	assert.Equal(t, h, NewHandler(mocks.MScheduleRepository, accountId.String(), mocks.MHederaMirrorClient, mocks.MTransferService, mocks.MReadOnlyService))
}

func Test_Handle(t *testing.T) {
	setup()
	mocks.MTransferService.On("InitiateNewTransfer", *tr).Return(&entity.Transfer{Status: status.Initial}, nil)
	mocks.MReadOnlyService.On("FindTransfer", mock.Anything, mock.Anything, mock.Anything)
	h.Handle(tr)
}

func Test_Handle_FindTransfer(t *testing.T) {
	setup()
	mocks.MTransferService.On("InitiateNewTransfer", *tr).Return(&entity.Transfer{Status: status.Initial}, nil)
	mocks.MReadOnlyService.On("FindTransfer", mock.Anything, mock.Anything, mock.Anything)
	h.Handle(tr)
}

func Test_Handle_NotInitialFails(t *testing.T) {
	setup()
	mocks.MTransferService.On("InitiateNewTransfer", *tr).Return(&entity.Transfer{Status: "not-initial"}, nil)
	h.Handle(tr)
}

func Test_Handle_InvalidPayload(t *testing.T) {
	setup()
	h.Handle("invalid-payload")
	mocks.MTransferService.AssertNotCalled(t, "InitiateNewTransfer", *tr)
}

func Test_Handle_InitiateNewTransferFails(t *testing.T) {
	setup()
	mocks.MTransferService.On("InitiateNewTransfer", *tr).Return(nil, errors.New("some-error"))
	h.Handle(tr)
}

func setup() {
	mocks.Setup()
	h = &Handler{
		bridgeAccount:      accountId,
		transfersService:   mocks.MTransferService,
		scheduleRepository: mocks.MScheduleRepository,
		mirrorNode:         mocks.MHederaMirrorClient,
		readOnlyService:    mocks.MReadOnlyService,
		logger:             config.GetLoggerFor("Hedera Mint and Transfer Handler"),
	}
}
