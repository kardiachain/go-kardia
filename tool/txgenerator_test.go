package tool

import (
	development "github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
	"testing"
)

func TestGenerateTx(t *testing.T) {
	result := GenerateRandomTx(0, nil, common.BytesToAddress([]byte{}))
	if len(result) != 10 {
		t.Error("default result len should be 10")
	}
	for _, tx := range result {
		from, _ := types.Sender(&tx)
		if containsInGenesis(from.String()) == false {
			t.Error("default sender should be in genesis block")
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
