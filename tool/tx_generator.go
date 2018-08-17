package tool

import (
	"github.com/kardiachain/go-kardia/account"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"math/rand"
)

const (
	defaultNumTx      = 10
	defaultReceiver   = "095e7baea6a6c7c4c2dfeb977efac326af552d87"
	defaultPrivateKey = "45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
	defaultGas        = 22000 // currently we don't care about tx fee and cost so just add it's a prefer number (ex: tx fee is 21000 wei to send eth)
)

// GenerateRandomTx generate an array of random transactions with numTx, senderAcc, receiver.
// numTx: number of transactions to send, default to 10
// senderAcc: is instance of keyStore if  sender is Nil, it will use default sender to create tx.
// receiver: is instance of common.Address, if address is empty, it will use default sender
func GenerateRandomTx(numTx int, senderAcc *account.KeyStore, receiver common.Address) []types.Transaction {
	if numTx <= 0 {
		numTx = defaultNumTx
	}
	key, _ := crypto.HexToECDSA(defaultPrivateKey)
	if senderAcc != nil {
		key = &senderAcc.PrivateKey
	}

	result := make([]types.Transaction, numTx)
	for i := 0; i < numTx; i++ {
		var to common.Address
		if receiver == common.BytesToAddress([]byte{}) { // empty address
			to = randomAddress()
		} else {
			to = receiver
		}
		tx, _ := types.SignTx(types.NewTransaction(
			uint64(i+1),
			to,
			big.NewInt(10),
			defaultGas,
			big.NewInt(10),
			nil,
		), key)
		result[i] = *tx
	}
	return result
}

func randomAddress() common.Address {
	address := make([]byte, 20)
	rand.Read(address)
	return common.BytesToAddress(address)
}
