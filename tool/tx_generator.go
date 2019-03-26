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

package tool

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"math/rand"
	"sync"
	"time"
	"github.com/kardiachain/go-kardia/configs"
)

const (
	defaultNumTx    = 10
	defaultGasLimit = 10 // currently we don't care about tx fee and cost.
)

var (
	defaultAmount   = big.NewInt(10)
	defaultGasPrice = big.NewInt(10)
)

type GeneratorTool struct {
	nonceMap map[string]uint64 // Map of nonce counter for each address

	mu sync.Mutex
}

func NewGeneratorTool() *GeneratorTool {
	genTool := new(GeneratorTool)

	genTool.nonceMap = make(map[string]uint64)

	return genTool
}

// GenerateTx generate an array of transfer transactions within genesis accounts.
// numTx: number of transactions to send, default to 10.
func (genTool *GeneratorTool) GenerateTx(numTx int) []*types.Transaction {
	if numTx <= 0 {
		numTx = defaultNumTx
	}
	result := make([]*types.Transaction, numTx)
	addrKeySize := len(configs.GenesisAddrKeys)
	var keys []*ecdsa.PrivateKey
	var addresses []common.Address

	for addrS, privateKey := range configs.GenesisAddrKeys {
		pkByte, _ := hex.DecodeString(privateKey)
		keys = append(keys, crypto.ToECDSAUnsafe(pkByte))
		addresses = append(addresses, common.HexToAddress(addrS))
	}

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

func GenerateRandomTxWithState(numTx int, stateDb *state.StateDB) []*types.Transaction {
	if numTx <= 0 {
		numTx = defaultNumTx
	}

	result := make([]*types.Transaction, numTx)
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses()
		nonce := stateDb.GetNonce(crypto.PubkeyToAddress(senderKey.PublicKey))
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
	}
	return result
}

func GenerateSmcCall(senderKey *ecdsa.PrivateKey, address common.Address, input []byte, stateDb *state.ManagedState) *types.Transaction {
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	nonce := stateDb.GetNonce(senderAddress)
	tx, err := types.SignTx(types.NewTransaction(
		nonce,
		address,
		big.NewInt(0),
		5000000,
		big.NewInt(1),
		input,
	), senderKey)
	if err != nil {
		panic(fmt.Sprintf("Fail to generate smc call: %v", err))
	}
	return tx
}

func GenerateCreateSmcCall(senderKey *ecdsa.PrivateKey, input []byte, stateDb *state.StateDB) *types.Transaction {
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	nonce := stateDb.GetNonce(senderAddress)
	tx, err := types.SignTx(types.NewContractCreation(
		nonce,
		defaultAmount,
		60000,
		big.NewInt(1),
		input,
	), senderKey)
	if err != nil {
		panic(fmt.Sprintf("Fail to generate smc call: %v", err))
	}
	return tx
}

func randomTxAddresses() (senderKey *ecdsa.PrivateKey, toAddr common.Address) {
	for {
		senderKey = randomGenesisPrivateKey()
		toAddr = randomGenesisAddress()

		if crypto.PubkeyToAddress(senderKey.PublicKey) != toAddr {
			break
		}
	}
	return senderKey, toAddr
}

func randomGenesisAddress() common.Address {
	size := len(configs.GenesisAddrKeys)
	randomI := rand.Intn(size)
	index := 0
	for addrS := range configs.GenesisAddrKeys {
		if index == randomI {
			return common.HexToAddress(addrS)
		}
		index++
	}
	panic("impossible failure")
}

func randomAddress() common.Address {
	rand.Seed(time.Now().UTC().UnixNano())
	address := make([]byte, 20)
	rand.Read(address)
	return common.BytesToAddress(address)
}

func randomGenesisPrivateKey() *ecdsa.PrivateKey {
	size := len(configs.GenesisAddrKeys)
	randomI := rand.Intn(size)
	index := 0
	for _, privateKey := range configs.GenesisAddrKeys {
		if index == randomI {
			pkByte, _ := hex.DecodeString(privateKey)
			return crypto.ToECDSAUnsafe(pkByte)
		}
		index++
	}
	panic("impossible failure")
}

func GetRandomGenesisAccount() common.Address {
	size := len(configs.GenesisAccounts)
	randomI := rand.Intn(size)
	index := 0
	for addrS := range configs.GenesisAccounts {
		if index == randomI {
			return common.HexToAddress(addrS)
		}
		index++
	}
	panic("impossible failure")
}
