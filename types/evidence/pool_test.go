package evidence

import (
	"sync"
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	kstate "github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
)

func initializeValidatorState(valAddr common.Address, height int64) kaidb.Database {
	stateDB := memorydb.New()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			&types.Validator{
				Address:     valAddr,
				VotingPower: 100,
				Accum:       common.NewBigInt64(1),
			},
		},
	}

	nextVal := valSet.Copy()
	nextVal.AdvanceProposer(1)

	state := state.LastestBlockState{
		LastBlockHeight:             common.NewBigInt64(0),
		LastBlockTime:               uint64(time.Now().Unix()),
		Validators:                  valSet,
		NextValidators:              nextVal,
		LastHeightValidatorsChanged: common.NewBigInt64(1),
		ConsensusParams: types.ConsensusParams{
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 10000,
				MaxAgeDuration:  48 * 60 * 60,
			},
		},
	}
	// save all states up to height
	for i := int64(0); i < height; i++ {
		state.LastBlockHeight = common.NewBigInt64(i)
		kstate.SaveState(stateDB, state)
	}

	return stateDB
}

func TestEvidencePool(t *testing.T) {

	var (
		valAddr      = common.BytesToAddress([]byte("val1"))
		height       = int64(100002)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = memorydb.New()
		pool         = NewPool(stateDB, evidenceDB)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	goodEvidence := types.NewMockEvidence(uint64(height), time.Now(), 0, valAddr)
	badEvidence := types.NewMockEvidence(1, evidenceTime, 0, valAddr)

	// bad evidence
	err := pool.AddEvidence(badEvidence)
	assert.Error(t, err)
	// err: evidence created at 2019-01-01 00:00:00 +0000 UTC has expired. Evidence can not be older than: ...

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-pool.EvidenceWaitChan()
		wg.Done()
	}()

	err = pool.AddEvidence(goodEvidence)
	assert.Nil(t, err)
	wg.Wait()

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt change the size
	err = pool.AddEvidence(goodEvidence)
	assert.Error(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}

func TestEvidencePoolIsCommitted(t *testing.T) {
	// Initialization:
	var (
		valAddr       = common.BytesToAddress([]byte("validator_address"))
		height        = int64(42)
		lastBlockTime = int64(time.Now().Unix())
		stateDB       = initializeValidatorState(valAddr, height)
		evidenceDB    = memorydb.New()
		pool          = NewPool(stateDB, evidenceDB)
	)

	// evidence not seen yet:
	evidence := types.NewMockEvidence(uint64(height), time.Now(), 0, valAddr)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, time.Unix(lastBlockTime, 0), []types.Evidence{evidence})
	assert.True(t, pool.IsCommitted(evidence))
}

func TestAddEvidence(t *testing.T) {

	var (
		valAddr      = common.BytesToAddress([]byte("val1"))
		height       = uint64(100002)
		stateDB      = initializeValidatorState(valAddr, int64(height))
		evidenceDB   = memorydb.New()
		pool         = NewPool(stateDB, evidenceDB)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	testCases := []struct {
		evHeight      uint64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, time.Now(), false, "valid evidence"},
		{uint64(1), evidenceTime, true,
			"evidence from height 1 is too old & evidence created at 2019-01-01 00:00:00 +0000 UTC has expired"},
	}

	for _, tc := range testCases {
		tc := tc
		ev := types.NewMockEvidence(tc.evHeight, tc.evTime, 0, valAddr)
		err := pool.AddEvidence(ev)
		if tc.expErr {
			assert.Error(t, err)
		}
	}
}
