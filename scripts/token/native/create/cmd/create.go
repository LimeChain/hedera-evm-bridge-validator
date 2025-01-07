/*
 * Copyright 2022 LimeChain Ltd.
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
	"flag"
	"fmt"
	"strings"

	"github.com/hashgraph/hedera-sdk-go/v2"
	mirrorNode "github.com/limechain/hedera-eth-bridge-validator/app/clients/hedera/mirror-node"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	"github.com/limechain/hedera-eth-bridge-validator/scripts/client"
	"github.com/limechain/hedera-eth-bridge-validator/scripts/token/associate"
	"github.com/limechain/hedera-eth-bridge-validator/scripts/token/native/create"
)

const (
	HederaMainnetNetworkId = 295
	HederaTestnetNetworkId = 296
)

func main() {
	privateKey := flag.String("privateKey", "0x0", "Hedera Private Key")
	accountID := flag.String("accountID", "0.0", "Hedera Account ID")
	network := flag.String("network", "", "Hedera Network Type")
	bridgeID := flag.String("bridgeID", "0.0", "Bridge account ID")
	memberPrKeys := flag.String("memberPrKeys", "", "The count of the members")
	tokenName := flag.String("name", "Hedera Native Generic Token", "token name")
	tokenSymbol := flag.String("symbol", "HNT", "token symbol")
	decimals := flag.Uint("decimals", 8, "decimals")
	setSupplyKey := flag.Bool("setSupplyKey", true, "Sets supply key to be the deployer")
	flag.Parse()
	if *privateKey == "0x0" {
		panic("Private key was not provided")
	}
	if *accountID == "0.0" {
		panic("Account id was not provided")
	}
	if *bridgeID == "0.0" {
		panic("Bridge id was not provided")
	}

	fmt.Println("-----------Start-----------")
	cl := client.Init(*privateKey, *accountID, *network)

	mirrorNodeConfigByNetwork := map[uint64]config.MirrorNode{
		HederaMainnetNetworkId: {
			ClientAddress: "mainnet-public.mirrornode.hedera.com/:443",
			ApiAddress:    "https://mainnet-public.mirrornode.hedera.com/api/v1/",
		},
		HederaTestnetNetworkId: {
			ClientAddress: "hcs.testnet.mirrornode.hedera.com:5600",
			ApiAddress:    "https://testnet.mirrornode.hedera.com/api/v1/",
		},
	}

	var hederaNetworkId uint64
	if *network != "testnet" && *setSupplyKey {
		var confirmation string
		fmt.Printf("Network is set to [%s] and setSupplyKey is set to [%v]. Are you sure you what to proceed?\n", *network, *setSupplyKey)
		fmt.Println("Y/N:")
		fmt.Scanln(&confirmation)
		if confirmation != "Y" {
			panic("Exiting")
		}
		hederaNetworkId = HederaMainnetNetworkId
	} else {
		hederaNetworkId = HederaTestnetNetworkId
	}

	mirrorNodeClient := mirrorNode.NewClient(mirrorNodeConfigByNetwork[hederaNetworkId])
	membersSlice := strings.Split(*memberPrKeys, ",")

	var custodianKey []hedera.PrivateKey
	var membersPublicKey []hedera.PublicKey
	for i := 0; i < len(membersSlice); i++ {
		privateKeyFromStr, err := hedera.PrivateKeyFromString(membersSlice[i])
		if err != nil {
			panic(err)
		}

		membersPublicKey = append(membersPublicKey, privateKeyFromStr.PublicKey())
		custodianKey = append(custodianKey, privateKeyFromStr)
	}

	tokenId, err := create.CreateNativeFungibleToken(
		cl,
		cl.GetOperatorAccountID(),
		*tokenName,
		*tokenSymbol,
		*decimals,
		100000000000000,
		hedera.HbarFrom(20, "hbar"),
		*setSupplyKey,
	)
	if err != nil {
		panic(err)
	}

	bridgeIDFromString, err := hedera.AccountIDFromString(*bridgeID)
	if err != nil {
		panic(err)
	}

	receipt, err := associate.TokenToAccountWithCustodianKey(cl, *tokenId, bridgeIDFromString, custodianKey)
	if err != nil {
		panic(err)
	}
	fmt.Println("Token ID:", tokenId)
	fmt.Println("Associate transaction status:", receipt.Status)

	// associate token with members
	for memberPubKeyIndex, memberPubKey := range membersPublicKey {
		accounts, err := mirrorNodeClient.GetAccountByPublicKey(memberPubKey.String())
		if err != nil {
			panic(fmt.Errorf("cannot obtain account by public key: %w", err))
		}

		if len(accounts.Accounts) == 0 {
			panic("cannot find account by public key")
		} else if len(accounts.Accounts) != 1 {
			panic("multiple accounts found for public key - " + memberPubKey.String())
		}

		hAccount, err := hedera.AccountIDFromString(accounts.Accounts[0].Account)
		if err != nil {
			panic(fmt.Errorf("cannot convert string to hedera account: %w", err))
		}

		hClient := client.Init(membersSlice[memberPubKeyIndex], hAccount.String(), *network)
		tokenToAccountReceipt, err := associate.TokenToAccount(hClient, *tokenId, hAccount)
		if err != nil {
			panic(fmt.Errorf("failed to associate token to account: %w", err))
		}
		fmt.Printf("Account[%s] associated with token[%s], tx status: %s\n",
			hAccount.String(), tokenId.String(), tokenToAccountReceipt.Status)
	}
}
