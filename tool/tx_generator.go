package tool

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"math/rand"
	"time"
	"github.com/kardiachain/go-kardia/state"
)

const (
	defaultNumTx    = 10
	defaultGasLimit = 10 // currently we don't care about tx fee and cost.
)

var (
	defaultAmount   = big.NewInt(10)
	defaultGasPrice = big.NewInt(10)
)

// GenerateRandomTx generate an array of random transfer transactions within genesis accounts.
// numTx: number of transactions to send, default to 10.
// senderAcc: instance of keyStore if  sender is Nil, it will get random from genesis account.
// receiverAddr: instance of common.Address, if address is empty, it will random address.
func GenerateRandomTx(numTx int) []*types.Transaction {
	if numTx <= 0 {
		numTx = defaultNumTx
	}
	result := make([]*types.Transaction, numTx)
	for i := 0; i < numTx; i++ {
		senderKey, toAddr := randomTxAddresses()

		tx, err := types.SignTx(types.NewTransaction(
			0, // TODO: need to set valid nonce after improving tx handling to handling nonce.
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
	}
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

func GenerateSmcCall(senderKey *ecdsa.PrivateKey, address common.Address, input []byte, stateDb *state.StateDB) *types.Transaction {
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	nonce := stateDb.GetNonce(senderAddress)
	tx, err := types.SignTx(types.NewTransaction(
		nonce,
		address,
		defaultAmount,
		22000,
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
	size := len(dev.GenesisAddrKeys)
	randomI := rand.Intn(size)
	index := 0
	for addrS := range dev.GenesisAddrKeys {
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
	size := len(dev.GenesisAddrKeys)
	randomI := rand.Intn(size)
	index := 0
	for _, privateKey := range dev.GenesisAddrKeys {
		if index == randomI {
			pkByte, _ := hex.DecodeString(privateKey)
			return crypto.ToECDSAUnsafe(pkByte)
		}
		index++
	}
	panic("impossible failure")
}
