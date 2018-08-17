package tool

import (
	"testing"
)

func TestGenerateTx(t *testing.T) {
	result := GenerateRandomTx(0, nil)
	if len(result) != 10 {
		t.Error("default result len should be 10")
	}
}
