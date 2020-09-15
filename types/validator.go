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
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/crypto"

	"github.com/kardiachain/go-kardiamain/lib/common"
)

const (
	MaxTotalVotingPower = int64(math.MaxInt64) / 8
)

// ErrTotalVotingPowerOverflow is returned if the total voting power of the
// resulting validator set exceeds MaxTotalVotingPower.
var ErrTotalVotingPowerOverflow = fmt.Errorf("total voting power of resulting valset exceeds max %d",
	MaxTotalVotingPower)

//-----------------

// IsErrNotEnoughVotingPowerSigned returns true if err is
// ErrNotEnoughVotingPowerSigned.
func IsErrNotEnoughVotingPowerSigned(err error) bool {
	return errors.As(err, &ErrNotEnoughVotingPowerSigned{})
}

// ErrNotEnoughVotingPowerSigned is returned when not enough validators signed
// a commit.
type ErrNotEnoughVotingPowerSigned struct {
	Got    uint64
	Needed uint64
}

func (e ErrNotEnoughVotingPowerSigned) Error() string {
	return fmt.Sprintf("invalid commit -- insufficient voting power: got %d, needed more than %d", e.Got, e.Needed)
}

// Volatile state for each Validator
type Validator struct {
	Address     common.Address `json:"address"`
	VotingPower uint64         `json:"voting_power"`
	Accum       *common.BigInt `json:"accum"`
}

func NewValidator(addr common.Address, votingPower uint64) *Validator {
	return &Validator{
		Address:     addr,
		VotingPower: votingPower,
		Accum:       common.NewBigInt64(0),
	}
}

// Hash computes the unique ID of a validator with a given voting power.
func (v *Validator) Hash() common.Hash {
	return rlpHash(v)
}

// Creates a new copy of the validator.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// Returns the one with higher Accum.
func (v *Validator) CompareAccum(other *Validator) *Validator {
	if v == nil {
		return other
	}
	if v.Accum.IsGreaterThan(other.Accum) {
		return v
	} else if v.Accum.IsLessThan(other.Accum) {
		return other
	} else {
		result := bytes.Compare(v.Address[:], other.Address[:])
		if result < 0 {
			return v
		} else if result > 0 {
			return other
		} else {
			common.PanicSanity("Cannot compare identical validators")
			return nil
		}
	}
}

// VerifyProposalSignature ...
func (v *Validator) VerifyProposalSignature(chainID string, proposal *Proposal) bool {
	hash := rlpHash(proposal.SignBytes(chainID))
	return VerifySignature(v.Address, hash[:], proposal.Signature[:])
}

// VerifyVoteSignature ...
func (v *Validator) VerifyVoteSignature(chainID string, vote *Vote) bool {
	hash := rlpHash(vote.SignBytes(chainID))
	return VerifySignature(v.Address, hash[:], vote.Signature[:])
}

// StringLong returns a long string representing full info about Validator
func (v *Validator) StringLong() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v VP:%v A:%v}",
		v.Address,
		v.VotingPower,
		v.Accum)
}

// StringShort returns a short string representing Validator
func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%X VP:%v A:%v}",
		common.Fingerprint(v.Address[:]),
		v.VotingPower,
		v.Accum)
}

// ValidatorSet ----------
// Represents a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// Note: Not goroutine-safe.
// Note: All get/set to validators should copy the value for safety.
type ValidatorSet struct {
	// Validator set.
	Validators []*Validator `json:"validators"`
	// Current proposing validator.
	Proposer *Validator `json:"proposer" rlp:"nil"`
	// Start block height of the staked validators. The value is inclusive.
	StartHeight uint64 `json:"startHeight"`
	// End block height of the staked validators. The value is inclusive.
	EndHeight uint64 `json:"endHeight"`

	// cached (unexported)
	totalVotingPower uint64

	// ======== DEV ENVIRONMENT CONFIG =========
	KeepSameProposer bool `json:"keep_same_proposer"`
	// TODO(namdoh): Move this node config
	// Indicates the how step height before the current staked validators' end height that we start
	// to refresh the validator set after the end height.
	refreshBackoffHeightStep int64
	// Indicates step height delta for refresh retry.
	refreshHeightDelta int64
}

// NewValidatorSet ..
func NewValidatorSet(vals []*Validator, startHeight int64, endHeight int64) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vs := &ValidatorSet{
		Validators:  validators,
		StartHeight: uint64(startHeight),
		EndHeight:   uint64(endHeight),
	}

	if vals != nil {
		vs.Proposer = vs.Validators[0]
	}

	return vs
}

// NOTE: This function should only be used only in dev environment.
func (valSet *ValidatorSet) TurnOnKeepSameProposer() {
	valSet.KeepSameProposer = true
}

// NOTE: This function should only be used in dev environment and when
// KeepSameProposer is set to true. For testnet, or mainnet proposer should be
// set automatically.
func (valSet *ValidatorSet) SetProposer(proposer *Validator) {
	if !valSet.KeepSameProposer {
		common.PanicSanity(
			"SetProposer should never be called when KeepSameProposer is off")
	}
	valSet.Proposer = proposer
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (valSet *ValidatorSet) HasAddress(address common.Address) bool {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address.Bytes(), valSet.Validators[i].Address.Bytes()) <= 0
	})
	return idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address.Bytes(), address.Bytes())
}

// GetByAddress returns an index of the validator with address and validator
// itself if found. Otherwise, -1 and nil are returned.
func (valSet *ValidatorSet) GetByAddress(address common.Address) (index int32, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address.Bytes(), valSet.Validators[i].Address.Bytes()) <= 0
	})
	if idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address.Bytes(), address.Bytes()) {
		return int32(idx), valSet.Validators[idx].Copy()
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (valSet *ValidatorSet) GetByIndex(index uint32) (address common.Address, val *Validator) {
	if valSet.Validators == nil || index < 0 || index >= uint32(len(valSet.Validators)) {
		return common.BytesToAddress(nil), nil
	}
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

// Returns the length of the validator set.
func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators)
}

// Returns the sum of the voting powers of all validators.
func (valSet *ValidatorSet) TotalVotingPower() uint64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators {
			// mind overflow
			valSet.totalVotingPower = valSet.totalVotingPower + val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

// Returns the current set of validators.
func (valSet *ValidatorSet) CurrentValidators() []*Validator {
	return valSet.Validators
}

// Returns the current proposer. If the validator set is empty, nil
// is returned.
func (valSet *ValidatorSet) GetProposer() *Validator {
	if valSet.Validators == nil || len(valSet.Validators) == 0 {
		return nil
	}
	if valSet.Proposer == nil {
		valSet.Proposer = valSet.Validators[0]
	}
	return valSet.Proposer.Copy()
}

// TODO(huny@): Probably use Merkle proof tree with Validators as leaves?
func (valSet *ValidatorSet) Hash() common.Hash {
	return rlpHash(valSet)
}

// Copy each validator into a new ValidatorSet
func (valSet *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(valSet.Validators))
	for i, val := range valSet.Validators {
		// NOTE: must copy, since AdvanceProposer updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		Validators:       validators,
		Proposer:         valSet.Proposer,
		StartHeight:      valSet.StartHeight,
		EndHeight:        valSet.EndHeight,
		totalVotingPower: valSet.totalVotingPower,
	}
}

// Advances proposer a given number of times. To advance to the next proposer, call this with
// 'times' is 1.
func (valSet *ValidatorSet) AdvanceProposer(times int64) {
	// MUST STAY AT THE BEGIN OF THE FUNCTION.
	// Note: This is --dev mode only. Do not remove.
	if valSet.KeepSameProposer {
		return
	}

	validatorsHeap := common.NewHeap()
	// Update voting power of each validator after "times" increments.
	for _, val := range valSet.Validators {
		val.Accum = common.NewBigInt64(common.AddWithClip(val.Accum.Int64(), common.MulWithClip(int64(val.VotingPower), int64(times))))
		validatorsHeap.PushComparable(val, accumComparable{val})
	}

	// Loop "times" time to set the latest proposer.
	// TODO(namdoh@): Revise the following logic as the next validator set is updated.
	for i := int64(0); i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		mostest.Accum = common.NewBigInt64(common.SubWithClip(mostest.Accum.Int64(), int64(valSet.TotalVotingPower())))

		if i == times-1 {
			valSet.Proposer = mostest
		} else {
			validatorsHeap.Update(mostest, accumComparable{mostest})
		}
	}
}

// Iterate will run the given function over the set.
func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range valSet.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// VerifyCommit that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height uint64, commit *Commit) error {
	if valSet.Size() != len(commit.Signatures) {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", valSet.Size(), len(commit.Signatures))
	}
	if commit.Height != height {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height)
	}

	talliedVotingPower := uint64(0)
	votingPowerNeeded := valSet.TotalVotingPower() * 2 / 3
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue // OK, some signatures can be absent.
		}

		// The vals and commit have a 1-to-1 correspondance.
		// This means we don't need the validator address or to do any lookup.
		val := valSet.Validators[idx]

		// Validate signature
		if !val.VerifyVoteSignature(chainID, commit.GetVote(uint32(idx))) {
			return fmt.Errorf("Invalid commit -- invalid signature: %v", commitSig)
		}
		if commitSig.ForBlock() {
			// Good precommit!
			talliedVotingPower += val.VotingPower
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}
	return nil
}

// StringLong returns a long string representing full info about Validator
func (valSet *ValidatorSet) StringLong() string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	valStrings := []string{}
	valSet.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf("ValidatorSet{Proposer:%v  Validators:%v}",
		valSet.GetProposer(), strings.Join(valStrings, "  "))
}

// Returns a short string representing of ValidatorSet
func (valSet *ValidatorSet) String() string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	valStrings := []string{}
	valSet.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf("ValidatorSet{Proposer:%v  Validators:%v}",
		valSet.GetProposer().String(), strings.Join(valStrings, "  "))
}

//-------------------------------------
// Implements sort for sorting validators by address.

// Sort validators by address
type ValidatorsByAddress []*Validator

func (vs ValidatorsByAddress) Len() int {
	return len(vs)
}

func (vs ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(vs[i].Address.Bytes(), vs[j].Address.Bytes()) == -1
}

func (vs ValidatorsByAddress) Swap(i, j int) {
	it := vs[i]
	vs[i] = vs[j]
	vs[j] = it
}

//-------------------------------------
// Use with Heap for sorting validators by accum

type accumComparable struct {
	*Validator
}

// We want to find the validator with the greatest accum.
func (ac accumComparable) Less(o interface{}) bool {
	other := o.(accumComparable).Validator
	larger := ac.CompareAccum(other)
	return bytes.Equal(larger.Address[:], ac.Address[:])
}

//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
func RandValidator(randPower bool, minPower uint64) (*Validator, IPrivValidator) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += rand.Uint64()
	}
	pubKey := privVal.GetPubKey()
	val := NewValidator(crypto.PubkeyToAddress(pubKey), votePower)
	return val, privVal
}

// RandValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has a voting power of +votingPower+.
//
// EXPOSED FOR TESTING.
func RandValidatorSet(numValidators int, votingPower uint64) (*ValidatorSet, []IPrivValidator) {
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]IPrivValidator, numValidators)
	)
	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}
	valSet := NewValidatorSet(valz, 0, 1000000)
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return valSet, privValidators
}
