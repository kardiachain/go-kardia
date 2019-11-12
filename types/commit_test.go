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
	"github.com/kardiachain/go-kardia/lib/log"
)

func TestCommitCreation(t *testing.T) {
	commit := CreateNewCommit()

	if err := commit.ValidateBasic(); err != nil {
		t.Fatal("Commit validate basic error", err)
	}
}

func TestCommitCopy(t *testing.T) {
	commit := CreateNewCommit()
	commitCopy := commit.Copy()
	if commit.Hash() != commitCopy.Hash() {
		t.Fatal("Commit Copy Error")
	}
}

func TestCommitGetFirstPrecommit(t *testing.T) {
	commit := CreateNewCommit()
	if commit.firstPrecommit != nil {
		t.Error("Commit Creation error")
	}
	firstPrecommit := commit.FirstPrecommit()
	if rlpHash(firstPrecommit) != rlpHash(commit.firstPrecommit) {
		t.Error("First Precommit Error")
	}
}

func TestCommitAccessorFunctions(t *testing.T) {
	commit := CreateNewCommit()
	if !commit.Height().Equals(common.NewBigInt64(2)) {
		t.Error("Height")
	}
	if !commit.Round().Equals(common.NewBigInt64(1)) {
		t.Error("Round")
	}
	if commit.Size() != 2 {
		t.Error("Size")
	}
	if !commit.IsCommit() {
		t.Error("IsCommit")
	}
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
	precommit := commit.GetByIndex(0)
	if rlpHash(precommit) != rlpHash(commit.Precommits[0]) {
		t.Error()
	}
}

func CreateNewCommit() *Commit {
	block := CreateNewBlockWithTwoVotes(1)
	block.lastCommit.BlockID = block.BlockID()
	return block.lastCommit
}

func CreateNewBlockWithTwoVotes(height uint64) *Block {
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
	return NewBlock(log.New(), &header, txns, nil, lastCommit)
}
