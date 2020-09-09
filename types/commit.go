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
	"errors"
	"fmt"
	"strings"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
)

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"block_id"`
	Precommits []*Vote `json:"precommits"`

	// Volatile
	firstPrecommit *Vote
	hash           cmn.Hash
	bitArray       *cmn.BitArray
}

// Construct a VoteSet from the Commit and validator set. Panics
// if precommits from the commit can't be added to the voteset.
// Inverse of VoteSet.MakeCommit().
func CommitToVoteSet(chainID string, commit *Commit, vals *ValidatorSet) *VoteSet {
	height, round, typ := commit.Height(), commit.Round(), VoteTypePrecommit
	voteSet := NewVoteSet(chainID, height, round, typ, vals)
	for _, precommit := range commit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := voteSet.AddVote(precommit)
		if !added || err != nil {
			panic(fmt.Sprintf("Failed to reconstruct LastCommit: %v", err))
		}
	}
	return voteSet
}

func (commit *Commit) Copy() *Commit {
	commitCopy := *commit
	if commit.firstPrecommit != nil {
		commitCopy.firstPrecommit = commit.firstPrecommit.Copy()
	}
	commitCopy.Precommits = make([]*Vote, len(commit.Precommits))
	for i := 0; i < len(commit.Precommits); i++ {
		if commit.Precommits[i] == nil {
			continue
		}
		commitCopy.Precommits[i] = commit.Precommits[i].Copy()
	}
	return &commitCopy
}

// FirstPrecommit returns the first non-nil precommit in the commit.
// If all precommits are nil, it returns an empty precommit with height 0.
func (commit *Commit) FirstPrecommit() *Vote {
	if len(commit.Precommits) == 0 {
		return nil
	}
	if commit.firstPrecommit != nil {
		return commit.firstPrecommit
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.firstPrecommit = precommit
			return precommit
		}
	}
	return &Vote{
		Type: VoteTypePrecommit,
	}
}

// Height returns the height of the commit
func (commit *Commit) Height() uint64 {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.Height()
}

// Round returns the round of the commit
func (commit *Commit) Round() uint32 {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.Round()
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

// Size returns the number of votes in the commit
func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Precommits)
}

// BitArray returns a BitArray of which validators voted in this commit
func (commit *Commit) BitArray() *cmn.BitArray {
	if commit.bitArray == nil {
		commit.bitArray = cmn.NewBitArray(len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			// TODO: need to check the BlockID otherwise we could be counting conflicts,
			// not just the one with +2/3 !
			commit.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return commit.bitArray
}

// GetByIndex returns the vote corresponding to a given validator index
func (commit *Commit) GetByIndex(index uint) *Vote {
	return commit.Precommits[index]
}

// IsCommit returns true if there is at least one vote
func (commit *Commit) IsCommit() bool {
	return len(commit.Precommits) != 0
}

// Hash returns the hash of the commit
func (commit *Commit) Hash() cmn.Hash {
	// TODO(namdoh): Cache hash so we don't have to re-hash all the time.
	return rlpHash(commit)
}

// ValidateBasic performs basic validation that doesn't involve state data.
func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
	if len(commit.Precommits) == 0 {
		return errors.New("No precommits in commit")
	}
	height, round := commit.Height(), commit.Round()

	// validate the precommits
	for _, precommit := range commit.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if uint64(precommit.Height) != height {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

// This function is used to address RLP's diosyncrasies (issues#73), enabling
// RLP encoding/decoding to pass.
// Note: Use this "before" sending the object to other peers.
func (commit *Commit) MakeNilEmpty() {
	for i := 0; i < len(commit.Precommits); i++ {
		if commit.Precommits[i] != nil {
			continue
		}
		commit.Precommits[i] = CreateEmptyVote()
	}
}

// This function is used to address RLP's diosyncrasies (issues#73), enabling
// RLP encoding/decoding to pass.
// Note: Use this "after" receiving the object to other peers.
func (commit *Commit) MakeEmptyNil() {
	for i := 0; i < len(commit.Precommits); i++ {
		if commit.Precommits[i] == nil {
			continue
		}
		if commit.Precommits[i].IsEmpty() {
			commit.Precommits[i] = nil
		}
	}
}

// StringLong returns a long string representing full info about Commit
func (commit *Commit) StringLong() string {
	if commit == nil {
		return "nil-Commit"
	}

	if len(commit.Precommits) == 0 {
		return "empty-Commit"
	}

	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	precommitStr := strings.Join(precommitStrings, "##")

	return fmt.Sprintf("Commit{BlockID:%v  Precommits:%v}#%v",
		commit.BlockID,
		precommitStr,
		commit.hash.Hex())
}

// String returns a short string representing commit by simplifying byte array to hex
func (commit *Commit) String() string {
	if commit == nil {
		return "nil-commit"
	}
	if len(commit.Precommits) == 0 {
		return "empty-Commit"
	}

	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	precommitStr := strings.Join(precommitStrings, "##")

	return fmt.Sprintf("Commit{BlockID:%v  Precommits:%v}#%v",
		commit.BlockID,
		precommitStr,
		commit.hash.Fingerprint())
}
