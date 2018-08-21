package tool

import (
	development "github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/types"
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

func containsInGenesis(address string) bool {
	for k := range development.GenesisAccounts {
		if k == address {
			return true
		}
	}
	return false
}
