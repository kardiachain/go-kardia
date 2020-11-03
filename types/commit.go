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
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

//-------------------------------------

// BlockIDFlag indicates which BlockID the signature is for.
type BlockIDFlag byte

const (
	// BlockIDFlagAbsent - no vote was received from a validator.
	BlockIDFlagAbsent BlockIDFlag = iota + 1
	// BlockIDFlagCommit - voted for the Commit.BlockID.
	BlockIDFlagCommit
	// BlockIDFlagNil - voted for nil.
	BlockIDFlagNil
)

// CommitSig is a part of the Vote included in a Commit.
type CommitSig struct {
	BlockIDFlag      BlockIDFlag    `json:"block_id_flag"`
	ValidatorAddress common.Address `json:"validator_address"`
	Timestamp        time.Time      `json:"timestamp"`
	Signature        []byte         `json:"signature"`
}

// NewCommitSigForBlock returns new CommitSig with BlockIDFlagCommit.
func NewCommitSigForBlock(signature []byte, valAddr common.Address, ts time.Time) CommitSig {
	return CommitSig{
		BlockIDFlag:      BlockIDFlagCommit,
		ValidatorAddress: valAddr,
		Timestamp:        ts,
		Signature:        signature,
	}
}

// ForBlock returns true if CommitSig is for the block.
func (cs CommitSig) ForBlock() bool {
	return cs.BlockIDFlag == BlockIDFlagCommit
}

// NewCommitSigAbsent returns new CommitSig with BlockIDFlagAbsent. Other
// fields are all empty.
func NewCommitSigAbsent() CommitSig {
	return CommitSig{
		BlockIDFlag: BlockIDFlagAbsent,
	}
}

// Absent returns true if CommitSig is absent.
func (cs CommitSig) Absent() bool {
	return cs.BlockIDFlag == BlockIDFlagAbsent
}

func (cs CommitSig) String() string {
	return fmt.Sprintf("CommitSig{%X by %X on %v @ %x}",
		common.Fingerprint(cs.Signature),
		common.Fingerprint(cs.ValidatorAddress.Bytes()),
		cs.BlockIDFlag,
		cs.Timestamp,
	)
}

// BlockID returns the Commit's BlockID if CommitSig indicates signing,
// otherwise - empty BlockID.
func (cs CommitSig) BlockID(commitBlockID BlockID) BlockID {
	var blockID BlockID
	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
		blockID = BlockID{}
	case BlockIDFlagCommit:
		blockID = commitBlockID
	case BlockIDFlagNil:
		blockID = BlockID{}
	default:
		panic(fmt.Sprintf("Unknown BlockIDFlag: %v", cs.BlockIDFlag))
	}
	return blockID
}

// ValidateBasic performs basic validation.
func (cs CommitSig) ValidateBasic() error {
	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
	case BlockIDFlagCommit:
	case BlockIDFlagNil:
	default:
		return fmt.Errorf("unknown BlockIDFlag: %v", cs.BlockIDFlag)
	}

	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
		if !cs.ValidatorAddress.Equal(common.Address{}) {
			return errors.New("validator address is present")
		}
		if !cs.Timestamp.IsZero() {
			return errors.New("time is present")
		}
		if len(cs.Signature) != 0 {
			return errors.New("signature is present")
		}
	default:
		// NOTE: Timestamp validation is subtle and handled elsewhere.
		if len(cs.Signature) == 0 {
			return errors.New("signature is missing")
		}
	}

	return nil
}

// ToProto converts CommitSig to protobuf
func (cs *CommitSig) ToProto() *kproto.CommitSig {
	if cs == nil {
		return nil
	}

	return &kproto.CommitSig{
		BlockIdFlag:      kproto.BlockIDFlag(cs.BlockIDFlag),
		ValidatorAddress: cs.ValidatorAddress.Bytes(),
		Timestamp:        cs.Timestamp,
		Signature:        cs.Signature,
	}
}

// FromProto sets a protobuf CommitSig to the given pointer.
// It returns an error if the CommitSig is invalid.
func (cs *CommitSig) FromProto(csp kproto.CommitSig) error {

	cs.BlockIDFlag = BlockIDFlag(csp.BlockIdFlag)
	cs.ValidatorAddress = common.BytesToAddress(csp.ValidatorAddress)
	cs.Timestamp = csp.Timestamp
	cs.Signature = csp.Signature

	return cs.ValidateBasic()
}

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID     `json:"block_id"`
	Signatures []CommitSig `json:"signatures"`
	Height     uint64      `json:"height"`
	Round      uint32      `json:"round"`

	// Volatile
	hash     cmn.Hash
	bitArray *cmn.BitArray
}

// NewCommit returns a new Commit.
func NewCommit(height uint64, round uint32, blockID BlockID, commitSigs []CommitSig) *Commit {
	return &Commit{
		Height:     height,
		Round:      round,
		BlockID:    blockID,
		Signatures: commitSigs,
	}
}

// CommitToVoteSet constructs a VoteSet from the Commit and validator set.
// Panics if signatures from the commit can't be added to the voteset.
// Inverse of VoteSet.MakeCommit().
func CommitToVoteSet(chainID string, commit *Commit, vals *ValidatorSet) *VoteSet {
	height, round := commit.GetHeight(), commit.GetRound()
	voteSet := NewVoteSet(chainID, height, round, kproto.PrecommitType, vals)
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue // OK, some precommits can be missing.
		}
		added, err := voteSet.AddVote(commit.GetVote(uint32(idx)))
		if !added || err != nil {
			panic(fmt.Sprintf("Failed to reconstruct LastCommit: %v", err))
		}
	}
	return voteSet
}

// VoteSignBytes constructs the SignBytes for the given CommitSig.
// The only unique part of the SignBytes is the Timestamp - all other fields
// signed over are otherwise the same for all validators.
// Panics if valIdx >= commit.Size().
func (commit *Commit) VoteSignBytes(chainID string, valIdx uint32) []byte {
	v := commit.GetVote(valIdx).ToProto()
	return VoteSignBytes(chainID, v)
}

// GetVote converts the CommitSig for the given valIdx to a Vote.
// Returns nil if the precommit at valIdx is nil.
// Panics if valIdx >= commit.Size().
func (commit *Commit) GetVote(valIdx uint32) *Vote {
	commitSig := commit.Signatures[valIdx]
	return &Vote{
		Type:             kproto.PrecommitType,
		Height:           commit.Height,
		Round:            commit.Round,
		BlockID:          commitSig.BlockID(commit.BlockID),
		Timestamp:        commitSig.Timestamp,
		ValidatorAddress: commitSig.ValidatorAddress,
		ValidatorIndex:   valIdx,
		Signature:        commitSig.Signature,
	}
}

// Copy ...
func (commit *Commit) Copy() *Commit {
	commitCopy := *commit
	commitCopy.Signatures = commit.Signatures
	return &commitCopy
}

// GetHeight returns the height of the commit
func (commit *Commit) GetHeight() uint64 {
	return commit.Height
}

// GetRound returns the round of the commit
func (commit *Commit) GetRound() uint32 {
	return commit.Round
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
func (commit *Commit) Type() kproto.SignedMsgType {
	return kproto.PrecommitType
}

// Size returns the number of votes in the commit
func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Signatures)
}

// BitArray returns a BitArray of which validators voted in this commit
func (commit *Commit) BitArray() *cmn.BitArray {
	if commit.bitArray == nil {
		commit.bitArray = cmn.NewBitArray(len(commit.Signatures))
		for i, commitSig := range commit.Signatures {
			// TODO: need to check the BlockID otherwise we could be counting conflicts,
			// not just the one with +2/3 !
			commit.bitArray.SetIndex(i, !commitSig.Absent())
		}
	}
	return commit.bitArray
}

// GetByIndex returns the vote corresponding to a given validator index
func (commit *Commit) GetByIndex(valIdx uint32) *Vote {
	return commit.GetVote(valIdx)
}

// IsCommit returns true if there is at least one signature.
// Implements VoteSetReader.
func (commit *Commit) IsCommit() bool {
	return len(commit.Signatures) != 0
}

// Hash returns the hash of the commit
func (commit *Commit) Hash() cmn.Hash {
	// TODO(namdoh): Cache hash so we don't have to re-hash all the time.
	return rlpHash(commit)
}

// ValidateBasic performs basic validation that doesn't involve state data.
func (commit *Commit) ValidateBasic() error {
	if commit.Height >= 1 {
		if commit.BlockID.IsZero() {
			return errors.New("Commit cannot be for nil block")
		}
		if len(commit.Signatures) == 0 {
			return errors.New("no signatures in commit")
		}
		for i, commitSig := range commit.Signatures {
			if err := commitSig.ValidateBasic(); err != nil {
				return fmt.Errorf("wrong CommitSig #%d: %v", i, err)
			}
		}
	}
	return nil
}

// StringLong returns a long string representing full info about Commit
func (commit *Commit) StringLong() string {
	if commit == nil {
		return "nil-Commit"
	}

	commitSigStrings := make([]string, len(commit.Signatures))
	for i, commitSig := range commit.Signatures {
		commitSigStrings[i] = commitSig.String()
	}

	return fmt.Sprintf("Commit{BlockID:%v  Precommits:%v}#%v",
		commit.BlockID,
		strings.Join(commitSigStrings, "\n,"),
		commit.hash.Hex())
}

// String returns a short string representing commit by simplifying byte array to hex
func (commit *Commit) String() string {
	if commit == nil {
		return "nil-commit"
	}
	commitSigStrings := make([]string, len(commit.Signatures))
	for i, commitSig := range commit.Signatures {
		commitSigStrings[i] = commitSig.String()
	}

	return fmt.Sprintf("Commit{BlockID:%v  Precommits:%v}#%v",
		commit.BlockID,
		strings.Join(commitSigStrings, "\n,"),
		commit.hash.Fingerprint())
}

// ToProto converts Commit to protobuf
func (commit *Commit) ToProto() *kproto.Commit {
	if commit == nil {
		return nil
	}

	c := new(kproto.Commit)
	sigs := make([]kproto.CommitSig, len(commit.Signatures))
	for i := range commit.Signatures {
		sigs[i] = *commit.Signatures[i].ToProto()
	}
	c.Signatures = sigs

	c.Height = commit.Height
	c.Round = commit.Round
	c.BlockID = commit.BlockID.ToProto()

	return c
}

// FromProto sets a protobuf Commit to the given pointer.
// It returns an error if the commit is invalid.
func CommitFromProto(cp *kproto.Commit) (*Commit, error) {
	if cp == nil {
		return nil, errors.New("nil Commit")
	}

	var (
		commit = new(Commit)
	)

	bi, err := BlockIDFromProto(&cp.BlockID)
	if err != nil {
		return nil, err
	}

	sigs := make([]CommitSig, len(cp.Signatures))
	for i := range cp.Signatures {
		if err := sigs[i].FromProto(cp.Signatures[i]); err != nil {
			return nil, err
		}
	}
	commit.Signatures = sigs

	commit.Height = cp.Height
	commit.Round = cp.Round
	commit.BlockID = *bi

	return commit, commit.ValidateBasic()
}
