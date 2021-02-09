package e2e

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	whbar "github.com/limechain/hedera-eth-bridge-validator/app/clients/ethereum/contracts/whbar"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	common "github.com/ethereum/go-ethereum/common"
	"github.com/hashgraph/hedera-sdk-go"
	ethclient "github.com/limechain/hedera-eth-bridge-validator/app/clients/ethereum"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	validatorproto "github.com/limechain/hedera-eth-bridge-validator/proto"
	"google.golang.org/protobuf/proto"
)

func Test_E2E(t *testing.T) {
	configuration := config.LoadTestConfig()

	memo := "0x7cFae2deF15dF86CfdA9f2d25A361f1123F42eDD1126221237211"
	whbarReceiverAddress := common.HexToAddress("0x7cFae2deF15dF86CfdA9f2d25A361f1123F42eDD")

	hBarAmount := 0.0001
	validatorsCount := 3
	ethSignaturesCollected := 0
	ethTransMsgCollected := 0
	ethTransactionHash := ""

	whbarContractAddress := common.HexToAddress(configuration.Hedera.Eth.WhbarContractAddress)
	acc, _ := hedera.AccountIDFromString(configuration.Hedera.Client.Operator.AccountId)
	receiving, _ := hedera.AccountIDFromString(configuration.Hedera.Watcher.CryptoTransfer.Accounts[0].Id)
	topicID, _ := hedera.TopicIDFromString(configuration.Hedera.Watcher.ConsensusMessage.Topics[0].Id)
	ethConfig := configuration.Hedera.Eth

	accID, _ := hedera.AccountIDFromString(configuration.Hedera.Client.Operator.AccountId)
	pK, _ := hedera.PrivateKeyFromString(configuration.Hedera.Client.Operator.PrivateKey)

	client := initClient(accID, pK)
	ethClient := ethclient.NewEthereumClient(ethConfig)
	whbarInstance, err := whbar.NewWhbar(whbarContractAddress, ethClient.Client)

	// Get the wrapped hbar balance of the receiver before the transfer
	whbarBalanceBefore, err := whbarInstance.BalanceOf(&bind.CallOpts{}, whbarReceiverAddress)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("WHBAR balance before transaction: [%s]\n", whbarBalanceBefore)

	// Get custodian hbar balance before transfer
	recieverBalance, err := hedera.NewAccountBalanceQuery().
		SetAccountID(receiving).
		Execute(client)

	fmt.Println(fmt.Sprintf(`HBAR custodian balance before transaction: [%d]`, recieverBalance.Hbars.AsTinybar()))

	// Get the transaction receipt to verify the transaction was executed
	transactionResponse, err := sendTransactionToCustodialAccount(acc, receiving, memo, hBarAmount, client)
	transactionReceipt, err := transactionResponse.GetReceipt(client)

	if err != nil {
		fmt.Println(fmt.Sprintf(`Transaction unsuccessful, Error: [%s]`, err))
		t.Fatal(err)
	} else {
		fmt.Println(fmt.Sprintf(`Successfully sent HBAR to custodian adress, Status: [%s]`, transactionReceipt.Status))
	}

	// Get custodian hbar balance after transfer
	recieverBalanceNew, err := hedera.NewAccountBalanceQuery().
		SetAccountID(receiving).
		Execute(client)

	fmt.Println(fmt.Sprintf(`HBAR custodian balance after transaction: [%d]`, recieverBalanceNew.Hbars.AsTinybar()))

	// Verify that the custodial address has recieved exactly the amount sent
	if (recieverBalanceNew.Hbars.AsTinybar() - recieverBalance.Hbars.AsTinybar()) != hedera.HbarFrom(hBarAmount, "hbar").AsTinybar() {
		t.Fatal(`Expected to recieve the exact transfer amount of hbar`)
	}

	// Subscribe to Topic
	hedera.NewTopicMessageQuery().
		SetStartTime(time.Unix(0, time.Now().UnixNano())).
		SetTopicID(topicID).
		Subscribe(
			client,
			func(response hedera.TopicMessage) {
				msg := &validatorproto.TopicSubmissionMessage{}
				proto.Unmarshal(response.Contents, msg)

				if msg.GetType() == validatorproto.TopicSubmissionType_EthSignature {
					//Verify that all the submitted messages have signed the same transaction
					if msg.GetTopicSignatureMessage().TransactionId != fromHederaTransactionID(transactionResponse.TransactionID) {
						t.Fatal(`Expected signature message to contain the transaction id`)
					}
					ethSignaturesCollected++
				}

				if msg.GetType() == validatorproto.TopicSubmissionType_EthTransaction {
					//Verify that the eth transaction message has been submitted
					if msg.GetTopicEthTransactionMessage().TransactionId != fromHederaTransactionID(transactionResponse.TransactionID) {
						t.Fatal(`Expected ethereum transaction message to contain the transaction id`)
					}
					ethTransactionHash = msg.GetTopicEthTransactionMessage().GetEthTxHash()
					ethTransMsgCollected++
				}
			},
		)

	// Wait for topic consensus messages to arrive
	time.Sleep(60 * time.Second)

	// Check that all the validators have submitted a message with authorisation signature
	if ethSignaturesCollected != validatorsCount {
		t.Fatal(`Expected the count of collected signatures to equal the number of validators`)
	}

	// Verify the exactly on eth transaction hash has been submitted
	if ethTransMsgCollected != 1 {
		t.Fatal(`Expected to submit exaclty 1 ethereum transaction in topic`)
	}

	success, err := ethClient.WaitForTransactionSuccess(common.HexToHash(ethTransactionHash))
	// Verify that the eth transaction has been mined and succeeded
	if success == false {
		t.Fatal(`Expected to mine successfully the broadcasted ethereum transaction`)
	}

	// Get the wrapped hbar balance of the receiver after the transfer
	whbarBalanceAfter, err := whbarInstance.BalanceOf(&bind.CallOpts{}, whbarReceiverAddress)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("WHBAR balance after transaction: [%s]\n", whbarBalanceAfter)

	// Verify that the ethereum address hass recieved the exact transfer amount of WHBARs
	if (whbarBalanceAfter.Int64() - whbarBalanceBefore.Int64()) != hedera.HbarFrom(hBarAmount, "hbar").AsTinybar() {
		t.Fatal(`Expected to recieve the exact transfer amount of WHBAR`)
	}

}

func sendTransactionToCustodialAccount(senderAccount hedera.AccountID, custodialAccount hedera.AccountID, memo string, hBarAmount float64, client *hedera.Client) (hedera.TransactionResponse, error) {
	fmt.Println(fmt.Sprintf(`Sending [%v] Hbars through the Bridge. Transaction Memo: [%s]`, hBarAmount, memo))
	res, _ := hedera.NewTransferTransaction().AddHbarSender(senderAccount, hedera.HbarFrom(hBarAmount, "hbar")).
		AddHbarRecipient(custodialAccount, hedera.HbarFrom(hBarAmount, "hbar")).
		SetTransactionMemo(memo).
		Execute(client)
	rec, err := res.GetReceipt(client)
	fmt.Println(fmt.Sprintf(`TX broadcasted. ID [%s], Status: [%s]`, res.TransactionID, rec.Status))
	time.Sleep(1 * time.Second)
	return res, err
}

func initClient(accID hedera.AccountID, pK hedera.PrivateKey) *hedera.Client {
	client := hedera.ClientForTestnet()
	client.SetOperator(accID, pK)
	return client
}

func fromHederaTransactionID(id hedera.TransactionID) string {
	stringTxID := id.String()
	split := strings.Split(stringTxID, "@")
	accID := split[0]
	split = strings.Split(split[1], ".")

	return fmt.Sprintf("%s-%s-%s", accID, split[0], split[1])
}
