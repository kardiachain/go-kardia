package evidence

import (
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"

	"github.com/kardiachain/go-kardia/kai/state/cstate"
	smocks "github.com/kardiachain/go-kardia/kai/state/cstate/mocks"
	"github.com/kardiachain/go-kardia/lib/common"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(1)})

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	v1 := vote1.ToProto()
	err := val.SignVote(chainID, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, bv)
	require.NoError(t, err)

	vote1.Signature = v1.Signature
	badVote.Signature = bv.Signature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2, defaultEvidenceTime), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2, defaultEvidenceTime), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2, defaultEvidenceTime), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2, defaultEvidenceTime), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), false},   // wrong validator
		// a different vote time doesn't matter
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), true},
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA:            c.vote1,
			VoteB:            c.vote2,
			ValidatorPower:   1,
			TotalVotingPower: 1,
			Timestamp:        defaultEvidenceTime,
		}
		if c.valid {
			assert.Nil(t, VerifyDuplicateVote(ev, chainID, valSet), "evidence should be valid")
		} else {
			assert.NotNil(t, VerifyDuplicateVote(ev, chainID, valSet), "evidence should be invalid")
		}
	}

	goodEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID)
	goodEv.ValidatorPower = 1
	goodEv.TotalVotingPower = 1
	badEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID)
	badTimeEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime.Add(1*time.Minute), val, chainID)
	badTimeEv.ValidatorPower = 1
	badTimeEv.TotalVotingPower = 1

	state := cstate.LatestBlockState{
		ChainID:         chainID,
		InitialHeight:   1,
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smocks.Store{}
	stateStore.On("LoadValidators", uint64(10)).Return(valSet, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", uint64(10)).Return(&types.BlockMeta{Header: &types.Header{Time: defaultEvidenceTime}})

	pool, err := NewPool(stateStore, memorydb.New(), blockStore)
	require.NoError(t, err)

	evList := types.EvidenceList{goodEv}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)

	// evidence with a different validator power should fail
	evList = types.EvidenceList{badEv}
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)

	// evidence with a different timestamp should fail
	evList = types.EvidenceList{badTimeEv}
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)
}

func makeVote(
	t *testing.T, val types.PrivValidator, chainID string, valIndex uint32, height uint64,
	round uint32, step int, blockID types.BlockID, time time.Time) *types.Vote {

	v := &types.Vote{
		ValidatorAddress: val.GetAddress(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             kproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err := val.SignVote(chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {

	return types.BlockID{
		Hash: common.BytesToHash(hash),
		PartsHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  common.BytesToHash(partSetHash),
		},
	}
}
