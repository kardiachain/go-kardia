package types

import (
	"bytes"
	"sort"

	"github.com/kardiachain/go-kardia/common"
)

// Volatile state for each Validator
type Validator struct {
	Address     common.Address `json:"address"`
	VotingPower int64          `json:"voting_power"`
}

func NewValidator(address common.Address, votingPower int64) *Validator {
	return &Validator{
		Address:     address,
		VotingPower: votingPower,
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

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
//
// TODO(huny@): The first prototype assumes static set of Validators with round-robin proposer
type ValidatorSet struct {
	// NOTE: persisted via reflect, must be exported.
	Validators []*Validator `json:"validators"`
	Proposer   *Validator   `json:"proposer"`

	// cached (unexported)
	totalVotingPower int64
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vs := &ValidatorSet{
		Validators: validators,
	}

	if vals != nil {
		vs.Proposer = vs.findNextProposer()
	}

	return vs
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
func (valSet *ValidatorSet) GetByAddress(address common.Address) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address.Bytes(), valSet.Validators[i].Address.Bytes()) <= 0
	})
	if idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address.Bytes(), address.Bytes()) {
		return idx, valSet.Validators[idx].Copy()
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (valSet *ValidatorSet) GetByIndex(index int) (address common.Address, val *Validator) {
	if index < 0 || index >= len(valSet.Validators) {
		return common.BytesToAddress(nil), nil
	}
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

// Size returns the length of the validator set.
func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators)
}

// TotalVotingPower returns the sum of the voting powers of all validators.
func (valSet *ValidatorSet) TotalVotingPower() int64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators {
			// mind overflow
			valSet.totalVotingPower = valSet.totalVotingPower + val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

// GetProposer returns the current proposer. If the validator set is empty, nil
// is returned.
func (valSet *ValidatorSet) GetProposer() (proposer *Validator) {
	if len(valSet.Validators) == 0 {
		return nil
	}
	if valSet.Proposer == nil {
		valSet.Proposer = valSet.findNextProposer()
	}
	return valSet.Proposer.Copy()
}

// Simple round-robin proposer picker
// TODO(huny@): Implement more fancy algo based on accum later
func (valSet *ValidatorSet) findNextProposer() *Validator {
	if valSet.Proposer == nil {
		return valSet.Validators[0]
	}
	for i, val := range valSet.Validators {
		if bytes.Equal(val.Address.Bytes(), valSet.Proposer.Address.Bytes()) {
			if i == valSet.Size()-1 {
				return valSet.Validators[0]
			} else {
				return valSet.Validators[i+1]
			}
		}
	}
	// Reaching here means current proposer is NOT in the set, so return the first validator
	return valSet.Validators[0]
}

// TODO(huny@): Probably use Merkle proof tree with Validators as leaves?
func (valSet *ValidatorSet) Hash() common.Hash {
	return rlpHash(valSet)
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
