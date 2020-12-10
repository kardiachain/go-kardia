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

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/merkle"
	krand "github.com/kardiachain/go-kardia/lib/rand"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBlockID(hash common.Hash, partSetSize uint32, partSetHash common.Hash) BlockID {
	return BlockID{
		Hash: hash,
		PartsHeader: PartSetHeader{
			Total: partSetSize,
			Hash:  partSetHash,
		},
	}

}

func createHeaderRandom() *Header {
	randAddress := common.BytesToAddress(krand.Bytes(20))
	h := Header{
		Height:             krand.Uint64(),
		Time:               time.Now(),
		NumTxs:             krand.Uint64(),
		GasLimit:           krand.Uint64(),
		LastBlockID:        BlockID{},
		ProposerAddress:    randAddress,
		LastCommitHash:     krand.Hash(merkle.Size),
		TxHash:             krand.Hash(merkle.Size),
		ValidatorsHash:     krand.Hash(merkle.Size),
		NextValidatorsHash: krand.Hash(merkle.Size),
		ConsensusHash:      krand.Hash(merkle.Size),
		AppHash:            krand.Hash(merkle.Size),
		EvidenceHash:       krand.Hash(merkle.Size),
	}
	return &h
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
		Signatures: []CommitSig{vote.CommitSig(), CommitSig{}},
	}
	evidence := []Evidence{}
	return NewBlock(&header, txns, lastCommit, evidence)
}

func TestBlockValidateBasic(t *testing.T) {
	require.Error(t, (*Block)(nil).ValidateBasic())

	addr1 := common.BytesToAddress([]byte("0x01"))
	txs := []*Transaction{
		NewTransaction(1, addr1, big.NewInt(1), 1, big.NewInt(1), []byte("tx")),
	}

	lastID := makeBlockIDRandom()
	h := uint64(3)

	voteSet, valSet, vals := randVoteSet(h-1, 1, kproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	testCases := []struct {
		testName      string
		malleateBlock func(*Block)
		expErr        bool
	}{
		{"Make Block", func(blk *Block) {}, false},
		{"Make Block w/ proposer Addr", func(blk *Block) { blk.header.ProposerAddress = valSet.GetProposer().Address }, false},
		{"Remove 1/2 the commits", func(blk *Block) {
			blk.lastCommit.Signatures = commit.Signatures[:commit.Size()/2]
			blk.lastCommit.hash = common.Hash{}
		}, true},
		{"Remove LastCommitHash", func(blk *Block) { blk.header.LastCommitHash = common.BytesToHash([]byte("something else")) }, true},
		{"Tampered Data", func(blk *Block) {
			blk.transactions[0] = NewTransaction(1, addr1, big.NewInt(1), 1, big.NewInt(1), []byte("something else"))
		}, true},
		{"Tampered DataHash", func(blk *Block) {
			blk.header.TxHash = common.BytesToHash([]byte("txhash"))
		}, true},
		{"Tampered EvidenceHash", func(blk *Block) {
			blk.header.EvidenceHash = common.BytesToHash([]byte("EvidenceHash"))
		}, true},
		{"Missing LastCommit", func(blk *Block) {
			blk.header.LastCommitHash = common.Hash{}
		}, true},
		{"Invalid LastCommit", func(blk *Block) {
			blk.lastCommit = NewCommit(0, 0, *voteSet.maj23, nil)
		}, true},
		{"Invalid Evidence", func(blk *Block) {
			emptyEv := &DuplicateVoteEvidence{}
			blk.evidence = &EvidenceData{Evidence: []Evidence{emptyEv}}
		}, true},
	}

	ev := NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []Evidence{ev}

	for i, tc := range testCases {
		tc := tc
		i := i
		t.Run(tc.testName, func(t *testing.T) {
			block := NewBlock(&Header{Height: h}, txs, commit, evList)
			block.header.ProposerAddress = valSet.GetProposer().Address
			tc.malleateBlock(block)
			err := block.ValidateBasic()
			t.Log(err)
			assert.Equal(t, tc.expErr, err != nil, "#%d: %v", i, err)
		})
	}
}

func TestBlockHash(t *testing.T) {
	assert.Equal(t, common.Hash{}, (*Block)(nil).Hash())
	//assert.Equal(t, common.Hash{}, NewBlock(&Header{}, []*Transaction{}, nil, nil).Hash())
}

func TestBlockMakePartSet(t *testing.T) {
	assert.Nil(t, (*Block)(nil).MakePartSet(2))

	partSet := NewBlock(&Header{Height: 3}, []*Transaction{}, nil, nil).MakePartSet(1024)
	assert.NotNil(t, partSet)
	assert.EqualValues(t, 1, partSet.Total())
}

func TestBlockMakePartSetWithEvidence(t *testing.T) {
	assert.Nil(t, (*Block)(nil).MakePartSet(2))

	lastID := makeBlockIDRandom()
	h := uint64(3)

	voteSet, _, vals := randVoteSet(h-1, 1, kproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []Evidence{ev}

	partSet := NewBlock(&Header{Height: 3}, []*Transaction{}, commit, evList).MakePartSet(512)
	assert.NotNil(t, partSet)
	assert.EqualValues(t, 4, partSet.Total())
}

func TestBlockHashesTo(t *testing.T) {
	assert.False(t, (*Block)(nil).HashesTo(common.Hash{}))

	lastID := makeBlockIDRandom()
	h := uint64(3)
	voteSet, valSet, vals := randVoteSet(h-1, 1, kproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []Evidence{ev}

	block := NewBlock(&Header{Height: 3}, []*Transaction{}, commit, evList)
	block.header.ValidatorsHash = valSet.Hash()
	assert.False(t, block.HashesTo(common.Hash{}))
	assert.False(t, block.HashesTo(common.BytesToHash([]byte("something else"))))
	assert.True(t, block.HashesTo(block.Hash()))
}

func TestBlockSize(t *testing.T) {
	size := NewBlock(&Header{Height: 3}, []*Transaction{}, nil, nil).Size()
	if size <= 0 {
		t.Fatal("Size of the block is zero or negative")
	}
}
