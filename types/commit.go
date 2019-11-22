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
	"io"
	"strings"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

//-------------------------------------

// CommitSig is a vote included in a Commit.
// For now, it is identical to a vote,
// but in the future it will contain fewer fields
// to eliminate the redundancy in commits.
type CommitSig Vote

// String returns the underlying Vote.String()
func (cs *CommitSig) String() string {
	return cs.toVote().String()
}

// toVote converts the CommitSig to a vote.
// TODO: deprecate for #1648. Converting to Vote will require
// access to ValidatorSet.
func (cs *CommitSig) toVote() *Vote {
	if cs == nil {
		return nil
	}
	v := Vote(*cs)
	return &v
}

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID      `json:"block_id"`
	Precommits []*CommitSig `json:"precommits"`

	// memoized in first call to corresponding method
	// NOTE: can't memoize in constructor because constructor
	// isn't used for unmarshaling
	height   *cmn.BigInt
	round    *cmn.BigInt
	hash     cmn.Hash
	bitArray *cmn.BitArray
}

// NewCommit returns a new Commit with the given blockID and precommits.
// TODO: memoize ValidatorSet in constructor so votes can be easily reconstructed
// from CommitSig after #1648.
func NewCommit(blockID BlockID, precommits []*CommitSig) *Commit {
	return &Commit{
		BlockID:    blockID,
		Precommits: precommits,
		height:     cmn.NewBigInt64(0),
		round:      cmn.NewBigInt64(0),
	}
}

// GetVote converts the CommitSig for the given valIdx to a Vote.
// Returns nil if the precommit at valIdx is nil.
// Panics if valIdx >= commit.Size().
func (commit *Commit) GetVote(valIdx int) *Vote {
	commitSig := commit.Precommits[valIdx]
	if commitSig == nil {
		return nil
	}

	// NOTE: this commitSig might be for a nil blockID,
	// so we can't just use commit.BlockID here.
	// For #1648, CommitSig will need to indicate what BlockID it's for !
	blockID := commitSig.BlockID
	commit.memoizeHeightRound()
	return &Vote{
		Type:             PrecommitType,
		Height:           commit.height,
		Round:            commit.round,
		BlockID:          blockID,
		Timestamp:        commitSig.Timestamp,
		ValidatorAddress: commitSig.ValidatorAddress,
		ValidatorIndex:   cmn.NewBigInt32(valIdx),
		Signature:        commitSig.Signature,
	}
}

// memoizeHeightRound memoizes the height and round of the commit using
// the first non-nil vote.
// Should be called before any attempt to access `commit.height` or `commit.round`.
func (commit *Commit) memoizeHeightRound() {
	if len(commit.Precommits) == 0 {
		return
	}
	if commit.height.IsLessThanInt(0) {
		return
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.height = precommit.Height
			commit.round = precommit.Round
		}
	}
}

// Construct a VoteSet from the Commit and validator set. Panics
// if precommits from the commit can't be added to the voteset.
// Inverse of VoteSet.MakeCommit().
func CommitToVoteSet(chainID string, commit *Commit, vals *ValidatorSet) *VoteSet {
	height, round, t := commit.Height(), commit.Round(), PrecommitType
	voteSet := NewVoteSet(chainID, height, round, t, vals)
	for idx, precommit := range commit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := voteSet.AddVote(commit.GetVote(idx))
		if !added || err != nil {
			panic(fmt.Sprintf("Failed to reconstruct LastCommit: %v", err))
		}
	}
	return voteSet
}

// Height returns the height of the commit
func (commit *Commit) Height() *cmn.BigInt {
	commit.memoizeHeightRound()
	return commit.height
}

// Round returns the round of the commit
func (commit *Commit) Round() *cmn.BigInt {
	commit.memoizeHeightRound()
	return commit.round
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
func (commit *Commit) Type() byte {
	return byte(PrecommitType)
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
func (commit *Commit) GetByIndex(valIdx int) *Vote {
	return commit.GetVote(valIdx)
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
		if precommit.Type != PrecommitType {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if !precommit.Height.Equals(height) {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if !precommit.Round.Equals(round) {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
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

func (commit *Commit) DecodeRLP(s *rlp.Stream) error {
	// Retrieve the entire receipt blob as we need to try multiple decoders
	blob, err := s.Raw()
	if err != nil {
		return err
	}

	var stored commitRLP
	if err := rlp.DecodeBytes(blob, &stored); err != nil {
		return err
	}
	commit.BlockID = stored.BlockID
	commit.Precommits = make([]*CommitSig, len(stored.Precommits))
	commit.height = cmn.NewBigInt64(0)
	commit.round = cmn.NewBigInt64(0)

	for idx, precommit := range stored.Precommits {
		if precommit.toVote().IsEmpty() {
			commit.Precommits[idx] = nil
		} else {
			commit.Precommits[idx] = precommit
		}
	}
	return nil
}

func (commit *Commit) EncodeRLP(w io.Writer) error {
	enc := &commitRLP{
		BlockID:    commit.BlockID,
		Precommits: make([]*CommitSig, len(commit.Precommits)),
	}

	for idx, precommit := range commit.Precommits {
		if precommit == nil {
			enc.Precommits[idx] = CreateEmptyVote().CommitSig()
		} else {
			enc.Precommits[idx] = precommit
		}
	}
	return rlp.Encode(w, enc)
}

type commitRLP struct {
	BlockID    BlockID
	Precommits []*CommitSig
}
