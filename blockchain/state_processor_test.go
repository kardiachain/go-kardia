package blockchain

import (
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"reflect"
	"testing"
)

func TestApplyTransactionsToAccountState(t *testing.T) {
	key1, _ := crypto.GenerateKey()
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	key2, _ := crypto.GenerateKey()
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	accounts := make(types.AccountStates, 2)
	accounts[0] = &types.BlockAccount{Addr: &addr1, Balance: big.NewInt(100)}
	accounts[1] = &types.BlockAccount{Addr: &addr2, Balance: big.NewInt(100)}

	emptyTx, _ := types.SignTx(types.NewTransaction(
		0,
		addr2,
		big.NewInt(10),
		0, big.NewInt(0),
		nil,
	), key1)
	txns := []*types.Transaction{emptyTx}

	// account1: 100 ; account2: 100
	newAccounts, err := ApplyTransactionsToAccountState(txns, accounts)
	if err != nil {
		t.Fatal("apply tx error: ", err)
	}
	// account1: 90 ; account2: 110

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Addr", accounts[0].Addr, (newAccounts)[0].Addr)
	check("Balance", accounts[0].Balance, big.NewInt(90))

	check("Addr", accounts[1].Addr, (newAccounts)[1].Addr)
	check("Balance", accounts[1].Balance, big.NewInt(110))
}
