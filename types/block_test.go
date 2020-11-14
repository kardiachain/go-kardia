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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

func CreateBlockIDRandom() BlockID {
	blockHash := common.BytesToHash(common.RandBytes(32))
	partSetHash := common.BytesToHash(common.RandBytes(32))
	blockPartsHeaders := PartSetHeader{Total: 123, Hash: partSetHash}
	return BlockID{
		Hash:        blockHash,
		PartsHeader: blockPartsHeaders,
	}
}

func TestBlockCreation(t *testing.T) {
	block := CreateNewBlock(1)
	if err := block.ValidateBasic(); err != nil {
		t.Fatal("Init block error", err)
	}
}

func TestBodyCreationAndCopy(t *testing.T) {
	body := CreateNewBlock(1).Body()
	copyBody := body.Copy()
	if rlpHash(body) != rlpHash(copyBody) {
		t.Fatal("Error copy body")
	}
}

func TestNewZeroBlockID(t *testing.T) {
	blockID := NewZeroBlockID()
	assert.Equal(t, blockID.IsZero(), true)
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
		Time:   time.Now(),
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
		ValidatorIndex: 1,
		Height:         2,
		Round:          1,
		Timestamp:      time.Now(),
		Type:           kproto.PrecommitType,
	}
	lastCommit := &Commit{
		Signatures: []CommitSig{vote.CommitSig(), {}},
	}
	evidence := []Evidence{}
	return NewBlock(&header, txns, lastCommit, evidence)
}
