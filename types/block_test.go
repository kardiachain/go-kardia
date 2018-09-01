package types

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"math/big"
	"reflect"
	"testing"
	"time"
)

func TestBlockEncodeDecode(t *testing.T) {
	header := Header{}
	header.Height = 1
	header.Time = big.NewInt(time.Now().Unix())

	addr := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	key, _ := crypto.GenerateKey()

	emptyTx := NewTransaction(
		0,
		addr,
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)
	signedTx, _ := SignTx(emptyTx, key)

	txns := []*Transaction{signedTx}

	// TODO(thientn/namdoh): adds all details for a block here
	block := NewBlock(&header, txns, nil, &Commit{})

	// TODO: enable validate after adding data to field Commit.
	//if err := block.ValidateBasic(); err != nil {
	//	t.Fatal("Init block error", err)
	//}

	encodedBlock, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	var decodedBlock Block
	if err := rlp.DecodeBytes(encodedBlock, &decodedBlock); err != nil {
		t.Fatal("decode error: ", err)
	}

	//if err := decodedBlock.ValidateBasic(); err != nil {
	//		t.Fatal("Decoded block error", err)
	//}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("Time", block.Time(), decodedBlock.Time())
	check("Header", block.Header(), decodedBlock.Header())
	check("emptyTx", block.Transactions()[0].Hash(), decodedBlock.Transactions()[0].Hash())
	check("Commit", block.LastCommit().String(), decodedBlock.LastCommit().String())
}
