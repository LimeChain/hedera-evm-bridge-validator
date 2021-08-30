package fee_message

import (
	"errors"
	model "github.com/limechain/hedera-eth-bridge-validator/app/model/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity"
	"github.com/limechain/hedera-eth-bridge-validator/app/persistence/entity/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/constants"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks/service"
	"testing"
)

var (
	mt = model.Transfer{
		TransactionId: "0.0.0-0000000-1234",
		Receiver:      "0x12345",
		Amount:        "10000000000",
		NativeAsset:   constants.Hbar,
		TargetAsset:   "0x45678",
	}
)

func InitializeHandler() (*Handler, *service.MockTransferService) {
	mocks.Setup()

	return NewHandler(mocks.MTransferService), mocks.MTransferService
}

func Test_Handle(t *testing.T) {
	ctHandler, mockedService := InitializeHandler()

	tx := &entity.Transfer{
		TransactionID: mt.TransactionId,
		Receiver:      mt.Receiver,
		Amount:        mt.Amount,
		NativeAsset:   mt.NativeAsset,
		TargetAsset:   mt.TargetAsset,
		Status:        transfer.StatusInitial,
	}

	mockedService.On("InitiateNewTransfer", mt).Return(tx, nil)
	mockedService.On("ProcessTransfer", mt).Return(nil)

	ctHandler.Handle(&mt)

	mockedService.AssertCalled(t, "InitiateNewTransfer", mt)
	mockedService.AssertCalled(t, "ProcessTransfer", mt)
}

func Test_Handle_Encoding_Fails(t *testing.T) {
	ctHandler, mockedService := InitializeHandler()

	invalidTransferPayload := []byte{1, 2, 1}

	ctHandler.Handle(invalidTransferPayload)

	mockedService.AssertNotCalled(t, "InitiateNewTransfer")
	mockedService.AssertNotCalled(t, "ProcessTransfer")
}

func Test_Handle_InitiateNewTransfer_Fails(t *testing.T) {
	ctHandler, mockedService := InitializeHandler()

	mockedService.On("InitiateNewTransfer", mt).Return(nil, errors.New("some-error"))

	ctHandler.Handle(&mt)

	mockedService.AssertNotCalled(t, "ProcessTransfer")
}

func Test_Handle_StatusNotInitial_Fails(t *testing.T) {
	ctHandler, mockedService := InitializeHandler()

	tx := &entity.Transfer{
		TransactionID: mt.TransactionId,
		Receiver:      mt.Receiver,
		Amount:        mt.Amount,
		Status:        transfer.StatusCompleted,
	}

	mockedService.On("InitiateNewTransfer", mt).Return(tx, nil)

	ctHandler.Handle(&mt)

	mockedService.AssertNotCalled(t, "ProcessTransfer")
}

func Test_Handle_ProcessTransfer_Fails(t *testing.T) {
	ctHandler, mockedService := InitializeHandler()

	tx := &entity.Transfer{
		TransactionID: mt.TransactionId,
		Receiver:      mt.Receiver,
		Amount:        mt.Amount,
		Status:        transfer.StatusInitial,
	}

	mockedService.On("InitiateNewTransfer", mt).Return(tx, nil)
	mockedService.On("ProcessTransfer", mt).Return(errors.New("some-error"))

	ctHandler.Handle(&mt)
}