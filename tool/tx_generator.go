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
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/types"
)

type Account struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
}

const (
	DefaultGasLimit             = configs.TxGas // currently we don't care about tx fee and cost.
	DefaultFaucetAcc            = "0x2BB7316884C7568F2C6A6aDf2908667C0d241A66"
	DefaultFaucetPrivAcc        = "4561f7d91a4f95ef0a72550fa423febaad3594f91611f9a2b10a7af4d3deb9ed"
	DefaultGenRandomWithStateTx = 1
	DefaultGenRandomTx          = 2
)

var (
	defaultAmount   = big.NewInt(10)
	defaultGasPrice = big.NewInt(1)
)

type GeneratorTool struct {
	nonceMap map[string]uint64 // Map of nonce counter for each address
	accounts []Account
	mu       sync.Mutex
}

func NewGeneratorTool(accounts []Account) *GeneratorTool {
	genTool := new(GeneratorTool)
	genTool.nonceMap = make(map[string]uint64, 0)
	genTool.accounts = accounts
	return genTool
}

// GenerateTx generate an array of transfer transactions within genesis accounts.
// numTx: number of transactions to send, default to 10.
func (genTool *GeneratorTool) GenerateTx(numTx int) []*types.Transaction {
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make([]*types.Transaction, numTx)
	genTool.mu.Lock()
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses(genTool.accounts)
		senderPublicKey := crypto.PubkeyToAddress(senderKey.PublicKey)
		senderAddrS := senderPublicKey.String()
		nonce := genTool.nonceMap[senderAddrS]
		amount := big.NewInt(int64(RandomInt(10, 20)))
		amount = amount.Mul(amount, big.NewInt(int64(math.Pow10(18))))
		tx, err := types.SignTx(types.HomesteadSigner{}, types.NewTransaction(
			nonce,
			toAddr,
			amount,
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

func (genTool *GeneratorTool) GenerateRandomTx(numTx int) types.Transactions {
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make(types.Transactions, numTx)
	genTool.mu.Lock()
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses(genTool.accounts)
		senderPublicKey := crypto.PubkeyToAddress(senderKey.PublicKey)
		amount := big.NewInt(int64(RandomInt(10, 20)))
		amount = amount.Mul(amount, big.NewInt(int64(math.Pow10(18))))
		senderAddrS := senderPublicKey.String()

		if _, ok := genTool.nonceMap[senderAddrS]; !ok {
			genTool.nonceMap[senderAddrS] = 1
		}
		nonce := genTool.nonceMap[senderAddrS]
		tx, err := types.SignTx(
			types.HomesteadSigner{},
			types.NewTransaction(
				nonce,
				toAddr,
				amount,
				DefaultGasLimit,
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

func (genTool *GeneratorTool) GenerateRandomTxWithState(numTx int, stateDb *state.StateDB) []*types.Transaction {
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make([]*types.Transaction, numTx)
	genTool.mu.Lock()
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses(genTool.accounts)
		senderPublicKey := crypto.PubkeyToAddress(senderKey.PublicKey)
		nonce := stateDb.GetNonce(senderPublicKey)
		amount := big.NewInt(int64(RandomInt(10, 20)))
		amount = amount.Mul(amount, big.NewInt(int64(math.Pow10(18))))
		senderAddrS := senderPublicKey.String()

		//get nonce from sender mapping
		nonceMap := genTool.GetNonce(senderAddrS)
		if nonce < nonceMap { // check nonce from statedb and nonceMap
			nonce = nonceMap
		}

		tx, err := types.SignTx(
			types.HomesteadSigner{},
			types.NewTransaction(
				nonce,
				toAddr,
				amount,
				DefaultGasLimit,
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

func (genTool *GeneratorTool) GenerateRandomTxWithAddressState(numTx int, txPool *tx_pool.TxPool) types.Transactions {
	if numTx <= 0 || len(genTool.accounts) == 0 {
		return nil
	}
	result := make(types.Transactions, numTx)
	genTool.mu.Lock()
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses(genTool.accounts)
		senderPublicKey := crypto.PubkeyToAddress(senderKey.PublicKey)
		nonce := txPool.Nonce(senderPublicKey)
		amount := big.NewInt(int64(RandomInt(10, 20)))
		amount = amount.Mul(amount, big.NewInt(int64(math.Pow10(18))))
		senderAddrS := senderPublicKey.String()

		//get nonce from sender mapping
		nonceMap := genTool.GetNonce(senderAddrS)
		if nonce < nonceMap { // check nonce from statedb and nonceMap
			nonce = nonceMap
		}

		tx, err := types.SignTx(
			types.HomesteadSigner{},
			types.NewTransaction(
				nonce,
				toAddr,
				amount,
				DefaultGasLimit,
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

func (genTool *GeneratorTool) GetNonce(address string) uint64 {
	return genTool.nonceMap[address]
}

// GenerateSmcCall generates tx which call a smart contract's method
// if isIncrement is true, nonce + 1 to prevent duplicate nonce if generateSmcCall is called twice.
func GenerateSmcCall(senderKey *ecdsa.PrivateKey, address common.Address, input []byte, txPool *tx_pool.TxPool, isIncrement bool) *types.Transaction {
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	nonce := txPool.Nonce(senderAddress)
	if isIncrement {
		nonce++
	}
	tx, err := types.SignTx(
		types.HomesteadSigner{},
		types.NewTransaction(
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
	log.Error("GenerateSmcCall", "nonce", tx.Nonce(), "tx", tx.Hash().Hex())
	return tx
}

func randomTxAddresses(accounts []Account) (senderKey *ecdsa.PrivateKey, toAddr common.Address) {
	for {
		senderKey = randomGenesisPrivateKey(accounts)
		toAddr = randomGenesisAddress()
		privateKeyBytes := crypto.FromECDSA(senderKey)
		privateKeyHex := common.Encode(privateKeyBytes)[2:]
		if senderKey != nil && crypto.PubkeyToAddress(senderKey.PublicKey) != toAddr && privateKeyHex != configs.KardiaPrivKeyToCallSmc && privateKeyHex != DefaultFaucetPrivAcc {
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

func randomAddress() common.Address {
	rand.Seed(time.Now().UTC().UnixNano())
	address := make([]byte, 20)
	rand.Read(address)
	return common.BytesToAddress(address)
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

func GetRandomGenesisAccount() common.Address {
	size := len(GenesisAccounts)
	randomI := rand.Intn(size)
	index := 0
	for addrS := range GenesisAccounts {
		if index == randomI {
			return common.HexToAddress(addrS)
		}
		index++
	}
	panic("impossible failure")
}

func GetAccounts(genesisAccounts map[string]string) []Account {
	accounts := make([]Account, 0)
	for key, value := range genesisAccounts {
		accounts = append(accounts, Account{
			Address:    key,
			PrivateKey: value,
		})
	}

	return accounts
}

func RandomInt(min int, max int) int {
	rand.Seed(time.Now().UnixNano())
	n := min + rand.Intn(max-min+1)
	return n
}
