package types

import (
	"math"
	"testing"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type voteData struct {
	vote1 *Vote
	vote2 *Vote
	valid bool
}

func makeVote(val PrivV, chainID string, valIndex int, height int64, round, step int, blockID BlockID) *Vote {
	addr := crypto.PubkeyToAddress(val.GetPubKey())
	v := &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   common.NewBigInt32(valIndex),
		Height:           common.NewBigInt64(height),
		Round:            common.NewBigInt32(round),
		Type:             SignedMsgType(step),
		BlockID:          blockID,
	}

	err := val.SignVote(chainID, v)
	if err != nil {
		panic(err)
	}
	return v
}

func TestEvidence(t *testing.T) {
	val := NewMockPV()
	val2 := NewMockPV()

	blockID := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt32(1000), common.BytesToHash([]byte("partshash")))
	blockID2 := makeBlockID(common.BytesToHash([]byte("blockhash2")), *common.NewBigInt32(1000), common.BytesToHash([]byte("partshash")))
	blockID3 := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt32(10000), common.BytesToHash([]byte("partshash")))
	blockID4 := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt32(10000), common.BytesToHash([]byte("partshash2")))

	const chainID = "mychain"

	vote1 := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	val.SignVote(chainID, vote1)

	badVote := makeVote(val, chainID, 0, 10, 2, 1, blockID)
	err := val2.SignVote(chainID, badVote)
	if err != nil {
		panic(err)
	}
	cases := []voteData{
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID2), true}, // different block ids
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID3), true},
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID4), true},
		{vote1, makeVote(val, chainID, 0, 10, 2, 1, blockID), false},     // wrong block id
		{vote1, makeVote(val, "mychain2", 0, 10, 2, 1, blockID2), false}, // wrong chain id
		{vote1, makeVote(val, chainID, 1, 10, 2, 1, blockID2), false},    // wrong val index
		{vote1, makeVote(val, chainID, 0, 11, 2, 1, blockID2), false},    // wrong height
		{vote1, makeVote(val, chainID, 0, 10, 3, 1, blockID2), false},    // wrong round
		{vote1, makeVote(val, chainID, 0, 10, 2, 2, blockID2), false},    // wrong step
		{vote1, makeVote(val2, chainID, 0, 10, 2, 1, blockID), false},    // wrong validator
		{vote1, badVote, false}, // signed by wrong key
	}

	pubKey := val.GetPubKey()
	for _, c := range cases {
		ev := &DuplicateVoteEvidence{
			VoteA: c.vote1,
			VoteB: c.vote2,
		}
		if c.valid {
			assert.Nil(t, ev.Verify(chainID, pubKey), "evidence should be valid")
		} else {
			assert.NotNil(t, ev.Verify(chainID, pubKey), "evidence should be invalid")
		}
	}
}

func TestDuplicatedVoteEvidence(t *testing.T) {
	ev := randomDuplicatedVoteEvidence()

	assert.True(t, ev.Equal(ev))
	assert.False(t, ev.Equal(&DuplicateVoteEvidence{}))
}

func TestEvidenceList(t *testing.T) {
	ev := randomDuplicatedVoteEvidence()
	evl := EvidenceList([]Evidence{ev})

	assert.NotNil(t, evl.Hash())
	assert.True(t, evl.Has(ev))
	assert.False(t, evl.Has(&DuplicateVoteEvidence{}))
}

func TestMaxEvidenceBytes(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt64(math.MaxInt64), common.BytesToHash([]byte("partshash")))
	blockID2 := makeBlockID(common.BytesToHash([]byte("blockhash2")), *common.NewBigInt64(math.MaxInt64), common.BytesToHash([]byte("partshash")))
	const chainID = "mychain"

	ev := &DuplicateVoteEvidence{
		PubKey: val.GetPubKey(), // use secp because it's pubkey is longer
		VoteA:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID),
		VoteB:  makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, blockID2),
	}
	bz, err := rlp.EncodeToBytes(ev)
	require.NoError(t, err)

	assert.EqualValues(t, MaxEvidenceBytes, len(bz))
}

func randomDuplicatedVoteEvidence() *DuplicateVoteEvidence {
	val := NewMockPV()
	blockID := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt64(1000), common.BytesToHash([]byte("partshash")))
	blockID2 := makeBlockID(common.BytesToHash([]byte("blockhash2")), *common.NewBigInt64(1000), common.BytesToHash([]byte("partshash")))
	const chainID = "mychain"
	return &DuplicateVoteEvidence{
		PubKey: val.GetPubKey(),
		VoteA:  makeVote(val, chainID, 0, 10, 2, 1, blockID),
		VoteB:  makeVote(val, chainID, 0, 10, 2, 1, blockID2),
	}
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID(common.BytesToHash([]byte("blockhash")), *common.NewBigInt64(math.MaxInt64), common.BytesToHash([]byte("partshash")))
	blockID2 := makeBlockID(common.BytesToHash([]byte("blockhash2")), *common.NewBigInt64(math.MaxInt64), common.BytesToHash([]byte("partshash")))
	const chainID = "mychain"

	testCases := []struct {
		testName         string
		malleateEvidence func(*DuplicateVoteEvidence)
		expectErr        bool
	}{
		{"Good DuplicateVoteEvidence", func(ev *DuplicateVoteEvidence) {}, false},
		{"Nil vote A", func(ev *DuplicateVoteEvidence) { ev.VoteA = nil }, true},
		{"Nil vote B", func(ev *DuplicateVoteEvidence) { ev.VoteB = nil }, true},
		{"Nil votes", func(ev *DuplicateVoteEvidence) {
			ev.VoteA = nil
			ev.VoteB = nil
		}, true},
		{"Invalid vote type", func(ev *DuplicateVoteEvidence) {
			ev.VoteA = makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 100, blockID2)
		}, true},
		{"Invalid vote order", func(ev *DuplicateVoteEvidence) {
			swap := ev.VoteA.Copy()
			ev.VoteA = ev.VoteB.Copy()
			ev.VoteB = swap
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			pk, err := crypto.GenerateKey()
			assert.NoError(t, err)
			vote1 := makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x01, blockID)
			vote2 := makeVote(val, chainID, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0x01, blockID2)
			ev := NewDuplicateVoteEvidence(pk.PublicKey, vote1, vote2)
			tc.malleateEvidence(ev)
			assert.Equal(t, tc.expectErr, ev.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMockGoodEvidenceValidateBasic(t *testing.T) {
	goodEvidence := NewMockGoodEvidence(common.NewBigInt64(1), 1, common.BytesToAddress([]byte{1}))
	assert.Nil(t, goodEvidence.ValidateBasic())
}

func TestMockBadEvidenceValidateBasic(t *testing.T) {
	badEvidence := MockBadEvidence{MockGoodEvidence: NewMockGoodEvidence(common.NewBigInt64(1), 1, common.BytesToAddress([]byte{1}))}
	assert.Nil(t, badEvidence.ValidateBasic())
}
