/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

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

	vote := &Vote{
		ValidatorIndex: common.NewBigInt(1),
		Height:         common.NewBigInt(2),
		Round:          common.NewBigInt(1),
		Timestamp:      big.NewInt(100),
		Type:           VoteTypePrecommit,
	}
	lastCommit := &Commit{
		Precommits: []*Vote{vote, nil},
	}

	// TODO: add more details to block.
	block := NewBlock(&header, txns, nil, lastCommit)

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

func TestBodyEncodeDecode(t *testing.T) {
	body := &Body{}

	addr := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	key, _ := crypto.GenerateKey()
	emptyTx := NewTransaction(
		0,
		addr,
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)
	signedTx, _ := SignTx(emptyTx, key)

	vote := &Vote{
		ValidatorIndex: common.NewBigInt(1),
		Height:         common.NewBigInt(2),
		Round:          common.NewBigInt(1),
		Timestamp:      big.NewInt(100),
		Type:           VoteTypePrecommit,
	}

	body.Transactions = []*Transaction{signedTx}
	body.LastCommit = &Commit{Precommits: []*Vote{vote, nil}}

	encodedBody, err := rlp.EncodeToBytes(body)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	var decodedBody Body
	if err := rlp.DecodeBytes(encodedBody, &decodedBody); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Txs", body.Transactions[0].Hash(), decodedBody.Transactions[0].Hash())
	check("Commit", body.LastCommit.Hash(), decodedBody.LastCommit.Hash())
}
