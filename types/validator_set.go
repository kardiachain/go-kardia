/*
 *  Copyright 2020 KardiaChain
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
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/merkle"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

// MaxTotalVotingPower - the maximum allowed total voting power.
// It needs to be sufficiently small to, in all cases:
// 1. prevent clipping in incrementProposerPriority()
// 2. let (diff+diffMax-1) not overflow in IncrementProposerPriority()
// (Proof of 1 is tricky, left to the reader).
// It could be higher, but this is sufficiently large for our purposes,
// and leaves room for defensive purposes.
// PriorityWindowSizeFactor - is a constant that when multiplied with the total voting power gives
// the maximum allowed distance between validator priorities.

const (
	//todo @longnd: should we move this one to configs folder to avoid misconfiguration for test/dev
	MaxTotalVotingPower      = int64(math.MaxInt64) / 8
	PriorityWindowSizeFactor = 2
)

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height - ie. the validators
// are sorted by their address.
// On the other hand, the .ProposerPriority of each validator and
// the designated .GetProposer() of a set changes every round,
// upon calling .IncrementProposerPriority().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
type ValidatorSet struct {
	// NOTE: persisted via reflect, must be exported.
	Validators []*Validator `json:"validators"`
	Proposer   *Validator   `json:"proposer"`

	// cached (unexported)
	totalVotingPower int64
}

// NewValidatorSet initializes a ValidatorSet by copying over the
// values from `valz`, a list of Validators. If valz is nil or empty,
// the new ValidatorSet will have an empty list of Validators.
// The addresses of validators in `valz` must be unique otherwise the
// function panics.
func NewValidatorSet(val []*Validator) *ValidatorSet {
	vs := &ValidatorSet{}
	err := vs.updateWithChangeSet(val, false)

	if err != nil {
		panic(fmt.Sprintf("cannot create validator set: %s", err))
	}
	if len(val) > 0 {
		vs.IncrementProposerPriority(1)
	}
	return vs
}

// ValidateBasic basic validate validator set
func (vs *ValidatorSet) ValidateBasic() error {
	if vs.IsNilOrEmpty() {
		return errors.New("validator set is nil or empty")
	}

	for idx, val := range vs.Validators {
		if err := val.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid validator #%d: %w", idx, err)
		}
	}

	if err := vs.Proposer.ValidateBasic(); err != nil {
		return fmt.Errorf("proposer failed validate basic, error: %w", err)
	}

	return nil
}

// IsNilOrEmpty validator sets are invalid.
func (vs *ValidatorSet) IsNilOrEmpty() bool {
	return vs == nil || len(vs.Validators) == 0
}

// CurrentValidators returns the current set of validators.
func (vs *ValidatorSet) CurrentValidators() []*Validator {
	return vs.Validators
}

// CopyIncrementProposerPriority Increment ProposerPriority and update the proposer on a copy, and return it.
// Use when create genesis state, so its should panic if vs nil before make this call
func (vs *ValidatorSet) CopyIncrementProposerPriority(times int64) *ValidatorSet {
	vsCopy := vs.Copy()
	vsCopy.IncrementProposerPriority(times)
	return vsCopy
}

// IncrementProposerPriority increments ProposerPriority of each validator and updates the
// proposer. Panics if validator set is empty.
// `times` must be positive.
func (vs *ValidatorSet) IncrementProposerPriority(times int64) {
	if vs.IsNilOrEmpty() {
		panic("empty validator set")
	}
	if times <= 0 {
		panic("Cannot call IncrementProposerPriority with non-positive times")
	}

	// Cap the difference between priorities to be proportional to 2*totalPower by
	// re-normalizing priorities, i.e., rescale all priorities by multiplying with:
	//  2*totalVotingPower/(maxPriority - minPriority)
	diffMax := PriorityWindowSizeFactor * vs.TotalVotingPower()
	vs.RescalePriorities(int64(diffMax))
	vs.shiftByAvgProposerPriority()

	var proposer *Validator
	// Call IncrementProposerPriority(1) times times.

	for i := int64(0); i < times; i++ {
		proposer = vs.incrementProposerPriority()
	}
	vs.Proposer = proposer
}

func (vs *ValidatorSet) RescalePriorities(diffMax int64) {
	if vs.IsNilOrEmpty() {
		panic("empty validator set")
	}
	// NOTE: This check is merely a sanity check which could be
	// removed if all tests would init. voting power appropriately;
	// i.e. diffMax should always be > 0
	if diffMax <= 0 {
		return
	}

	// Calculating ceil(diff/diffMax):
	// Re-normalization is performed by dividing by an integer for simplicity.
	// NOTE: This may make debugging priority issues easier as well.
	diff := computeMaxMinPriorityDiff(vs)
	ratio := (diff + diffMax - 1) / diffMax
	if diff > diffMax {
		for _, val := range vs.Validators {
			cmpPriority := val.ProposerPriority / ratio
			val.ProposerPriority = cmpPriority
		}
	}
}

func (vs *ValidatorSet) incrementProposerPriority() *Validator {
	for _, val := range vs.Validators {
		// Check for overflow for sum.
		newPriority := val.ProposerPriority + int64(val.VotingPower)
		val.ProposerPriority = newPriority
	}
	// Decrement the validator with most ProposerPriority.
	mostest := vs.getValWithMostPriority()
	// Mind the underflow.
	mostest.ProposerPriority = safeSubClip(mostest.ProposerPriority, int64(vs.TotalVotingPower()))
	return mostest
}

// Should not be called on an empty validator set.
func (vs *ValidatorSet) computeAvgProposerPriority() int64 {
	n := int64(len(vs.Validators))

	sum := big.NewInt(0)
	for _, v := range vs.Validators {
		sum.Add(sum, big.NewInt(v.ProposerPriority))
	}
	avg := sum.Div(sum, big.NewInt(n))
	if avg.IsInt64() {
		return avg.Int64()
	}

	// This should never happen: each val.ProposerPriority is in bounds of int64.
	panic(fmt.Sprintf("Cannot represent avg ProposerPriority as an int64 %v", avg))
}

// Compute the difference between the max and min ProposerPriority of that set.
func computeMaxMinPriorityDiff(vals *ValidatorSet) int64 {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	max := int64(math.MaxInt64)
	min := int64(math.MinInt64)
	for _, v := range vals.Validators {
		if v.ProposerPriority < min {
			min = v.ProposerPriority
		}
		if v.ProposerPriority > max {
			max = v.ProposerPriority
		}
	}
	diff := max - min
	if diff < 0 {
		return -1 * diff
	} else {
		return diff
	}
}

// getValWithMostPriority get Validator with highest priority
func (vs *ValidatorSet) getValWithMostPriority() *Validator {
	var res *Validator
	for _, val := range vs.Validators {
		res = res.CompareProposerPriority(val)
	}
	return res
}

// shiftByAvgProposerPriority shift average proposer priority
func (vs *ValidatorSet) shiftByAvgProposerPriority() {
	if vs.IsNilOrEmpty() {
		panic("empty validator set")
	}
	avgProposerPriority := vs.computeAvgProposerPriority()
	for _, val := range vs.Validators {
		proposerPriority := safeSubClip(val.ProposerPriority, avgProposerPriority)
		val.ProposerPriority = proposerPriority
	}
}

// Makes a copy of the validator list.
func validatorListCopy(valsList []*Validator) []*Validator {
	if valsList == nil {
		return nil
	}
	valsCopy := make([]*Validator, len(valsList))
	for i, val := range valsList {
		valsCopy[i] = val.Copy()
	}
	return valsCopy
}

// Copy each validator into a new ValidatorSet.
func (vs *ValidatorSet) Copy() *ValidatorSet {
	return &ValidatorSet{
		Validators:       validatorListCopy(vs.Validators),
		Proposer:         vs.Proposer,
		totalVotingPower: vs.totalVotingPower,
	}
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (vs *ValidatorSet) HasAddress(address common.Address) bool {
	for _, val := range vs.Validators {
		if address.Equal(val.Address) {
			return true
		}
	}
	return false
}

// GetByAddress returns an index of the validator with address and validator
// itself if found. Otherwise, -1 and nil are returned.
func (vs *ValidatorSet) GetByAddress(address common.Address) (index int, val *Validator) {
	for idx, val := range vs.Validators {
		if address.Equal(val.Address) {
			return idx, val.Copy()
		}
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (vs *ValidatorSet) GetByIndex(index uint32) (address common.Address, val *Validator) {
	if index >= uint32(len(vs.Validators)) {
		return common.Address{}, nil
	}
	val = vs.Validators[index]
	return val.Address, val.Copy()
}

// Size returns the length of the validator set.
func (vs *ValidatorSet) Size() int {
	return len(vs.Validators)
}

// Force recalculation of the set's total voting power.
func (vs *ValidatorSet) updateTotalVotingPower() {

	sum := int64(0)
	for _, val := range vs.Validators {
		// mind overflow
		sum = safeAddClip(sum, val.VotingPower)
		if sum > MaxTotalVotingPower {
			panic(fmt.Sprintf(
				"Total voting power should be guarded to not exceed %v; got: %v",
				MaxTotalVotingPower,
				sum))
		}
	}
	vs.totalVotingPower = sum
}

// TotalVotingPower returns the sum of the voting powers of all validators.
// It recomputes the total voting power if required.
func (vs *ValidatorSet) TotalVotingPower() int64 {
	if vs.totalVotingPower == 0 {
		vs.updateTotalVotingPower()
	}
	return vs.totalVotingPower
}

// GetProposer returns the current proposer. If the validator set is empty, nil
// is returned.
func (vs *ValidatorSet) GetProposer() (proposer *Validator) {
	if len(vs.Validators) == 0 {
		return nil
	}
	if vs.Proposer == nil {
		vs.Proposer = vs.findProposer()
	}
	return vs.Proposer.Copy()
}

func (vs *ValidatorSet) findProposer() *Validator {
	var proposer *Validator
	for _, val := range vs.Validators {
		if proposer == nil || !val.Address.Equal(proposer.Address) {
			proposer = proposer.CompareProposerPriority(val)
		}
	}
	return proposer
}

// Hash returns the Merkle root hash build using validators (as leaves) in the
// set.
func (vs *ValidatorSet) Hash() common.Hash {
	if len(vs.Validators) == 0 {
		return common.NewZeroHash()
	}
	bzs := make([][]byte, len(vs.Validators))
	for i, val := range vs.Validators {
		bzs[i] = val.Bytes()
	}
	proof := merkle.SimpleHashFromByteSlices(bzs)
	return common.BytesToHash(proof)
}

// Iterate will run the given function over the set.
func (vs *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range vs.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// Checks changes against duplicates, splits the changes in updates and removals, sorts them by address.
//
// Returns:
// updates, removals - the sorted lists of updates and removals
// err - non-nil if duplicate entries or entries with negative voting power are seen
//
// No changes are made to 'origChanges'.
func processChanges(origChanges []*Validator) (updates, removals []*Validator, err error) {
	// Make a deep copy of the changes and sort by address.
	changes := validatorListCopy(origChanges)
	sort.Sort(ValidatorsByAddress(changes))

	removals = make([]*Validator, 0, len(changes))
	updates = make([]*Validator, 0, len(changes))
	var prevAddr common.Address

	// Scan changes by address and append valid validators to updates or removals lists.
	for _, valUpdate := range changes {
		if valUpdate.Address.Equal(prevAddr) {
			err = fmt.Errorf("duplicate entry %v in %v", valUpdate, changes)
			return nil, nil, err
		}

		switch {
		case valUpdate.VotingPower < 0:
			return nil, nil, fmt.Errorf("voting power can't be negative: %d", valUpdate.VotingPower)
		case valUpdate.VotingPower > MaxTotalVotingPower:
			return nil, nil, fmt.Errorf("to prevent clipping/overflow, voting power can't be higher than %d, got %d", MaxTotalVotingPower, valUpdate.VotingPower)
		case valUpdate.VotingPower == 0:
			removals = append(removals, valUpdate)
		default:
			updates = append(updates, valUpdate)
		}

		prevAddr = valUpdate.Address
	}
	return updates, removals, err
}

// Verifies a list of updates against a validator set, making sure the allowed
// total voting power would not be exceeded if these updates would be applied to the set.
//
// Returns:
// updatedTotalVotingPower - the new total voting power if these updates would be applied
// numNewValidators - number of new validators
// err - non-nil if the maximum allowed total voting power would be exceeded
//
// 'updates' should be a list of proper validator changes, i.e. they have been verified
// by processChanges for duplicates and invalid values.
// No changes are made to the validator set 'vals'.
func verifyUpdates(
	updates []*Validator,
	vals *ValidatorSet,
	removedPower int64,
) (tvpAfterUpdatesBeforeRemovals int64, err error) {

	delta := func(update *Validator, vals *ValidatorSet) int64 {
		_, val := vals.GetByAddress(update.Address)
		if val != nil {
			return int64(update.VotingPower) - int64(val.VotingPower)
		}
		return int64(update.VotingPower)
	}

	updatesCopy := validatorListCopy(updates)
	sort.Slice(updatesCopy, func(i, j int) bool {
		return delta(updatesCopy[i], vals) < delta(updatesCopy[j], vals)
	})

	tvpAfterRemovals := int64(vals.TotalVotingPower()) - removedPower
	for _, upd := range updatesCopy {
		tvpAfterRemovals += delta(upd, vals)
		if tvpAfterRemovals > int64(MaxTotalVotingPower) {
			return 0, fmt.Errorf("total voting power of resulting valset exceeds max %d",
				MaxTotalVotingPower)
		}
	}
	return tvpAfterRemovals + removedPower, nil
}

// Computes the proposer priority for the validators not present in the set based on 'updatedTotalVotingPower'.
// Leaves unchanged the priorities of validators that are changed.
//
// 'updates' parameter must be a list of unique validators to be added or updated.
// No changes are made to the validator set 'vs'.
func computeNewPriorities(updates []*Validator, vs *ValidatorSet, updatedTotalVotingPower int64) {

	for _, valUpdate := range updates {
		address := valUpdate.Address
		_, val := vs.GetByAddress(address)
		if val == nil {
			// add val
			// Set ProposerPriority to -C*totalVotingPower (with C ~= 1.125) to make sure validators can't
			// un-bond and then re-bond to reset their (potentially previously negative) ProposerPriority to zero.
			//
			// Contract: updatedVotingPower < MaxTotalVotingPower to ensure ProposerPriority does
			// not exceed the bounds of int64.
			//
			// Compute ProposerPriority = -1.125*totalVotingPower == -(updatedVotingPower + (updatedVotingPower >> 3)).
			proposerPriority := -(updatedTotalVotingPower + (updatedTotalVotingPower >> 3))
			valUpdate.ProposerPriority = proposerPriority
		} else {
			valUpdate.ProposerPriority = val.ProposerPriority
		}
	}

}

// Merges the vals' validator list with the updates list.
// When two elements with same address are seen, the one from updates is selected.
// Expects updates to be a list of updates sorted by address with no duplicates or errors,
// must have been validated with verifyUpdates() and priorities computed with computeNewPriorities().
func (vs *ValidatorSet) applyUpdates(updates []*Validator) {

	existing := vs.Validators
	sort.Sort(ValidatorsByAddress(existing))

	merged := make([]*Validator, len(existing)+len(updates))
	i := 0

	for len(existing) > 0 && len(updates) > 0 {
		if bytes.Compare(existing[0].Address.Bytes(), updates[0].Address.Bytes()) < 0 { // unchanged validator
			merged[i] = existing[0]
			existing = existing[1:]
		} else {
			// Apply add or update.
			merged[i] = updates[0]
			if existing[0].Address.Equal(updates[0].Address) {
				// Validator is present in both, advance existing.
				existing = existing[1:]
			}
			updates = updates[1:]
		}
		i++
	}

	// Add the elements which are left.
	for j := 0; j < len(existing); j++ {
		merged[i] = existing[j]
		i++
	}
	// OR add updates which are left.
	for j := 0; j < len(updates); j++ {
		merged[i] = updates[j]
		i++
	}

	vs.Validators = merged[:i]
}

// Checks that the validators to be removed are part of the validator set.
// No changes are made to the validator set 'vals'.
func verifyRemovals(deletes []*Validator, vs *ValidatorSet) (int64, error) {
	removedVotingPower := int64(0)
	for _, valUpdate := range deletes {
		address := valUpdate.Address
		_, val := vs.GetByAddress(address)
		if val == nil {
			return removedVotingPower, fmt.Errorf("failed to find validator %X to remove", address)
		}
		removedVotingPower += val.VotingPower
	}
	if len(deletes) > len(vs.Validators) {
		panic("more deletes than validators")
	}
	return removedVotingPower, nil
}

// Removes the validators specified in 'deletes' from validator set 'vals'.
// Should not fail as verification has been done before.
func (vs *ValidatorSet) applyRemovals(deletes []*Validator) {

	existing := vs.Validators

	merged := make([]*Validator, len(existing)-len(deletes))
	i := 0

	// Loop over deletes until we removed all of them.
	for len(deletes) > 0 {
		if existing[0].Address.Equal(deletes[0].Address) {
			deletes = deletes[1:]
		} else { // Leave it in the resulting slice.
			merged[i] = existing[0]
			i++
		}
		existing = existing[1:]
	}

	// Add the elements which are left.
	for j := 0; j < len(existing); j++ {
		merged[i] = existing[j]
		i++
	}

	vs.Validators = merged[:i]
}

// Main function used by UpdateWithChangeSet() and NewValidatorSet().
// If 'allowDeletes' is false then delete operations (identified by validators with voting power 0)
// are not allowed and will trigger an error if present in 'changes'.
// The 'allowDeletes' flag is set to false by NewValidatorSet() and to true by UpdateWithChangeSet().
func (vs *ValidatorSet) updateWithChangeSet(changes []*Validator, allowDeletes bool) error {

	if len(changes) == 0 {
		return nil
	}

	// Check for duplicates within changes, split in 'updates' and 'deletes' lists (sorted).
	updates, deletes, err := processChanges(changes)
	if err != nil {
		return err
	}

	if !allowDeletes && len(deletes) != 0 {
		return fmt.Errorf("cannot process validators with voting power 0: %v", deletes)
	}

	// Verify that applying the 'deletes' against 'vals' will not result in error.
	removedVotingPower, err := verifyRemovals(deletes, vs)
	if err != nil {
		return err
	}

	// Verify that applying the 'updates' against 'vals' will not result in error.
	updatedTotalVotingPower, err := verifyUpdates(updates, vs, int64(removedVotingPower))
	if err != nil {
		return err
	}

	// Check that the resulting set will not be empty.
	// Check that the resulting set will not be empty.
	if numNewValidators(updates, vs) == 0 && len(vs.Validators) == len(deletes) {
		return errors.New("applying the validator changes would result in empty set")
	}

	// Compute the priorities for updates.
	computeNewPriorities(updates, vs, int64(updatedTotalVotingPower))
	// Apply updates and removals.
	vs.applyUpdates(updates)
	// vs.Proposer = updates[0]
	vs.applyRemovals(deletes)

	vs.updateTotalVotingPower()

	// Scale and center.
	vs.RescalePriorities(PriorityWindowSizeFactor * int64(vs.TotalVotingPower()))

	vs.shiftByAvgProposerPriority()
	sort.Sort(ValidatorsByVotingPower(vs.Validators))

	return nil
}

// UpdateWithChangeSet attempts to update the validator set with 'changes'.
// It performs the following steps:
// - validates the changes making sure there are no duplicates and splits them in updates and deletes
// - verifies that applying the changes will not result in errors
// - computes the total voting power BEFORE removals to ensure that in the next steps the priorities
//   across old and newly added validators are fair
// - computes the priorities of new validators against the final set
// - applies the updates against the validator set
// - applies the removals against the validator set
// - performs scaling and centering of priority values
// If an error is detected during verification steps, it is returned and the validator set
// is not changed.
func (vs *ValidatorSet) UpdateWithChangeSet(changes []*Validator) error {
	return vs.updateWithChangeSet(changes, true)
}

// VerifyCommit verify that +2/3 of the set had signed the given signBytes.
func (vs *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height uint64, commit *Commit) error {
	if vs == nil {
		return ErrNilValidatorSet
	}
	if commit == nil {
		return ErrNilCommit
	}
	if err := commit.ValidateBasic(); err != nil {
		return err
	}
	if vs.Size() != len(commit.Signatures) {
		return NewErrInvalidCommitSignatures(uint64(vs.Size()), uint64(len(commit.Signatures)))
	}
	if height != commit.GetHeight() {
		return NewErrInvalidCommitHeight(height, commit.GetHeight())
	}
	if !blockID.Equal(commit.BlockID) {
		return fmt.Errorf("Invalid commit -- wrong block id: want %v got %v",
			blockID, commit.BlockID)
	}

	talliedVotingPower := int64(0)
	votingPowerNeeded := vs.TotalVotingPower() * 2 / 3
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue // OK, some signatures can be absent.
		}
		// The vals and commit have a 1-to-1 correspondance.
		// This means we don't need the validator address or to do any lookup.
		val := vs.Validators[idx]

		// Validate signature.
		signBytes := commit.VoteSignBytes(chainID, uint32(idx))
		if !VerifySignature(val.Address, crypto.Keccak256(signBytes), commitSig.Signature) {
			return errors.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
		}
		// Good precommit!
		if blockID.Equal(commitSig.BlockID(commit.BlockID)) {
			talliedVotingPower += val.VotingPower
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}
	return nil
}

// IsErrTooMuchChange returns too much change error
func IsErrTooMuchChange(err error) bool {
	_, ok := errors.Cause(err).(errTooMuchChange)
	return ok
}

type errTooMuchChange struct {
	got    int64
	needed int64
}

func (e errTooMuchChange) Error() string {
	return fmt.Sprintf("Invalid commit -- insufficient old voting power: got %v, needed %v", e.got, e.needed)
}

// StringIndented returns validator set as string
func (vs *ValidatorSet) StringIndented(indent string) string {
	if vs == nil {
		return "nil-ValidatorSet"
	}
	var valStrings []string
	vs.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
 %s  Proposer: %v
 %s  Validators:
 %s    %v
 %s}`,
		indent, vs.GetProposer().String(),
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}

// ToProto converts ValidatorSet to protobuf
func (vs *ValidatorSet) ToProto() (*kproto.ValidatorSet, error) {
	if vs.IsNilOrEmpty() {
		return &kproto.ValidatorSet{}, nil // validator set should never be nil
	}

	vp := new(kproto.ValidatorSet)
	valsProto := make([]*kproto.Validator, len(vs.Validators))
	for i := 0; i < len(vs.Validators); i++ {
		valp, err := vs.Validators[i].ToProto()
		if err != nil {
			return nil, err
		}
		valsProto[i] = valp
	}
	vp.Validators = valsProto

	valProposer, err := vs.Proposer.ToProto()
	if err != nil {
		return nil, fmt.Errorf("toProto: validatorSet proposer error: %w", err)
	}
	vp.Proposer = valProposer

	vp.TotalVotingPower = vs.totalVotingPower

	return vp, nil
}

// ValidatorSetFromProto sets a protobuf ValidatorSet to the given pointer.
// It returns an error if any of the validators from the set or the proposer
// is invalid
func ValidatorSetFromProto(vp *kproto.ValidatorSet) (*ValidatorSet, error) {
	if vp == nil {
		return nil, errors.New("nil validator set") // validator set should never be nil, bigger issues are at play if empty
	}
	vals := new(ValidatorSet)

	valsProto := make([]*Validator, len(vp.Validators))
	for i := 0; i < len(vp.Validators); i++ {
		v, err := ValidatorFromProto(vp.Validators[i])
		if err != nil {
			return nil, err
		}
		valsProto[i] = v
	}
	vals.Validators = valsProto

	p, err := ValidatorFromProto(vp.GetProposer())
	if err != nil {
		return nil, fmt.Errorf("fromProto: validatorSet proposer error: %w", err)
	}

	vals.Proposer = p

	vals.totalVotingPower = vp.GetTotalVotingPower()

	return vals, vals.ValidateBasic()
}

//-------------------------------------
// Implements sort for sorting validators by address.

// ValidatorsByAddress sorts validators by address.
type ValidatorsByAddress []*Validator

func (vals ValidatorsByAddress) Len() int {
	return len(vals)
}

func (vals ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(vals[i].Address.Bytes(), vals[j].Address.Bytes()) == -1
}

func (vals ValidatorsByAddress) Swap(i, j int) {
	vals[i], vals[j] = vals[j], vals[i]
}

//----------------------------------------
// for testing

// RandValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has a voting power of +votingPower+.
// EXPOSED FOR TESTING.
// RandValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has a voting power of +votingPower+.
//
func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []PrivValidator) {
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]PrivValidator, numValidators)
		// privValz       = make([]PrivValidator, numValidators)
	)
	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}
	valSet := NewValidatorSet(valz)
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return valSet, privValidators
}

// Errors handle
type (
	// ErrInvalidCommitHeight is returned when we encounter a commit with an
	// unexpected height.
	ErrInvalidCommitHeight struct {
		Expected uint64
		Actual   uint64
	}

	// ErrInvalidCommitSignatures is returned when we encounter a commit where
	// the number of signatures doesn't match the number of validators.
	ErrInvalidCommitSignatures struct {
		Expected uint64
		Actual   uint64
	}
)

// NewErrInvalidCommitHeight return invalid commit height error
func NewErrInvalidCommitHeight(expected, actual uint64) ErrInvalidCommitHeight {
	return ErrInvalidCommitHeight{
		Expected: expected,
		Actual:   actual,
	}
}

// NewErrInvalidCommitSignatures returns invalid commit signatures error
func NewErrInvalidCommitSignatures(expected, actual uint64) ErrInvalidCommitSignatures {
	return ErrInvalidCommitSignatures{
		Expected: expected,
		Actual:   actual,
	}
}

func (e ErrInvalidCommitHeight) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong height: %v vs %v", e.Expected, e.Actual)
}

func (e ErrInvalidCommitSignatures) Error() string {
	return fmt.Sprintf("Invalid commit -- wrong set size: %v vs %v", e.Expected, e.Actual)
}

// Safe maths

func safeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return -1, true
	} else if b < 0 && a < math.MinInt64-b {
		return -1, true
	}
	return a + b, false
}

func safeSub(a, b int64) (int64, bool) {
	if b > 0 && a < math.MinInt64+b {
		return -1, true
	} else if b < 0 && a > math.MaxInt64+b {
		return -1, true
	}
	return a - b, false
}

func safeAddClip(a, b int64) int64 {
	c, overflow := safeAdd(a, b)
	if overflow {
		if b < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func safeSubClip(a, b int64) int64 {
	c, overflow := safeSub(a, b)
	if overflow {
		if b > 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func numNewValidators(updates []*Validator, vals *ValidatorSet) int {
	numNewValidators := 0
	for _, valUpdate := range updates {
		if !vals.HasAddress(valUpdate.Address) {
			numNewValidators++
		}
	}
	return numNewValidators
}

// ValidatorsByVotingPower implements sort.Interface for []*Validator based on
// the VotingPower and Address fields.
type ValidatorsByVotingPower []*Validator

func (valz ValidatorsByVotingPower) Len() int { return len(valz) }

func (valz ValidatorsByVotingPower) Less(i, j int) bool {
	if valz[i].VotingPower == valz[j].VotingPower {
		return bytes.Compare(valz[i].Address.Bytes(), valz[j].Address.Bytes()) == -1
	}
	return valz[i].VotingPower > valz[j].VotingPower
}

func (valz ValidatorsByVotingPower) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}
