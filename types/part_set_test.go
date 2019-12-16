package types

import (
	"testing"

	"github.com/kardiachain/go-kardia/lib/rlp"
)

func TestEncodeDecodePart(t *testing.T) {
	block := CreateNewBlock(1)
	partsSet := block.MakePartSet(BlockPartSizeBytes)

	partBytes, err := rlp.EncodeToBytes(partsSet.GetPart(0))

	if err != nil {
		t.Fatal(err)
	}

	decoded := &Part{}

	if err := rlp.DecodeBytes(partBytes, decoded); err != nil {
		t.Fatal(err)
	}

}
