package tool

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"testing"
)

func TestGenerateTx(t *testing.T) {
	result := GenerateRandomTx(0, nil, common.BytesToAddress([]byte{}))
	if len(result) != 10 {
		t.Error("default result len should be 10")
	}
}
