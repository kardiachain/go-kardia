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
	"crypto/ecdsa"
	"fmt"
	"sort"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
)

// Volatile state for each Validator
type Validator struct {
	Address     common.Address  `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`

	Accum int64 `json:"accum"`
}

func NewValidator(pubKey ecdsa.PublicKey, votingPower int64) *Validator {
	return &Validator{
		Address:     crypto.PubkeyToAddress(pubKey),
		PubKey:      pubKey,
		VotingPower: votingPower,
		Accum:       0,
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
	if v.Accum > other.Accum {
		return v
	} else if v.Accum < other.Accum {
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

func (v *Validator) VerifyProposalSignature(chainID string, proposal *Proposal) bool {
	hash := rlpHash(proposal.SignBytes(chainID))
	pubKey, _ := crypto.SigToPub(hash[:], proposal.Signature[:])
	// TODO(thientn): Verifying signature shouldn't be this complicated. After
	// cleaning up our crypto package, clean up this as well.
	return bytes.Equal(crypto.CompressPubkey(pubKey), crypto.CompressPubkey(&v.PubKey))
}

func (v *Validator) VerifyVoteSignature(chainID string, vote *Vote) bool {
	hash := rlpHash(vote.SignBytes(chainID))
	pubKey, _ := crypto.SigToPub(hash[:], vote.Signature[:])
	// TODO(thientn): Verifying signature shouldn't be this complicated. After
	// cleaning up our crypto package, clean up this as well.
	return bytes.Equal(crypto.CompressPubkey(pubKey), crypto.CompressPubkey(&v.PubKey))
}

// StringLong returns a long string representing full info about Validator
func (v *Validator) StringLong() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v %v VP:%v A:%v}",
		v.Address,
		v.PubKey,
		v.VotingPower,
		v.Accum)
}

// StringShort returns a short string representing Validator
func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%X %v VP:%v A:%v}",
		common.Fingerprint(v.Address[:]),
		v.PubKey,
		v.VotingPower,
		v.Accum)
}

// --------- ValidatorSet ----------

// Represents a set of validators staked for a given window.
type StakedValidators struct {
	Validators []*Validator `json:"validators"`
	// Start block height of the staked validators. The value is inclusive.
	StartHeight int64 `json:"startHeight"`
	// End block height of the staked validators. The value is inclusive.
	EndHeight int64 `json:"endHeight"`
}

// Returns a long string representing full info about StakedValidator
func (stakedVals *StakedValidators) StringLong() string {
	if stakedVals == nil {
		return "nil-StakedValidators"
	}
	valStrings := []string{}
	stakedVals.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf("StakedValidators{Validators:%v, StartHeight:%v, EndHeight:%v}",
		strings.Join(valStrings, "  "), stakedVals.StartHeight, stakedVals.EndHeight)
}

// Iterate will run the given function over the set.
func (stakedVals *StakedValidators) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range stakedVals.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

func (stakedVals *StakedValidators) Copy() *StakedValidators {
	validators := make([]*Validator, len(stakedVals.Validators))
	for i, val := range stakedVals.Validators {
		// NOTE: must copy, since AdvanceProposer updates in place.
		validators[i] = val.Copy()
	}

	return &StakedValidators{
		Validators:  validators,
		StartHeight: stakedVals.StartHeight,
		EndHeight:   stakedVals.EndHeight,
	}
}

// Represents a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// Note: Not goroutine-safe.
// Note: All get/set to validators should copy the value for safety.
type ValidatorSet struct {
	// Current staked validator set.
	Validators *StakedValidators `json:"validators"`
	// Next staked validator set.
	NextValidators *StakedValidators `json:"nextValidators"`
	// Current proposing validator.
	Proposer *Validator `json:"proposer"`

	// cached (unexported)
	totalVotingPower int64

	// ======== DEV ENVIRONMENT CONFIG =========
	KeepSameProposer bool `json:"keep_same_proposer"`
	// TODO(namdoh): Move this node config
	// Indicates the how step height before the current staked validators' end height that we start
	// to refresh the validator set after the end height.
	refreshBackoffHeightStep int64
	// Indicates step height delta for refresh retry.
	refreshHeightDelta int64
}

func NewValidatorSet(vals []*Validator, startHeight int64, endHeight int64, refreshBackoff int64, refreshDelta int64) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vs := &ValidatorSet{
		Validators: &StakedValidators{
			Validators:  validators,
			StartHeight: startHeight,
			EndHeight:   endHeight,
		},
		NextValidators:           nil,
		refreshBackoffHeightStep: refreshBackoff,
		refreshHeightDelta:       refreshDelta,
	}

	if vals != nil {
		vs.Proposer = vs.Validators.Validators[0]
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
	idx := sort.Search(len(valSet.Validators.Validators), func(i int) bool {
		return bytes.Compare(address.Bytes(), valSet.Validators.Validators[i].Address.Bytes()) <= 0
	})
	return idx < len(valSet.Validators.Validators) && bytes.Equal(valSet.Validators.Validators[idx].Address.Bytes(), address.Bytes())
}

// GetByAddress returns an index of the validator with address and validator
// itself if found. Otherwise, -1 and nil are returned.
func (valSet *ValidatorSet) GetByAddress(address common.Address) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators.Validators), func(i int) bool {
		return bytes.Compare(address.Bytes(), valSet.Validators.Validators[i].Address.Bytes()) <= 0
	})
	if idx < len(valSet.Validators.Validators) && bytes.Equal(valSet.Validators.Validators[idx].Address.Bytes(), address.Bytes()) {
		return idx, valSet.Validators.Validators[idx].Copy()
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (valSet *ValidatorSet) GetByIndex(index int) (address common.Address, val *Validator) {
	if valSet.Validators == nil || index < 0 || index >= len(valSet.Validators.Validators) {
		return common.BytesToAddress(nil), nil
	}
	val = valSet.Validators.Validators[index]
	return val.Address, val.Copy()
}

// Returns the length of the validator set.
func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators.Validators)
}

// Returns the sum of the voting powers of all validators.
func (valSet *ValidatorSet) TotalVotingPower() int64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators.Validators {
			// mind overflow
			valSet.totalVotingPower = valSet.totalVotingPower + val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

// May fresh current or future validator sets, or both.
// The refreshing policy is needed to optimize how often we need to fetch validator set from
// staking smart contract. Policy:
//
// (1) if "height" is greater than the current staked validator set's end height:
//     i)  if "height" is in within the next validator set's start/end window: assign the next
//         validator set to the current validator set. However, if next validator set is empty,
//         do nothing.
//     ii) if "height" is greater than the next validator set's end height: fetch current staked
//         validator set.
// (2) if "height" is within the current staked validator set's start/end height window:
//     i)  current validator set: do not fetch.
//     ii) next validator set: if "height" is greater or equal to
//         (end_height - refreshBackoffHeightStep) and the next staked validator set is nil,
//         fetch it.
//         NOTE: Consider doing this asynchronously, but beware of race condition.
// (3) if "height" is less than the current staked validator set's start height:
//     i)  current validator set: do not fetch
//     ii) next validator set: do not fetch
//
// Note: This must be called before advancing to the next proposer.
func (valSet *ValidatorSet) mayRefreshValidatorSet(height int64) {
	// Case #1
	currentVals := valSet.Validators
	nextVals := valSet.NextValidators
	if height > currentVals.EndHeight {
		if height >= nextVals.StartHeight && height <= nextVals.EndHeight {
			valSet.Validators = valSet.NextValidators
		} else if height > nextVals.EndHeight {
			valSet.Validators = valSet.fetchValidatorSet(height)
		}
		valSet.NextValidators = nil
	}

	// Case #2
	currentVals = valSet.Validators
	nextVals = valSet.NextValidators
	if nextVals != nil && height >= currentVals.StartHeight && height <= nextVals.EndHeight {
		if height >= currentVals.EndHeight-valSet.refreshBackoffHeightStep && nextVals != nil &&
			(height-(currentVals.EndHeight-valSet.refreshBackoffHeightStep))%valSet.refreshHeightDelta == 0 /* check step-wise refresh */ {
			valSet.NextValidators = valSet.fetchValidatorSet(currentVals.EndHeight + 1)
		}
	}

	// Case #3: Do nothing
}

// Fetches the validator set at a given height.
// TODO(huny@): Please implement this function.
func (valSet *ValidatorSet) fetchValidatorSet(height int64) *StakedValidators {
	// TODO(huny@): Update this.
	return valSet.Validators
}

// Returns the current set of validators.
func (valSet *ValidatorSet) CurrentValidators() []*Validator {
	return valSet.Validators.Validators
}

// Returns the current proposer. If the validator set is empty, nil
// is returned.
func (valSet *ValidatorSet) GetProposer() *Validator {
	if valSet.Validators == nil || len(valSet.Validators.Validators) == 0 {
		return nil
	}
	if valSet.Proposer == nil {
		valSet.Proposer = valSet.Validators.Validators[0]
	}
	return valSet.Proposer.Copy()
}

// TODO(huny@): Probably use Merkle proof tree with Validators.Validators as leaves?
func (valSet *ValidatorSet) Hash() common.Hash {
	return rlpHash(valSet)
}

// Copy each validator into a new ValidatorSet
func (valSet *ValidatorSet) Copy() *ValidatorSet {
	var copiedVals *StakedValidators
	if valSet.Validators == nil {
		copiedVals = nil
	} else {
		copiedVals = valSet.Validators.Copy()
	}
	var copiedNextVals *StakedValidators
	if valSet.NextValidators == nil {
		copiedNextVals = nil
	} else {
		copiedNextVals = valSet.NextValidators.Copy()
	}
	return &ValidatorSet{
		Validators:               copiedVals,
		NextValidators:           copiedNextVals,
		Proposer:                 valSet.Proposer,
		totalVotingPower:         valSet.totalVotingPower,
		refreshBackoffHeightStep: valSet.refreshBackoffHeightStep,
		refreshHeightDelta:       valSet.refreshHeightDelta,
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

	valSet.mayRefreshValidatorSet(times)

	validatorsHeap := common.NewHeap()
	// Update voting power of each validator after "times" increments.
	for _, val := range valSet.Validators.Validators {
		val.Accum = common.AddWithClip(val.Accum, common.MulWithClip(val.VotingPower, int64(times)))
		validatorsHeap.PushComparable(val, accumComparable{val})
	}

	// Loop "times" time to set the latest proposer.
	// TODO(namdoh@): Revise the following logic as the next validator set is udpated.
	for i := int64(0); i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		mostest.Accum = common.SubWithClip(mostest.Accum, valSet.TotalVotingPower())

		if i == times-1 {
			valSet.Proposer = mostest
		} else {
			validatorsHeap.Update(mostest, accumComparable{mostest})
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int64, commit *Commit) error {
	if valSet.Size() != len(commit.Precommits) {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", valSet.Size(), len(commit.Precommits))
	}
	if !commit.Height().EqualsInt64(height) {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}

	talliedVotingPower := int64(0)
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		// may be nil if validator skipped.
		if precommit == nil {
			continue
		}
		if !precommit.Height.EqualsInt64(height) {
			return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, precommit.Height)
		}
		if !precommit.Round.Equals(round) {
			return fmt.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		_, val := valSet.GetByIndex(idx)
		// Validate signature
		if !val.VerifyVoteSignature(chainID, precommit) {
			return fmt.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		if !blockID.Equal(precommit.BlockID) {
			continue // Not an error, but doesn't count
		}
		// Good precommit!
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	}
	return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
		talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
}

// StringLong returns a long string representing full info about Validator
func (valSet *ValidatorSet) StringLong() string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	return fmt.Sprintf("ValidatorSet{Proposer:%v  Validators:%v  NextValidators:%v}",
		valSet.GetProposer(), valSet.Validators, valSet.NextValidators)

}

// StringShort returns a short string representing of ValidatorSet
func (valSet *ValidatorSet) String() string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	return fmt.Sprintf("ValidatorSet{Proposer:%v  Validators:%v  NextValidators:%v}",
		valSet.GetProposer().String(), valSet.Validators, valSet.NextValidators)
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
