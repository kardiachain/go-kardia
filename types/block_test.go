package types

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"math/big"
	"reflect"
	"testing"
)

func TestBlockEncodeDecode(t *testing.T) {
	header := Header{}
	addr := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	addr2 := common.HexToAddress("0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359")

	emptyTx := NewTransaction(
		0,
		addr,
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)
	txns := []*Transaction{emptyTx}
	accounts := make(AccountStates, 2)
	accounts[0] = &BlockAccount{Addr: &addr, Balance: big.NewInt(100)}
	accounts[1] = &BlockAccount{Addr: &addr2, Balance: big.NewInt(100)}

	// TODO(thientn/namdoh): adds all details for a block here
	block := NewBlock(&header, txns, nil, &Commit{}, &accounts)

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

	check("Header", block.Header(), decodedBlock.Header())
	check("emptyTx", block.Transactions()[0].Hash(), decodedBlock.Transactions()[0].Hash())
	check("Commit", block.LastCommit().String(), decodedBlock.LastCommit().String())
	check("Account 0", accounts[0], decodedBlock.Accounts().GetAccount(&addr))
	check("Account 1", accounts[1], decodedBlock.Accounts().GetAccount(&addr2))
}
