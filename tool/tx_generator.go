package tool

import (
	"github.com/kardiachain/go-kardia/account"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
)

// GenerateRandomTx generate an array of random transactions with numberOfTx, senderAccount, receiver.
// numberOfTx (default: 10)
// senderAccount is instance of keyStore if  sender is Nil, it will use default sender to create tx.
// receiver is instance of common.Address, if address is empty, it will use default sender
func GenerateRandomTx(numberOfTx int, senderAccount *account.KeyStore, receiver common.Address) []types.Transaction {
	if numberOfTx <= 0 {
		numberOfTx = 10
	}
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	if senderAccount != nil {
		key = &senderAccount.PrivateKey
	}
	if receiver == common.BytesToAddress([]byte{}) {
		receiver = common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	}
	result := make([]types.Transaction, numberOfTx)
	for i := 0; i < numberOfTx; i++ {
		tx, _ := types.SignTx(types.NewTransaction(
			uint64(i+1),
			receiver,
			big.NewInt(10),
			22000,
			big.NewInt(10),
			nil,
		), key)
		result[i] = *tx
	}
	return result
}
