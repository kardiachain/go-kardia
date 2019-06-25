package main

/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"math/rand"
	"sync"
)

type Account struct {
	Address    string      `json:"address"`
	PrivateKey string      `json:"privateKey"`
}

const defaultGasLimit = 10 // currently we don't care about tx fee and cost.

var (
	defaultAmount   = big.NewInt(10)
	defaultGasPrice = big.NewInt(10)
)

type GeneratorTool struct {
	nonceMap map[string]uint64 // Map of nonce counter for each address
	accounts []Account
	mu sync.Mutex
}

func NewGeneratorTool(accounts []Account, nonceMap map[string]uint64) *GeneratorTool {
	return &GeneratorTool{
		accounts: accounts,
		nonceMap: nonceMap,
	}
}

// GenerateTx generate an array of transfer transactions within genesis accounts.
// numTx: number of transactions to send, default to 10.
func (genTool *GeneratorTool) GenerateTx(numTx int) []*types.Transaction {
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make([]*types.Transaction, numTx)
	var keys []*ecdsa.PrivateKey
	var addresses []common.Address

	for _, account := range genTool.accounts {

		if account.Address != configs.KardiaAccountToCallSmc { // skip account call smc
			pkByte, _ := hex.DecodeString(account.PrivateKey)
			keys = append(keys, crypto.ToECDSAUnsafe(pkByte))
			addresses = append(addresses, common.HexToAddress(account.Address))
		}
	}
	addrKeySize := len(addresses)

	genTool.mu.Lock()
	for i := 0; i < numTx; i++ {
		senderKey := keys[i%addrKeySize]
		toAddr := addresses[(i+1)%addrKeySize]

		senderAddrS := crypto.PubkeyToAddress(senderKey.PublicKey).String()
		nonce := genTool.nonceMap[senderAddrS]

		tx, err := types.SignTx(types.NewTransaction(
			nonce,
			toAddr,
			defaultAmount,
			1000,
			big.NewInt(1),
			nil,
		), senderKey)
		if err != nil {
			panic(fmt.Sprintf("Fail to sign generated tx: %v", err))
		}
		result[i] = tx
		nonce += 1
		genTool.nonceMap[senderAddrS] = nonce
	}
	genTool.mu.Unlock()
	return result
}

func (genTool *GeneratorTool) GetNonce(address string) uint64 {
	return genTool.nonceMap[address]
}

func (genTool *GeneratorTool) GenerateRandomTxWithState(numTx uint64, state *state.ManagedState) []*types.Transaction {
	genTool.mu.Lock()
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make([]*types.Transaction, numTx)
	for i := uint64(0); i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses(genTool.accounts)
		senderPublicKey := crypto.PubkeyToAddress(senderKey.PublicKey)
		//nonce := stateDb.GetNonce(senderPublicKey)
		senderAddrS := senderPublicKey.String()

		if _, ok := genTool.nonceMap[senderAddrS]; !ok {
			genTool.nonceMap[senderAddrS] = 1
		}

		//nonceDb := state.GetNonce(senderPublicKey)
		//if genTool.nonceMap[senderAddrS] < nonceDb {
		//	genTool.nonceMap[senderAddrS] = nonceDb + 1
		//}

		nonce := genTool.nonceMap[senderAddrS]
		//log.Error("generate tx", "addr", senderAddrS, "nonce", nonce, "nonceMap", genTool.nonceMap[senderAddrS])

		//get nonce from sender mapping
		//nonceMap := genTool.nonceMap[senderAddrS]
		//if nonce < nonceMap { // check nonce from statedb and nonceMap
		//	nonce = nonceMap
		//}

		tx, err := types.SignTx(types.NewTransaction(
			nonce,
			toAddr,
			defaultAmount,
			defaultGasLimit,
			defaultGasPrice,
			nil,
		), senderKey)
		if err != nil {
			panic(fmt.Sprintf("Fail to sign generated tx: %v", err))
		}
		result[i] = tx
		nonce += 1
		genTool.nonceMap[senderAddrS] = nonce
	}
	genTool.mu.Unlock()
	return result
}

func randomTxAddresses(accounts []Account) (senderKey *ecdsa.PrivateKey, toAddr common.Address) {
	for {
		senderKey = randomGenesisPrivateKey(accounts)
		toAddr = randomGenesisAddress()
		if crypto.PubkeyToAddress(senderKey.PublicKey) != toAddr {
			// skip senderAddr = toAddr && senderAddr that call smc
			break
		}
	}
	return senderKey, toAddr
}

func randomGenesisAddress() common.Address {
	size := len(GenesisAddrKeys)
	randomI := rand.Intn(size)
	index := 0
	for addrS := range GenesisAddrKeys {
		if index == randomI {
			return common.HexToAddress(addrS)
		}
		index++
	}
	panic("impossible failure")
}

func randomGenesisPrivateKey(accounts []Account) *ecdsa.PrivateKey {
	size := len(accounts)
	randomI := rand.Intn(size)
	index := 0
	for _, account := range accounts {
		if index == randomI {
			pkByte, _ := hex.DecodeString(account.PrivateKey)
			return crypto.ToECDSAUnsafe(pkByte)
		}
		index++
	}
	panic("impossible failure")
}

func GetAccounts(genesisAccounts map[string]string) []Account {
	accounts := make([]Account, 0)
	for key, value := range genesisAccounts {
		accounts = append(accounts, Account{
			Address: key,
			PrivateKey: value,
		})
	}

	return accounts
}
