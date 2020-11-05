/*
 *  Copyright 2019 KardiaChain
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
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	kmath "github.com/kardiachain/go-kardiamain/lib/math"
	krand "github.com/kardiachain/go-kardiamain/lib/rand"
)

func TestValidatorSetValidateBasic(t *testing.T) {
	val, _ := RandValidator(false, 1)

	testCases := []struct {
		vals ValidatorSet
		err  bool
		msg  string
	}{
		{
			vals: ValidatorSet{},
			err:  true,
			msg:  "validator set is nil or empty",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{},
			},
			err: true,
			msg: "validator set is nil or empty",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{val},
			},
			err: true,
			msg: "proposer failed validate basic, error: nil validator",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{val},
				Proposer:   val,
			},
			err: false,
			msg: "",
		},
	}

	for _, tc := range testCases {
		err := tc.vals.ValidateBasic()
		if tc.err {
			if assert.Error(t, err) {
				assert.Equal(t, tc.msg, err.Error())
			}
		} else {
			assert.NoError(t, err)
		}
	}

}

func randValidator(totalVotingPower int64) *Validator {
	// this modulo limits the ProposerPriority/VotingPower to stay in the
	// bounds of MaxTotalVotingPower minus the already existing voting power:
	val := NewValidator(generateAddress(), rand.Int63()%(MaxTotalVotingPower-totalVotingPower))
	proposerPriority := rand.Int63() % (MaxTotalVotingPower - totalVotingPower)
	val.ProposerPriority = proposerPriority
	return val
}

func randValidatorSet(numValidators int) *ValidatorSet {
	validators := make([]*Validator, numValidators)
	totalVotingPower := int64(0)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator(totalVotingPower)
		totalVotingPower += validators[i].VotingPower
	}
	return NewValidatorSet(validators)
}

func TestCopy(t *testing.T) {
	vset := randValidatorSet(10)
	vsetHash := vset.Hash()
	if len(vsetHash) == 0 {
		t.Fatalf("ValidatorSet had unexpected zero hash")
	}

	vsetCopy := vset.Copy()
	vsetCopyHash := vsetCopy.Hash()

	if !vsetHash.Equal(vsetCopyHash) {
		t.Fatalf("ValidatorSet copy had wrong hash. Orig: %X, Copy: %X", vsetHash, vsetCopyHash)
	}
}

// Test that IncrementProposerPriority requires positive times.
func TestIncrementProposerPriorityPositiveTimes(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator(generateAddress(), 1000),
		newValidator(generateAddress(), 300),
		newValidator(generateAddress(), 330),
	})

	assert.Panics(t, func() { vset.IncrementProposerPriority(-1) })
	assert.Panics(t, func() { vset.IncrementProposerPriority(0) })
	vset.IncrementProposerPriority(1)
}

func newValidator(address common.Address, power int64) *Validator {
	return &Validator{Address: address, VotingPower: power}
}

func generateAddress() common.Address {
	privateKey, _ := crypto.GenerateKey()
	publicKey := privateKey.Public()
	publicKeyECDSA, _ := publicKey.(*ecdsa.PublicKey)

	return crypto.PubkeyToAddress(*publicKeyECDSA)
}

func TestProposerSelection1(t *testing.T) {
	v1 := common.BytesToAddress([]byte("foo"))
	v2 := common.BytesToAddress([]byte("bar"))
	v3 := common.BytesToAddress([]byte("baz"))

	vset := NewValidatorSet([]*Validator{
		newValidator(v1, 1000),
		newValidator(v2, 300),
		newValidator(v3, 330),
	})
	var proposers []string
	for i := 0; i < 9; i++ {
		val := vset.GetProposer()
		proposers = append(proposers, val.Address.String())
		vset.IncrementProposerPriority(1)
	}
	// expected := `foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar` +
	// 	` foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar` +
	// 	` foo baz foo foo bar foo baz foo foo bar foo baz foo foo foo baz bar foo foo foo baz` +
	// 	` foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo` +
	// 	` foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo`

	expected := []string{
		v1.String(), v3.String(), v1.String(), v2.String(), v1.String(),
		v1.String(), v3.String(), v1.String(), v2.String(),
	}
	if strings.Join(expected, " ") != strings.Join(proposers, " ") {
		t.Errorf("expected sequence of proposers was\n%v\nbut got \n%v", expected, strings.Join(proposers, " "))
	}
}

func TestProposerSelection2(t *testing.T) {
	addr0 := common.BytesToAddress([]byte{0x1})
	addr1 := common.BytesToAddress([]byte{0x2})
	addr2 := common.BytesToAddress([]byte{0x3})

	// when all voting power is same, we go in order of addresses
	val0, val1, val2 := newValidator(addr0, 100), newValidator(addr1, 100), newValidator(addr2, 100)
	valList := []*Validator{val0, val1, val2}
	vals := NewValidatorSet(valList)
	for i := 0; i < len(valList)*5; i++ {
		ii := (i) % len(valList)
		prop := vals.GetProposer()
		if !bytes.Equal(prop.Address.Bytes(), valList[ii].Address.Bytes()) {
			t.Fatalf("(%d): Expected %X. Got %X", i, valList[ii].Address, prop.Address)
		}
		vals.IncrementProposerPriority(1)
	}

	// One validator has more than the others, but not enough to propose twice in a row
	*val2 = *newValidator(addr2, 400)
	vals = NewValidatorSet(valList)
	//vals.IncrementProposerPriority(1)
	prop := vals.GetProposer()
	if !prop.Address.Equal(addr2) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !prop.Address.Equal(addr0) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// One validator has more than the others, and enough to be proposer twice in a row
	*val2 = *newValidator(addr2, 401)
	vals = NewValidatorSet(valList)
	prop = vals.GetProposer()
	if !prop.Address.Equal(addr2) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !prop.Address.Equal(addr2) {
		t.Fatalf("Expected address with highest voting power to be second proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !prop.Address.Equal(addr0) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// each validator should be the proposer a proportional number of times
	val0, val1, val2 = newValidator(addr0, 4), newValidator(addr1, 5), newValidator(addr2, 3)
	valList = []*Validator{val0, val1, val2}
	propCount := make([]int, 3)
	vals = NewValidatorSet(valList)
	N := 1
	for i := 0; i < 120*N; i++ {
		prop := vals.GetProposer()
		ii := prop.Address[19] - 1
		propCount[ii]++
		vals.IncrementProposerPriority(1)
	}

	if propCount[0] != 40*N {
		t.Fatalf(
			"Expected prop count for validator with 4/12 of voting power to be %d/%d. Got %d/%d",
			40*N,
			120*N,
			propCount[0],
			120*N,
		)
	}
	if propCount[1] != 50*N {
		t.Fatalf(
			"Expected prop count for validator with 5/12 of voting power to be %d/%d. Got %d/%d",
			50*N,
			120*N,
			propCount[1],
			120*N,
		)
	}
	if propCount[2] != 30*N {
		t.Fatalf(
			"Expected prop count for validator with 3/12 of voting power to be %d/%d. Got %d/%d",
			30*N,
			120*N,
			propCount[2],
			120*N,
		)
	}
}

//-------------------------------------------------------------------

func TestValidatorSetTotalVotingPowerPanicsOnOverflow(t *testing.T) {
	// NewValidatorSet calls IncrementProposerPriority which calls TotalVotingPower()
	// which should panic on overflows:
	shouldPanic := func() {
		NewValidatorSet([]*Validator{
			{Address: common.BytesToAddress([]byte("a")), VotingPower: math.MaxInt64, ProposerPriority: 0},
			{Address: common.BytesToAddress([]byte("b")), VotingPower: math.MaxInt64, ProposerPriority: 0},
			{Address: common.BytesToAddress([]byte("c")), VotingPower: math.MaxInt64, ProposerPriority: 0},
		})
	}

	assert.Panics(t, shouldPanic)
}

func TestAvgProposerPriority(t *testing.T) {
	// Create Validator set without calling IncrementProposerPriority:
	tcs := []struct {
		vs   ValidatorSet
		want int64
	}{
		0: {ValidatorSet{Validators: []*Validator{{ProposerPriority: 0}, {ProposerPriority: 0}, {ProposerPriority: 0}}}, 0},
		1: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}, {ProposerPriority: 0}},
			}, math.MaxInt64 / 3,
		},
		2: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}},
			}, math.MaxInt64 / 2,
		},
		3: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: math.MaxInt64}},
			}, math.MaxInt64,
		},
		4: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MinInt64}, {ProposerPriority: math.MinInt64}},
			}, math.MinInt64,
		},
	}
	for i, tc := range tcs {
		got := tc.vs.computeAvgProposerPriority()
		assert.Equal(t, tc.want, got, "test case: %v", i)
	}
}

func TestAveragingInIncrementProposerPriority(t *testing.T) {
	// Test that the averaging works as expected inside of IncrementProposerPriority.
	// Each validator comes with zero voting power which simplifies reasoning about
	// the expected ProposerPriority.
	addr1 := common.BytesToAddress([]byte("a"))
	addr2 := common.BytesToAddress([]byte("b"))
	addr3 := common.BytesToAddress([]byte("c"))
	tcs := []struct {
		vs    ValidatorSet
		times int32
		avg   int64
	}{
		0: {ValidatorSet{
			Validators: []*Validator{
				{Address: addr1, ProposerPriority: 1},
				{Address: addr2, ProposerPriority: 2},
				{Address: addr3, ProposerPriority: 3}}},
			1, 2},
		1: {ValidatorSet{
			Validators: []*Validator{
				{Address: addr1, ProposerPriority: 10},
				{Address: addr2, ProposerPriority: -10},
				{Address: addr3, ProposerPriority: 1}}},
			// this should average twice but the average should be 0 after the first iteration
			// (voting power is 0 -> no changes)
			11, 1 / 3},
		2: {ValidatorSet{
			Validators: []*Validator{
				{Address: addr1, ProposerPriority: 100},
				{Address: addr2, ProposerPriority: -10},
				{Address: addr3, ProposerPriority: 1}}},
			1, 91 / 3},
	}
	for i, tc := range tcs {
		// work on copy to have the old ProposerPriorities:
		newVset := tc.vs.CopyIncrementProposerPriority(int64(tc.times))
		for _, val := range tc.vs.Validators {
			_, updatedVal := newVset.GetByAddress(val.Address)
			assert.Equal(t, updatedVal.ProposerPriority, val.ProposerPriority-tc.avg, "test case: %v", i)
		}
	}
}

func TestAveragingInIncrementProposerPriorityWithVotingPower(t *testing.T) {
	// Other than TestAveragingInIncrementProposerPriority this is a more complete test showing
	// how each ProposerPriority changes in relation to the validator's voting power respectively.
	// average is zero in each round:

	addr1 := common.BytesToAddress([]byte{0x01})
	addr2 := common.BytesToAddress([]byte{0x02})
	addr3 := common.BytesToAddress([]byte{0x03})

	vp0 := int64(10)
	vp1 := int64(1)
	vp2 := int64(1)
	total := vp0 + vp1 + vp2
	avg := (vp0 + vp1 + vp2 - total) / 3
	vals := ValidatorSet{Validators: []*Validator{
		{Address: addr1, ProposerPriority: 0, VotingPower: vp0},
		{Address: addr2, ProposerPriority: 0, VotingPower: vp1},
		{Address: addr3, ProposerPriority: 0, VotingPower: vp2}}}
	tcs := []struct {
		vals                  *ValidatorSet
		wantProposerPrioritys []int64
		times                 int32
		wantProposer          *Validator
	}{

		0: {
			vals.Copy(),
			[]int64{
				// Acumm+VotingPower-Avg:
				0 + vp0 - total - avg, // mostest will be subtracted by total voting power (12)
				0 + vp1,
				0 + vp2},
			1,
			vals.Validators[0]},
		1: {
			vals.Copy(),
			[]int64{
				(0 + vp0 - total) + vp0 - total - avg, // this will be mostest on 2nd iter, too
				(0 + vp1) + vp1,
				(0 + vp2) + vp2},
			2,
			vals.Validators[0]}, // increment twice -> expect average to be subtracted twice
		2: {
			vals.Copy(),
			[]int64{
				0 + 3*(vp0-total) - avg, // still mostest
				0 + 3*vp1,
				0 + 3*vp2},
			3,
			vals.Validators[0]},
		3: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total), // still mostest
				0 + 4*vp1,
				0 + 4*vp2},
			4,
			vals.Validators[0]},
		4: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total) + vp0, // 4 iters was mostest
				0 + 5*vp1 - total,       // now this val is mostest for the 1st time (hence -12==totalVotingPower)
				0 + 5*vp2},
			5,
			vals.Validators[1]},
		5: {
			vals.Copy(),
			[]int64{
				0 + 6*vp0 - 5*total, // mostest again
				0 + 6*vp1 - total,   // mostest once up to here
				0 + 6*vp2},
			6,
			vals.Validators[0]},
		6: {
			vals.Copy(),
			[]int64{
				0 + 7*vp0 - 6*total, // in 7 iters this val is mostest 6 times
				0 + 7*vp1 - total,   // in 7 iters this val is mostest 1 time
				0 + 7*vp2},
			7,
			vals.Validators[0]},
		7: {
			vals.Copy(),
			[]int64{
				0 + 8*vp0 - 7*total, // mostest again
				0 + 8*vp1 - total,
				0 + 8*vp2},
			8,
			vals.Validators[0]},
		8: {
			vals.Copy(),
			[]int64{
				0 + 9*vp0 - 7*total,
				0 + 9*vp1 - total,
				0 + 9*vp2 - total}, // mostest
			9,
			vals.Validators[2]},
		9: {
			vals.Copy(),
			[]int64{
				0 + 10*vp0 - 8*total, // after 10 iters this is mostest again
				0 + 10*vp1 - total,   // after 6 iters this val is "mostest" once and not in between
				0 + 10*vp2 - total},  // in between 10 iters this val is "mostest" once
			10,
			vals.Validators[0]},
		10: {
			vals.Copy(),
			[]int64{
				0 + 11*vp0 - 9*total,
				0 + 11*vp1 - total,  // after 6 iters this val is "mostest" once and not in between
				0 + 11*vp2 - total}, // after 10 iters this val is "mostest" once
			11,
			vals.Validators[0]},
	}
	for i, tc := range tcs {
		tc.vals.IncrementProposerPriority(int64(tc.times))

		assert.Equal(t, tc.wantProposer.Address, tc.vals.GetProposer().Address,
			"test case: %v",
			i)

		for valIdx, val := range tc.vals.Validators {
			assert.Equal(t,
				tc.wantProposerPrioritys[valIdx],
				val.ProposerPriority,
				"test case: %v, validator: %v",
				i,
				valIdx)
		}
	}
}

func TestSafeAdd(t *testing.T) {
	f := func(a, b int64) bool {
		c, overflow := safeAdd(a, b)
		return overflow || (!overflow && c == a+b)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSafeAddClip(t *testing.T) {
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, 10))
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, math.MaxInt64))
	assert.EqualValues(t, math.MinInt64, safeAddClip(math.MinInt64, -10))
}

func TestSafeSubClip(t *testing.T) {
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, 10))
	assert.EqualValues(t, 0, safeSubClip(math.MinInt64, math.MinInt64))
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, math.MaxInt64))
	assert.EqualValues(t, math.MaxInt64, safeSubClip(math.MaxInt64, -10))
}

func TestUpdatesForNewValidatorSet(t *testing.T) {
	addr1 := common.BytesToAddress([]byte("v1"))
	addr2 := common.BytesToAddress([]byte("v2"))
	addr3 := common.BytesToAddress([]byte("v3"))

	v1 := newValidator(addr1, 100)
	v2 := newValidator(addr2, 100)
	valList := []*Validator{v1, v2}
	valSet := NewValidatorSet(valList)
	verifyValidatorSet(t, valSet)

	// Verify duplicates are caught in NewValidatorSet() and it panics
	v111 := newValidator(addr1, 100)
	v112 := newValidator(addr1, 123)
	v113 := newValidator(addr1, 234)
	valList = []*Validator{v111, v112, v113}
	assert.Panics(t, func() { NewValidatorSet(valList) })

	// Verify set including validator with voting power 0 cannot be created
	v1 = newValidator(addr1, 0)
	v2 = newValidator(addr2, 22)
	v3 := newValidator(addr3, 33)
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList) })

	// Verify set including validator with negative voting power cannot be created
	v1 = newValidator(addr1, 10)
	v2 = newValidator(addr2, -20)
	v3 = newValidator(addr3, 30)
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList) })

}

func valSetTotalProposerPriority(valSet *ValidatorSet) int64 {
	sum := int64(0)
	for _, val := range valSet.Validators {
		// mind overflow
		sum = safeAddClip(sum, val.ProposerPriority)
	}
	return sum
}

func verifyValidatorSet(t *testing.T, valSet *ValidatorSet) {
	// verify that the capacity and length of validators is the same
	assert.Equal(t, len(valSet.Validators), cap(valSet.Validators))

	// verify that the set's total voting power has been updated
	tvp := valSet.totalVotingPower
	valSet.updateTotalVotingPower()
	expectedTvp := valSet.TotalVotingPower()
	assert.Equal(t, expectedTvp, tvp,
		"expected TVP %d. Got %d, valSet=%s", expectedTvp, tvp, valSet)

	// verify that validator priorities are centered
	valsCount := int64(len(valSet.Validators))
	tpp := valSetTotalProposerPriority(valSet)
	assert.True(t, tpp < valsCount && tpp > -valsCount,
		"expected total priority in (-%d, %d). Got %d", valsCount, valsCount, tpp)

	// verify that priorities are scaled
	dist := computeMaxMinPriorityDiff(valSet)
	assert.True(t, dist <= PriorityWindowSizeFactor*int64(tvp),
		"expected priority distance < %d. Got %d", PriorityWindowSizeFactor*tvp, dist)
}

type testVal struct {
	name  string
	power int64
}

type valSetErrTestCase struct {
	startVals  []testVal
	updateVals []testVal
}

func testValSet(nVals int, power int64) []testVal {
	vals := make([]testVal, nVals)
	for i := 0; i < nVals; i++ {
		vals[i] = testVal{fmt.Sprintf("v%d", i+1), power}
	}
	return vals
}

func TestValSetUpdatesDuplicateEntries(t *testing.T) {
	testCases := []valSetErrTestCase{
		// Duplicate entries in changes
		{ // first entry is duplicated change
			testValSet(2, 10),
			[]testVal{{"v1", 11}, {"v1", 22}},
		},
		{ // second entry is duplicated change
			testValSet(2, 10),
			[]testVal{{"v2", 11}, {"v2", 22}},
		},
		{ // change duplicates are separated by a valid change
			testValSet(2, 10),
			[]testVal{{"v1", 11}, {"v2", 22}, {"v1", 12}},
		},
		{ // change duplicates are separated by a valid change
			testValSet(3, 10),
			[]testVal{{"v1", 11}, {"v3", 22}, {"v1", 12}},
		},

		// Duplicate entries in remove
		{ // first entry is duplicated remove
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v1", 0}},
		},
		{ // second entry is duplicated remove
			testValSet(2, 10),
			[]testVal{{"v2", 0}, {"v2", 0}},
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 0}, {"v1", 0}},
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(3, 10),
			[]testVal{{"v1", 0}, {"v3", 0}, {"v1", 0}},
		},

		{ // remove and update same val
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 20}, {"v1", 30}},
		},
		{ // duplicate entries in removes + changes
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 20}, {"v2", 30}, {"v1", 0}},
		},
		{ // duplicate entries in removes + changes
			testValSet(3, 10),
			[]testVal{{"v1", 0}, {"v3", 5}, {"v2", 20}, {"v2", 30}, {"v1", 0}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesOverflows(t *testing.T) {
	maxVP := int64(MaxTotalVotingPower)
	testCases := []valSetErrTestCase{
		{ // single update leading to overflow
			testValSet(2, 10),
			[]testVal{{"v1", math.MaxInt64}},
		},
		{ // single update leading to overflow
			testValSet(2, 10),
			[]testVal{{"v2", math.MaxInt64}},
		},
		{ // add validator leading to overflow
			testValSet(1, maxVP),
			[]testVal{{"v2", math.MaxInt64}},
		},
		{ // add validator leading to exceed Max
			testValSet(1, maxVP-1),
			[]testVal{{"v2", 5}},
		},
		{ // add validator leading to exceed Max
			testValSet(2, maxVP/3),
			[]testVal{{"v3", maxVP / 2}},
		},
		{ // add validator leading to exceed Max
			testValSet(1, maxVP),
			[]testVal{{"v2", maxVP}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesOtherErrors(t *testing.T) {
	testCases := []valSetErrTestCase{
		{ // update with negative voting power
			testValSet(2, 10),
			[]testVal{{"v1", -123}},
		},
		{ // update with negative voting power
			testValSet(2, 10),
			[]testVal{{"v2", -123}},
		},
		{ // remove non-existing validator
			testValSet(2, 10),
			[]testVal{{"v3", 0}},
		},
		{ // delete all validators
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}},
			[]testVal{{"v1", 0}, {"v2", 0}, {"v3", 0}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesBasicTestsExecute(t *testing.T) {
	valSetUpdatesBasicTests := []struct {
		startVals    []testVal
		updateVals   []testVal
		expectedVals []testVal
	}{
		{ // no changes
			testValSet(2, 10),
			[]testVal{},
			testValSet(2, 10),
		},
		{ // voting power changes
			testValSet(2, 10),
			[]testVal{{"v2", 22}, {"v1", 11}},
			[]testVal{{"v2", 22}, {"v1", 11}},
		},
		{ // add new validators
			[]testVal{{"v2", 20}, {"v1", 10}},
			[]testVal{{"v4", 40}, {"v3", 30}},
			[]testVal{{"v4", 40}, {"v3", 30}, {"v2", 20}, {"v1", 10}},
		},
		{ // add new validator to middle
			[]testVal{{"v3", 20}, {"v1", 10}},
			[]testVal{{"v2", 30}},
			[]testVal{{"v2", 30}, {"v3", 20}, {"v1", 10}},
		},
		{ // add new validator to beginning
			[]testVal{{"v3", 20}, {"v2", 10}},
			[]testVal{{"v1", 30}},
			[]testVal{{"v1", 30}, {"v3", 20}, {"v2", 10}},
		},
		{ // delete validators
			[]testVal{{"v3", 30}, {"v2", 20}, {"v1", 10}},
			[]testVal{{"v2", 0}},
			[]testVal{{"v3", 30}, {"v1", 10}},
		},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new set and apply updates, keeping copies for the checks
		valSet := createNewValidatorSet(tt.startVals)
		valList := createNewValidatorList(tt.updateVals)
		err := valSet.UpdateWithChangeSet(valList)
		assert.NoError(t, err, "test %d", i)

		valListCopy := validatorListCopy(valSet.Validators)
		// check that the voting power in the set's validators is not changing if the voting power
		// is changed in the list of validators previously passed as parameter to UpdateWithChangeSet.
		// this is to make sure copies of the validators are made by UpdateWithChangeSet.
		if len(valList) > 0 {
			valList[0].VotingPower++
			assert.Equal(t, toTestValList(valListCopy), toTestValList(valSet.Validators), "test %v", i)

		}

		// check the final validator list is as expected and the set is properly scaled and centered.
		assert.Equal(t, tt.expectedVals, toTestValList(valSet.Validators), "test %v", i)
		verifyValidatorSet(t, valSet)
	}
}

// Test that different permutations of an update give the same result.
func TestValSetUpdatesOrderIndependenceTestsExecute(t *testing.T) {
	// startVals - initial validators to create the set with
	// updateVals - a sequence of updates to be applied to the set.
	// updateVals is shuffled a number of times during testing to check for same resulting validator set.
	valSetUpdatesOrderTests := []struct {
		startVals  []testVal
		updateVals []testVal
	}{
		0: { // order of changes should not matter, the final validator sets should be the same
			[]testVal{{"v4", 40}, {"v3", 30}, {"v2", 10}, {"v1", 10}},
			[]testVal{{"v4", 44}, {"v3", 33}, {"v2", 22}, {"v1", 11}}},

		1: { // order of additions should not matter
			[]testVal{{"v2", 20}, {"v1", 10}},
			[]testVal{{"v3", 30}, {"v4", 40}, {"v5", 50}, {"v6", 60}}},

		2: { // order of removals should not matter
			[]testVal{{"v4", 40}, {"v3", 30}, {"v2", 20}, {"v1", 10}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v4", 0}}},

		3: { // order of mixed operations should not matter
			[]testVal{{"v4", 40}, {"v3", 30}, {"v2", 20}, {"v1", 10}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v2", 22}, {"v5", 50}, {"v4", 44}}},
	}

	for i, tt := range valSetUpdatesOrderTests {
		// create a new set and apply updates
		valSet := createNewValidatorSet(tt.startVals)
		valSetCopy := valSet.Copy()
		valList := createNewValidatorList(tt.updateVals)
		assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList))

		// save the result as expected for next updates
		valSetExp := valSetCopy.Copy()

		// perform at most 20 permutations on the updates and call UpdateWithChangeSet()
		n := len(tt.updateVals)
		maxNumPerms := kmath.MinInt(20, n*n)
		for j := 0; j < maxNumPerms; j++ {
			// create a copy of original set and apply a random permutation of updates
			valSetCopy := valSet.Copy()
			valList := createNewValidatorList(permutation(tt.updateVals))

			// check there was no error and the set is properly scaled and centered.
			assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList),
				"test %v failed for permutation %v", i, valList)
			verifyValidatorSet(t, valSetCopy)

			// verify the resulting test is same as the expected
			assert.Equal(t, valSetCopy, valSetExp,
				"test %v failed for permutation %v", i, valList)
		}
	}
}

// This tests the private function validator_set.go:applyUpdates() function, used only for additions and changes.
// Should perform a proper merge of updatedVals and startVals
func TestValSetApplyUpdatesTestsExecute(t *testing.T) {
	valSetUpdatesBasicTests := []struct {
		startVals    []testVal
		updateVals   []testVal
		expectedVals []testVal
	}{
		// additions
		0: { // prepend
			[]testVal{{"v4", 44}, {"v5", 55}},
			[]testVal{{"v1", 11}},
			[]testVal{{"v1", 11}, {"v4", 44}, {"v5", 55}}},
		1: { // append
			[]testVal{{"v4", 44}, {"v5", 55}},
			[]testVal{{"v6", 66}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}}},
		2: { // insert
			[]testVal{{"v4", 44}, {"v6", 66}},
			[]testVal{{"v5", 55}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}}},
		3: { // insert multi
			[]testVal{{"v4", 44}, {"v6", 66}, {"v9", 99}},
			[]testVal{{"v5", 55}, {"v7", 77}, {"v8", 88}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}, {"v7", 77}, {"v8", 88}, {"v9", 99}}},
		// changes
		4: { // head
			[]testVal{{"v1", 111}, {"v2", 22}},
			[]testVal{{"v1", 11}},
			[]testVal{{"v1", 11}, {"v2", 22}}},
		5: { // tail
			[]testVal{{"v1", 11}, {"v2", 222}},
			[]testVal{{"v2", 22}},
			[]testVal{{"v1", 11}, {"v2", 22}}},
		6: { // middle
			[]testVal{{"v1", 11}, {"v2", 222}, {"v3", 33}},
			[]testVal{{"v2", 22}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}}},
		7: { // multi
			[]testVal{{"v1", 111}, {"v2", 222}, {"v3", 333}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}}},
		// additions and changes
		8: {
			[]testVal{{"v1", 111}, {"v2", 22}},
			[]testVal{{"v1", 11}, {"v3", 33}, {"v4", 44}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}, {"v4", 44}}},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new validator set with the start values
		valSet := createNewValidatorSet(tt.startVals)

		// applyUpdates() with the update values
		valList := createNewValidatorList(tt.updateVals)
		valSet.applyUpdates(valList)

		// check the new list of validators for proper merge
		assert.Equal(t, toTestValList(valSet.Validators), tt.expectedVals, "test %v", i)
	}
}

func permutation(valList []testVal) []testVal {
	if len(valList) == 0 {
		return nil
	}
	permList := make([]testVal, len(valList))
	perm := krand.Perm(len(valList))
	for i, v := range perm {
		permList[v] = valList[i]
	}
	return permList
}

func toTestValList(valList []*Validator) []testVal {
	testList := make([]testVal, len(valList))
	zeroB := []byte{0x0}
	for i, val := range valList {
		zeroLength := 0
		for _, b := range val.Address.Bytes() {
			zeroLength++
			if b != zeroB[0] {
				break
			}
		}
		testList[i].name = string(val.Address[zeroLength-1:])
		testList[i].power = int64(val.VotingPower)
	}
	return testList
}

func createNewValidatorList(testValList []testVal) []*Validator {
	valList := make([]*Validator, 0, len(testValList))
	for _, val := range testValList {
		valList = append(valList, newValidator(common.BytesToAddress([]byte(val.name)), val.power))
	}
	return valList
}

func createNewValidatorSet(testValList []testVal) *ValidatorSet {
	return NewValidatorSet(createNewValidatorList(testValList))
}

func executeValSetErrTestCase(t *testing.T, idx int, tt valSetErrTestCase) {
	// create a new set and apply updates, keeping copies for the checks
	valSet := createNewValidatorSet(tt.startVals)
	fmt.Println(valSet.totalVotingPower)
	valSetCopy := valSet.Copy()
	valList := createNewValidatorList(tt.updateVals)
	valListCopy := validatorListCopy(valList)
	err := valSet.UpdateWithChangeSet(valList)

	// for errors check the validator set has not been changed
	assert.Error(t, err, "test %d", idx)
	assert.Equal(t, valSet, valSetCopy, "test %v", idx)

	// check the parameter list has not changed
	assert.Equal(t, valList, valListCopy, "test %v", idx)
}

type testVSetCfg struct {
	name         string
	startVals    []testVal
	deletedVals  []testVal
	updatedVals  []testVal
	addedVals    []testVal
	expectedVals []testVal
	expErr       error
}

func randTestVSetCfg(t *testing.T, nBase, nAddMax int) testVSetCfg {
	if nBase <= 0 || nAddMax < 0 {
		panic(fmt.Sprintf("bad parameters %v %v", nBase, nAddMax))
	}

	const maxPower = 1000
	var nOld, nDel, nChanged, nAdd int

	nOld = int(krand.Uint()%uint(nBase)) + 1
	if nBase-nOld > 0 {
		nDel = int(krand.Uint() % uint(nBase-nOld))
	}
	nChanged = nBase - nOld - nDel

	if nAddMax > 0 {
		nAdd = krand.Int()%nAddMax + 1
	}

	cfg := testVSetCfg{}

	cfg.startVals = make([]testVal, nBase)
	cfg.deletedVals = make([]testVal, nDel)
	cfg.addedVals = make([]testVal, nAdd)
	cfg.updatedVals = make([]testVal, nChanged)
	cfg.expectedVals = make([]testVal, nBase-nDel+nAdd)

	for i := 0; i < nBase; i++ {
		cfg.startVals[i] = testVal{fmt.Sprintf("v%d", i), int64(krand.Uint()%maxPower + 1)}
		if i < nOld {
			cfg.expectedVals[i] = cfg.startVals[i]
		}
		if i >= nOld && i < nOld+nChanged {
			cfg.updatedVals[i-nOld] = testVal{fmt.Sprintf("v%d", i), int64(krand.Uint()%maxPower + 1)}
			cfg.expectedVals[i] = cfg.updatedVals[i-nOld]
		}
		if i >= nOld+nChanged {
			cfg.deletedVals[i-nOld-nChanged] = testVal{fmt.Sprintf("v%d", i), 0}
		}
	}

	for i := nBase; i < nBase+nAdd; i++ {
		cfg.addedVals[i-nBase] = testVal{fmt.Sprintf("v%d", i), int64(krand.Uint()%maxPower + 1)}
		cfg.expectedVals[i-nDel] = cfg.addedVals[i-nBase]
	}

	sort.Sort(testValsByVotingPower(cfg.startVals))
	sort.Sort(testValsByVotingPower(cfg.deletedVals))
	sort.Sort(testValsByVotingPower(cfg.updatedVals))
	sort.Sort(testValsByVotingPower(cfg.addedVals))
	sort.Sort(testValsByVotingPower(cfg.expectedVals))

	return cfg

}

type testValsByVotingPower []testVal

func (tvals testValsByVotingPower) Len() int {
	return len(tvals)
}

func (tvals testValsByVotingPower) Less(i, j int) bool {
	if tvals[i].power == tvals[j].power {
		return bytes.Compare([]byte(tvals[i].name), []byte(tvals[j].name)) == -1
	}
	return tvals[i].power > tvals[j].power
}

func (tvals testValsByVotingPower) Swap(i, j int) {
	tvals[i], tvals[j] = tvals[j], tvals[i]
}

func TestValSetUpdatePriorityOrderTests(t *testing.T) {
	const nMaxElections int32 = 5000

	testCases := []testVSetCfg{
		0: { // remove high power validator, keep old equal lower power validators
			startVals:    []testVal{{"v3", 1000}, {"v1", 1}, {"v2", 1}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{},
			addedVals:    []testVal{},
			expectedVals: []testVal{{"v1", 1}, {"v2", 1}},
		},
		1: { // remove high power validator, keep old different power validators
			startVals:    []testVal{{"v3", 1000}, {"v2", 10}, {"v1", 1}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{},
			addedVals:    []testVal{},
			expectedVals: []testVal{{"v2", 10}, {"v1", 1}},
		},
		2: { // remove high power validator, add new low power validators, keep old lower power
			startVals:    []testVal{{"v3", 1000}, {"v2", 2}, {"v1", 1}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{{"v2", 1}},
			addedVals:    []testVal{{"v5", 50}, {"v4", 40}},
			expectedVals: []testVal{{"v5", 50}, {"v4", 40}, {"v1", 1}, {"v2", 1}},
		},

		// generate a configuration with 100 validators,
		// randomly select validators for updates and deletes, and
		// generate 10 new validators to be added

		// 3: randTestVSetCfg(t, 100, 10),

		// 4: randTestVSetCfg(t, 1000, 100),

		// 5: randTestVSetCfg(t, 10, 100),

		// 6: randTestVSetCfg(t, 100, 1000),

		// 7: randTestVSetCfg(t, 1000, 1000),
	}

	for _, cfg := range testCases {

		// create a new validator set
		valSet := createNewValidatorSet(cfg.startVals)
		verifyValidatorSet(t, valSet)

		// run election up to nMaxElections times, apply changes and verify that the priority order is correct
		verifyValSetUpdatePriorityOrder(t, valSet, cfg, nMaxElections)
	}
}

func verifyValSetUpdatePriorityOrder(t *testing.T, valSet *ValidatorSet, cfg testVSetCfg, nMaxElections int32) {
	// Run election up to nMaxElections times, sort validators by priorities
	valSet.IncrementProposerPriority(int64(krand.Int31()%nMaxElections + 1))

	// apply the changes, get the updated validators, sort by priorities
	applyChangesToValSet(t, nil, valSet, cfg.addedVals, cfg.updatedVals, cfg.deletedVals)

	// basic checks
	assert.Equal(t, cfg.expectedVals, toTestValList(valSet.Validators))
	verifyValidatorSet(t, valSet)

	// verify that the added validators have the smallest priority:
	//  - they should be at the beginning of updatedValsPriSorted since it is
	//  sorted by priority
	if len(cfg.addedVals) > 0 {
		updatedValsPriSorted := validatorListCopy(valSet.Validators)
		sort.Sort(validatorsByPriority(updatedValsPriSorted))

		addedValsPriSlice := updatedValsPriSorted[:len(cfg.addedVals)]
		sort.Sort(ValidatorsByVotingPower(addedValsPriSlice))
		assert.Equal(t, cfg.addedVals, toTestValList(addedValsPriSlice))

		//  - and should all have the same priority
		expectedPri := addedValsPriSlice[0].ProposerPriority
		for _, val := range addedValsPriSlice[1:] {
			assert.Equal(t, expectedPri, val.ProposerPriority)
		}
	}
}

func applyChangesToValSet(t *testing.T, expErr error, valSet *ValidatorSet, valsLists ...[]testVal) {
	changes := make([]testVal, 0)
	for _, valsList := range valsLists {
		changes = append(changes, valsList...)
	}
	valList := createNewValidatorList(changes)
	err := valSet.UpdateWithChangeSet(valList)
	if expErr != nil {
		assert.Equal(t, expErr, err)
	} else {
		assert.NoError(t, err)
	}
}

//---------------------
// Sort validators by priority and address
type validatorsByPriority []*Validator

func (valz validatorsByPriority) Len() int {
	return len(valz)
}

func (valz validatorsByPriority) Less(i, j int) bool {
	if valz[i].ProposerPriority < valz[j].ProposerPriority {
		return true
	}
	if valz[i].ProposerPriority > valz[j].ProposerPriority {
		return false
	}
	return bytes.Compare(valz[i].Address.Bytes(), valz[j].Address.Bytes()) < 0
}

func (valz validatorsByPriority) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}
