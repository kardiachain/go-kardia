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
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

func makeBlockIDRandom() BlockID {
	blockHash := common.BytesToHash(common.RandBytes(32))
	partSetHash := common.BytesToHash(common.RandBytes(32))
	blockPartsHeaders := PartSetHeader{Total: *common.NewBigInt32(123), Hash: partSetHash}
	return BlockID{
		Hash:        blockHash,
		PartsHeader: blockPartsHeaders,
	}
}

func makeBlockID(hash common.Hash, partSetSize common.BigInt, partSetHash common.Hash) BlockID {
	return BlockID{
		Hash: hash,
		PartsHeader: PartSetHeader{
			Total: partSetSize,
			Hash:  partSetHash,
		},
	}

}

func TestBlockCreation(t *testing.T) {
	block := CreateNewBlock(1)
	if err := block.ValidateBasic(); err != nil {
		t.Fatal("Init block error", err)
	}
}

func TestBlockEncodeDecode(t *testing.T) {
	block := CreateNewBlock(1)
	encodedBlock, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	var decodedBlock Block
	if err := rlp.DecodeBytes(encodedBlock, &decodedBlock); err != nil {
		t.Fatal("decode error: ", err)
	}

	if decodedBlock.Hash() != block.Hash() {
		t.Error("Encode Decode block error")
	}
}

func TestNewDualBlock(t *testing.T) {
	block := CreateNewDualBlock()
	if err := block.ValidateBasic(); err != nil {
		t.Fatal("Error validating New Dual block", err)
	}
}

func TestBlockEncodeDecodeFile(t *testing.T) {
	block := CreateNewBlock(1)
	blockCopy := block.WithBody(block.Body())
	encodeFile, err := os.Create("encodeFile.txt")
	defer encodeFile.Close()
	if err != nil {
		t.Error("Error creating file")
	}

	if err := block.EncodeRLP(encodeFile); err != nil {
		t.Fatal("Error encoding block")
	}

	f, err := os.Open("encodeFile.txt")
	if err != nil {
		t.Error("Error opening file:", err)
	}

	stream := rlp.NewStream(f, 99999)
	if err := block.DecodeRLP(stream); err != nil {
		t.Fatal("Decoding block error:", err)
	}
	if block.Hash() != blockCopy.Hash() {
		t.Fatal("Encode Decode File error")
	}

}

func TestGetDualEvents(t *testing.T) {
	dualBlock := CreateNewDualBlock()
	dualEvents := dualBlock.DualEvents()
	dualEventCopy := NewDualEvent(100, false, "KAI", new(common.Hash), new(EventSummary), &DualAction{Name: "dualTest"})
	if dualEvents[0].Hash() != dualEventCopy.Hash() {
		t.Error("Dual Events hash not equal")
	}
}

func TestBodyCreationAndCopy(t *testing.T) {
	body := CreateNewBlock(1).Body()
	copyBody := body.Copy()
	if rlpHash(body) != rlpHash(copyBody) {
		t.Fatal("Error copy body")
	}
}

func TestBodyEncodeDecodeFile(t *testing.T) {
	body := CreateNewBlock(1).Body()
	bodyCopy := body.Copy()
	encodeFile, err := os.Create("encodeFile.txt")
	if err != nil {
		t.Error("Error creating file")
	}

	if err := body.EncodeRLP(encodeFile); err != nil {
		t.Fatal("Error encoding block")
	}

	encodeFile.Close()

	f, err := os.Open("encodeFile.txt")
	if err != nil {
		t.Error("Error opening file:", err)
	}

	stream := rlp.NewStream(f, 99999)
	if err := body.DecodeRLP(stream); err != nil {
		t.Fatal("Decoding block error:", err)
	}
	defer f.Close()

	if rlpHash(body) != rlpHash(bodyCopy) {
		t.Fatal("Encode Decode from file error")
	}
}

func TestBlockWithBodyFunction(t *testing.T) {
	block := CreateNewBlock(1)
	body := CreateNewDualBlock().Body()

	blockWithBody := block.WithBody(body)
	bwbBody := blockWithBody.Body()
	if blockWithBody.header.Hash() != block.header.Hash() {
		t.Error("BWB Header Error")
	}
	for i := range bwbBody.Transactions {
		if bwbBody.Transactions[i] != body.Transactions[i] {
			t.Error("BWB Transaction Error")
			break
		}
	}
	for i := range bwbBody.DualEvents {
		if bwbBody.DualEvents[i] != body.DualEvents[i] {
			t.Error("BWB Dual Events Error")
			break
		}
	}
	if bwbBody.LastCommit != body.LastCommit {
		t.Error("BWB Last Commit Error")
	}
}

func TestNewZeroBlockID(t *testing.T) {
	blockID := NewZeroBlockID()
	if !blockID.IsZero() {
		t.Fatal("NewZeroBlockID is not empty")
	}
}

func TestBlockSorterSwap(t *testing.T) {
	firstBlock := CreateNewBlock(1)
	secondBlock := CreateNewBlock(3)
	blockSorter := blockSorter{
		blocks: []*Block{firstBlock, secondBlock},
	}
	blockSorter.Swap(0, 1)
	if blockSorter.blocks[0] != secondBlock && blockSorter.blocks[1] != firstBlock {
		t.Fatal("blockSorter Swap error")
	}
}

func TestBlockHeightFunction(t *testing.T) {
	lowerBlock := CreateNewBlock(1)
	higherBlock := CreateNewBlock(2)
	if Height(higherBlock, lowerBlock) {
		t.Fatal("block Height func error")
	} else if !Height(lowerBlock, higherBlock) {
		t.Fatal("Block Height func error")
	}
}

func TestBlockSortByHeight(t *testing.T) {
	GetBlockByHeight := BlockBy(
		func(b1, b2 *Block) bool {
			return b1.header.Height < b2.header.Height
		})
	b0 := CreateNewBlock(0)
	b1 := CreateNewBlock(1)
	b2 := CreateNewBlock(2)
	b3 := CreateNewBlock(3)
	blocks := []*Block{b3, b2, b1, b0}

	GetBlockByHeight.Sort(blocks)
	if !CheckSortedHeight(blocks) {
		t.Error("Blocks not sorted")
	}
}

func CheckSortedHeight(blocks []*Block) bool {
	prev := blocks[0].header.Height
	for i := range blocks {
		if prev > blocks[i].header.Height {
			return false
		}
		prev = blocks[i].header.Height
	}
	return true
}

func CreateNewBlock(height uint64) *Block {
	header := Header{
		Height: height,
		Time:   big.NewInt(time.Now().Unix()),
	}

	addr := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	key, _ := crypto.GenerateKey()
	emptyTx := NewTransaction(
		1,
		addr,
		big.NewInt(99), 1000, big.NewInt(100),
		nil,
	)
	signedTx, _ := SignTx(HomesteadSigner{}, emptyTx, key)

	txns := []*Transaction{signedTx}

	vote := &Vote{
		ValidatorIndex: common.NewBigInt64(1),
		Height:         common.NewBigInt64(2),
		Round:          common.NewBigInt64(1),
		Timestamp:      big.NewInt(100),
		Type:           VoteTypePrecommit,
	}
	lastCommit := &Commit{
		Precommits: []*Vote{vote, nil},
	}
	return NewBlock(&header, txns, nil, lastCommit)
}

func CreateNewDualBlock() *Block {
	header := Header{
		Height: 1,
		Time:   big.NewInt(1),
	}
	vote := &Vote{
		ValidatorIndex: common.NewBigInt64(1),
		Height:         common.NewBigInt64(2),
		Round:          common.NewBigInt64(1),
		Timestamp:      big.NewInt(100),
		Type:           VoteTypePrecommit,
	}
	lastCommit := &Commit{
		Precommits: []*Vote{vote, vote},
	}
	header.LastCommitHash = lastCommit.Hash()
	de := NewDualEvent(100, false, "KAI", new(common.Hash), new(EventSummary), &DualAction{Name: "dualTest"})
	return NewDualBlock(&header, []*DualEvent{de, nil}, lastCommit)
}
