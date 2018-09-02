package tool

import (
	development "github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/state"
	kaidb "github.com/kardiachain/go-kardia/storage"
	"testing"
)

func TestGenerateTx(t *testing.T) {
	result := GenerateRandomTx(1000)
	for _, tx := range result {
		from, _ := types.Sender(tx)
		if !containsInGenesis(from.String()) {
			t.Error("Sender addr should be in genesis block")
		}
		if !containsInGenesis(tx.To().String()) {
			t.Error("Receiver addr should be in genesis")
		}
		if from == *tx.To() {
			t.Error("Sender & receiver addrs should not be the same")
		}
	}
}

func TestGenerateRandomTxWithState(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	result := GenerateRandomTxWithState(10, state)
	for _, tx := range result {
		from, _ := types.Sender(tx)
		if !containsInGenesis(from.String()) {
			t.Error("Sender addr should be in genesis block")
		}
		if !containsInGenesis(tx.To().String()) {
			t.Error("Receiver addr should be in genesis")
		}
		if from == *tx.To() {
			t.Error("Sender & receiver addrs should not be the same")
		}
		if tx.Nonce() != state.GetNonce(from) + 1 {
			t.Error("Nonce should be bigger than currrent nonce by 1")
		}
	}
}


func containsInGenesis(address string) bool {
	for k := range development.GenesisAccounts {
		if k == address {
			return true
		}
	}
	return false
}
