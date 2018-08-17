package tool

import (
	"crypto/ecdsa"
	"encoding/hex"
	"github.com/kardiachain/go-kardia/account"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"math/rand"
	"time"
)

const (
	defaultNumTx = 10
	defaultGas   = 22000 // currently we don't care about tx fee and cost so just add it's a prefer number (ex: tx fee is 21000 wei to send eth)
)

var privKeys = []string{
	"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
	"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
	"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
	"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
	"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
	"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
	"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
	"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
	"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
	"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d",
}

// GenerateRandomTx generate an array of random transactions with numTx, senderAcc, receiver.
// numTx: number of transactions to send, default to 10.
// senderAcc: instance of keyStore if  sender is Nil, it will get random from genesis account.
// receiverAddr: instance of common.Address, if address is empty, it will random address.
func GenerateRandomTx(numTx int, senderAcc *account.KeyStore, receiverAddr common.Address) []types.Transaction {
	if numTx <= 0 {
		numTx = defaultNumTx
	}
	result := make([]types.Transaction, numTx)
	for i := 0; i < numTx; i++ {
		var to common.Address
		var key *ecdsa.PrivateKey
		if senderAcc != nil {
			key = &senderAcc.PrivateKey
		} else {
			key = randomSenderPrivateKey()
		}
		if receiverAddr == common.BytesToAddress([]byte{}) { // empty address
			to = randomAddress()
		} else {
			to = receiverAddr
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
	rand.Seed(time.Now().UTC().UnixNano())
	address := make([]byte, 20)
	rand.Read(address)
	return common.BytesToAddress(address)
}

func randomSenderPrivateKey() *ecdsa.PrivateKey {
	size := len(privKeys)
	index := rand.Intn(size)
	privKey := privKeys[index]
	pkByte, _ := hex.DecodeString(privKey)
	return crypto.ToECDSAUnsafe(pkByte)
}
