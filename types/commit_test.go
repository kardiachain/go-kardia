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
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

func makeBlockIDRandom() BlockID {
	var (
		blockHash   = make([]byte, 32)
		partSetHash = make([]byte, 32)
	)
	rand.Read(blockHash)   //nolint: gosec
	rand.Read(partSetHash) //nolint: gosec
	return BlockID{common.BytesToHash(blockHash), PartSetHeader{123, common.BytesToHash(partSetHash)}}
}

func randCommit(now time.Time) *Commit {
	lastID := makeBlockIDRandom()
	h := int64(3)
	voteSet, _, vals := randVoteSet(uint64(h-1), 1, kproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(lastID, uint64(h-1), 1, voteSet, vals, now)
	if err != nil {
		panic(err)
	}
	return commit
}

func TestCommitValidateBasic(t *testing.T) {
	testCases := []struct {
		testName       string
		malleateCommit func(*Commit)
		expectErr      bool
	}{
		{"Random Commit", func(com *Commit) {}, false},
		{"Incorrect signature", func(com *Commit) { com.Signatures[0].Signature = []byte{0} }, false},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			com := randCommit(time.Now())
			tc.malleateCommit(com)
			assert.Equal(t, tc.expectErr, com.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestCommitCopy(t *testing.T) {
	commit := CreateNewCommit()
	commitCopy := commit.Copy()
	if commit.Hash() != commitCopy.Hash() {
		t.Fatal("Commit Copy Error")
	}
}

func TestCommitAccessorFunctions(t *testing.T) {
	commit := CreateNewCommit()
	assert.Equal(t, commit.Height, uint64(2))
	assert.Equal(t, commit.Round, uint32(1))
	assert.Equal(t, commit.Size(), 2)
	assert.Equal(t, commit.IsCommit(), true)
}

func TestCommitToBitArray(t *testing.T) {
	commit := CreateNewCommit()
	if commit.bitArray != nil {
		t.Error("Commit creation error")
	}
	bitArray := commit.BitArray()
	if commit.bitArray == nil || bitArray != commit.bitArray {
		t.Error("Commit bit Array error")
	}
}

func TestCommitGetByIndex(t *testing.T) {
	commit := CreateNewCommit()
	vote := commit.GetByIndex(0)
	assert.Equal(t, vote.Signature, commit.Signatures[0].Signature)
}

func CreateNewCommit() *Commit {
	block := CreateNewBlockWithTwoVotes(1)
	block.lastCommit.BlockID = CreateBlockIDRandom()
	return block.lastCommit
}

func CreateNewBlockWithTwoVotes(height uint64) *Block {
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
		BlockID:        BlockID{},
	}
	lastCommit := &Commit{
		Height:     2,
		Round:      1,
		Signatures: []CommitSig{vote.CommitSig(), NewCommitSigAbsent()},
	}
	return NewBlock(&header, txns, lastCommit, nil)
}
