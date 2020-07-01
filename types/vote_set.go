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
	"bytes"
	"fmt"
	"sync"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/p2p/discover"
	"github.com/pkg/errors"
)

/*
	VoteSet helps collect signatures from validators at each height+round for a
	predefined vote type.
	We need VoteSet to be able to keep track of conflicting votes when validators
	double-sign.  Yet, we can't keep track of *all* the votes seen, as that could
	be a DoS attack vector.
	There are two storage areas for votes.
	1. voteSet.votes
	2. voteSet.votesByBlock
	`.votes` is the "canonical" list of votes.  It always has at least one vote,
	if a vote from a validator had been seen at all.  Usually it keeps track of
	the first vote seen, but when a 2/3 majority is found, votes for that get
	priority and are copied over from `.votesByBlock`.
	`.votesByBlock` keeps track of a list of votes for a particular block.  There
	are two ways a &blockVotes{} gets created in `.votesByBlock`.
	1. the first vote seen by a validator was for the particular block.
	2. a peer claims to have seen 2/3 majority for the particular block.
	Since the first vote from a validator will always get added in `.votesByBlock`
	, all votes in `.votes` will have a corresponding entry in `.votesByBlock`.
	When a &blockVotes{} in `.votesByBlock` reaches a 2/3 majority quorum, its
	votes are copied into `.votes`.
	All this is memory bounded because conflicting votes only get added if a peer
	told us to track that block, each peer only gets to tell us 1 such block, and,
	there's only a limited number of peers.
	NOTE: Assumes that the sum total of voting power does not exceed MaxUInt64.
*/
type VoteSet struct {
	chainID string
	height  *cmn.BigInt
	round   *cmn.BigInt
	type_   SignedMsgType
	valSet  *ValidatorSet

	mtx           sync.Mutex
	votesBitArray *cmn.BitArray
	votes         []*Vote                     // Primary votes to share
	sum           uint64                      // Sum of voting power for seen votes, discounting conflicts
	maj23         *BlockID                    // First 2/3 majority seen
	votesByBlock  map[string]*blockVotes      // string(blockHash|blockParts) -> blockVotes
	peerMaj23s    map[discover.NodeID]BlockID // Maj23 for each peer
}

// Constructs a new VoteSet struct used to accumulate votes for given height/round.
func NewVoteSet(chainID string, height *cmn.BigInt, round *cmn.BigInt, t SignedMsgType, valSet *ValidatorSet) *VoteSet {
	if height.EqualsInt(0) {
		panic("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	return &VoteSet{
		chainID:       chainID,
		height:        height,
		round:         round,
		type_:         t,
		valSet:        valSet,
		votesBitArray: cmn.NewBitArray(valSet.Size()),
		votes:         make([]*Vote, valSet.Size()),
		sum:           0,
		maj23:         nil,
		votesByBlock:  make(map[string]*blockVotes, valSet.Size()),
		peerMaj23s:    make(map[discover.NodeID]BlockID),
	}
}

// Returns added=true if vote is valid and new.
// Otherwise returns err=ErrVote[
//		UnexpectedStep | InvalidIndex | InvalidAddress |
//		InvalidSignature | InvalidBlockHash | ConflictingVotes ]
// Duplicate votes return added=false, err=nil.
// Conflicting votes return added=*, err=ErrVoteConflictingVotes.
// NOTE: vote should not be mutated after adding.
// NOTE: VoteSet must not be nil
// NOTE: Vote must not be nil
func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if voteSet == nil {
		cmn.PanicSanity("AddVote() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addVote(vote)
}

// NOTE: Validates as much as possible before attempting to verify the signature.
func (voteSet *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, ErrVoteNil
	}
	valIndex := vote.ValidatorIndex.Int32()
	valAddr := vote.ValidatorAddress
	blockKey := vote.BlockID.Key()

	// Ensure that validator index was set
	if valIndex < 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorIndex, "Index < 0")
	} else if len(valAddr) == 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorAddress, "Empty address")
	}

	// Make sure the step matches.
	if !vote.Height.Equals(voteSet.height) ||
		!vote.Round.Equals(voteSet.round) ||
		vote.Type != voteSet.type_ {
		return false, errors.Wrapf(ErrVoteUnexpectedStep, "Got %v/%v/%v, expected %v/%v/%v",
			voteSet.height, voteSet.round, voteSet.type_,
			vote.Height, vote.Round, vote.Type)
	}

	// Ensure that signer is a validator.
	lookupAddr, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		return false, errors.Wrapf(ErrVoteInvalidValidatorIndex,
			"Cannot find validator %d in valSet of size %d", valIndex, voteSet.valSet.Size())
	}

	// Ensure that the signer has the right address
	if !valAddr.Equal(lookupAddr) {
		return false, errors.Wrapf(ErrVoteInvalidValidatorAddress,
			"vote.ValidatorAddress (%X) does not match address (%X) for vote.ValidatorIndex (%d)\nEnsure the genesis file is correct across all validators.",
			valAddr, lookupAddr, valIndex)
	}

	// If we already know of this vote, return false.
	if existing, ok := voteSet.getVote(valIndex, blockKey); ok {
		if bytes.Equal(existing.Signature, vote.Signature) {
			return false, nil // duplicate
		}
		return false, errors.Wrapf(ErrVoteNonDeterministicSignature, "Existing vote: %v; New vote: %v", existing, vote)
	}

	// Check signature.
	if !val.VerifyVoteSignature(voteSet.chainID, vote) {
		return false, errors.Wrapf(ErrVoteInvalidSignature, "Failed to verify vote with ChainID: %s and Validator: %s", voteSet.chainID, val.Address.String())
	}

	// Add vote and get conflicting vote if any
	added, conflicting := voteSet.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, NewConflictingVoteError(val, conflicting, vote)
	}
	if !added {
		cmn.PanicSanity("Expected to add non-conflicting vote")
	}
	return added, nil
}

// Returns (vote, true) if vote exists for valIndex and blockKey
func (voteSet *VoteSet) getVote(valIndex int, blockKey string) (vote *Vote, ok bool) {
	if existing := voteSet.votes[valIndex]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := voteSet.votesByBlock[blockKey].getByIndex(valIndex); existing != nil {
		return existing, true
	}
	return nil, false
}

// Assumes signature is valid.
// If conflicting vote exists, returns it.
func (voteSet *VoteSet) addVerifiedVote(vote *Vote, blockKey string, votingPower int64) (added bool, conflicting *Vote) {
	valIndex := vote.ValidatorIndex.Int32()

	// Already exists in voteSet.votes?
	if existing := voteSet.votes[valIndex]; existing != nil {
		if existing.BlockID.Equal(vote.BlockID) {
			cmn.PanicSanity("addVerifiedVote does not expect duplicate votes")
		} else {
			conflicting = existing
		}
		// Replace vote if blockKey matches voteSet.maj23.
		if voteSet.maj23 != nil && voteSet.maj23.Key() == blockKey {
			voteSet.votes[valIndex] = vote
			voteSet.votesBitArray.SetIndex(valIndex, true)
		}
		// Otherwise don't add it to voteSet.votes
	} else {
		// Add to voteSet.votes and incr .sum
		voteSet.votes[valIndex] = vote
		voteSet.votesBitArray.SetIndex(valIndex, true)
		voteSet.sum += uint64(votingPower)
	}

	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if conflicting != nil && !votesByBlock.peerMaj23 {
			// There's a conflict and no peer claims that this block is special.
			return false, conflicting
		}
		// We'll add the vote in a bit.
	} else {
		// .votesByBlock doesn't exist...
		if conflicting != nil {
			// ... and there's a conflicting vote.
			// We're not even tracking this blockKey, so just forget it.
			return false, conflicting
		}
		// ... and there's no conflicting vote.
		// Start tracking this blockKey
		votesByBlock = newBlockVotes(false, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// We'll add the vote in a bit.
	}

	// Before adding to votesByBlock, see if we'll exceed quorum
	origSum := votesByBlock.sum
	quorum := voteSet.valSet.TotalVotingPower()*2/3 + 1

	// Add vote to votesByBlock
	votesByBlock.addVerifiedVote(vote, votingPower)

	// If we just crossed the quorum threshold and have 2/3 majority...
	if origSum < quorum && quorum <= votesByBlock.sum {
		// Only consider the first quorum reached
		if voteSet.maj23 == nil {
			maj23BlockID := vote.BlockID
			voteSet.maj23 = &maj23BlockID
			// And also copy votes over to voteSet.votes
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					voteSet.votes[i] = vote
				}
			}
		}
	}

	return true, conflicting
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
// NOTE: VoteSet must not be nil
func (voteSet *VoteSet) SetPeerMaj23(peerID discover.NodeID, blockID BlockID) error {
	if voteSet == nil {
		cmn.PanicSanity("SetPeerMaj23() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	blockKey := blockID.Key()

	// Make sure peer hasn't already told us something.
	if existing, ok := voteSet.peerMaj23s[peerID]; ok {
		if existing.Equal(blockID) {
			return nil // Nothing to do
		}
		return fmt.Errorf("SetPeerMaj23: Received conflicting blockID from peer %v. Got %v, expected %v",
			peerID, blockID, existing)
	}
	voteSet.peerMaj23s[peerID] = blockID

	// Create .votesByBlock entry if needed.
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if votesByBlock.peerMaj23 {
			return nil // Nothing to do
		}
		votesByBlock.peerMaj23 = true
		// No need to copy votes, already there.
	} else {
		votesByBlock = newBlockVotes(true, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// No need to copy votes, no votes to copy over.
	}
	return nil
}

func (voteSet *VoteSet) ChainID() string {
	return voteSet.chainID
}

func (voteSet *VoteSet) Height() *cmn.BigInt {
	if voteSet == nil {
		return cmn.NewBigInt64(0)
	}
	return voteSet.height
}

func (voteSet *VoteSet) Round() *cmn.BigInt {
	if voteSet == nil {
		return cmn.NewBigInt64(-1)
	}
	return voteSet.round
}

func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	}
	return byte(voteSet.type_)
}

func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	}
	return voteSet.valSet.Size()
}

func (voteSet *VoteSet) BitArray() *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	votesByBlock, ok := voteSet.votesByBlock[blockID.Key()]
	if ok {
		return votesByBlock.bitArray.Copy()
	}
	return nil
}

// NOTE: if validator has conflicting votes, returns "canonical" vote
func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) IsCommit() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.type_ != PrecommitType {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) GetByAddress(address cmn.Address) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		panic("GetByAddress(address) returned nil")
	}
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum > uint64(voteSet.valSet.TotalVotingPower()*2/3)
}

func (voteSet *VoteSet) HasAll() bool {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum == uint64(voteSet.valSet.TotalVotingPower())
}

// If there was a +2/3 majority for blockID, return blockID and true.
// Else, return the empty BlockID{} and false.
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if voteSet == nil {
		return NewZeroBlockID(), false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23 != nil {
		return *voteSet.maj23, true
	}
	return NewZeroBlockID(), false
}

func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	_, _, frac := voteSet.sumTotalFrac()
	return fmt.Sprintf("VoteSet{H:%v R:%v T:%v +2/3:%v(%v) %v %v}",
		voteSet.height, voteSet.round, voteSet.type_, voteSet.maj23, frac, voteSet.votesBitArray, voteSet.peerMaj23s)
}

// return the power voted, the total, and the fraction
func (voteSet *VoteSet) sumTotalFrac() (int64, int64, float64) {
	voted, total := voteSet.sum, voteSet.valSet.TotalVotingPower()
	fracVoted := float64(voted) / float64(total)
	return int64(voted), total, fracVoted
}

func (voteSet *VoteSet) MakeCommit() *Commit {
	if voteSet.type_ != PrecommitType {
		cmn.PanicSanity("Cannot MakeCommit() unless VoteSet.Type is VoteTypePrecommit")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Make sure we have a 2/3 majority
	if voteSet.maj23 == nil {
		cmn.PanicSanity("Cannot MakeCommit() unless a blockhash has +2/3")
	}

	// For every validator, get the precommit
	commitSigs := make([]*CommitSig, len(voteSet.votes))
	for i, v := range voteSet.votes {
		commitSigs[i] = v.CommitSig()
	}
	return NewCommit(*voteSet.maj23, commitSigs)
}

//--------------------------------------------------------------------------------

/*
	Votes for a particular block
	There are two ways a *blockVotes gets created for a blockKey.
	1. first (non-conflicting) vote of a validator w/ blockKey (peerMaj23=false)
	2. A peer claims to have a 2/3 majority w/ blockKey (peerMaj23=true)
*/
type blockVotes struct {
	peerMaj23 bool          // peer claims to have maj23
	bitArray  *cmn.BitArray // valIndex -> hasVote?
	votes     []*Vote       // valIndex -> *Vote
	sum       int64         // vote sum
}

func newBlockVotes(peerMaj23 bool, numValidators int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  cmn.NewBitArray(numValidators),
		votes:     make([]*Vote, numValidators),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower int64) {
	valIndex := vote.ValidatorIndex.Int32()
	if existing := vs.votes[valIndex]; existing == nil {
		vs.bitArray.SetIndex(valIndex, true)
		vs.votes[valIndex] = vote
		vs.sum += votingPower
	}
}

func (vs *blockVotes) getByIndex(index int) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[index]
}

// Common interface between *consensus.VoteSet and types.Commit
type VoteSetReader interface {
	Height() *cmn.BigInt
	Round() *cmn.BigInt
	Type() byte
	Size() int
	BitArray() *cmn.BitArray
	GetByIndex(int) *Vote
	IsCommit() bool
}
