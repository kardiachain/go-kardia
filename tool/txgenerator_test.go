package tool

import (
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
		to := tx.To()
		if from.String() != "0xa94f5374Fce5edBC8E2a8697C15331677e6EbF0B" {
			t.Error("default sender should be 0xa94f5374Fce5edBC8E2a8697C15331677e6EbF0B")
		}
		if to.String() != "0x095E7BAea6a6c7c4c2DfeB977eFac326aF552d87" {
			t.Error("default receiver should be 0x095E7BAea6a6c7c4c2DfeB977eFac326aF552d87")
		}
	}
}
