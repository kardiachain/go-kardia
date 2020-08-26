package evidence

import (
	"math/big"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

func initializeValidatorState(valAddr common.Address, height int64) kaidb.Database {
	stateDB := memorydb.New()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr},
		},
	}

	nextVal := valSet.Copy()
	nextVal.AdvanceProposer(1)

	state := state.LastestBlockState{
		LastBlockHeight:             common.NewBigInt64(0),
		LastBlockTime:               big.NewInt(time.Now().Unix()),
		Validators:                  valSet,
		NextValidators:              nextVal,
		LastHeightValidatorsChanged: common.NewBigInt32(1),
		ConsensusParams: types.ConsensusParams{
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 10000,
				MaxAgeDuration:  48 * time.Hour,
			},
		},
	}
	// save all states up to height
	for i := int64(0); i < height; i++ {
		state.LastBlockHeight = common.NewBigInt64(i)
		state.SaveState(stateDB, state)
	}

	return stateDB
}
