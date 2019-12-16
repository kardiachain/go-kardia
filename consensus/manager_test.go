package consensus

import (
	"sort"
	"time"

	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/types"
)

func randGenesisDoc(numValidators int, randPower bool, minPower int64) (*genesis.Genesis, []types.PrivValidator) {
	validators := make([]*types.Validator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = &types.Validator{
			Address:     val.Address,
			PubKey:      val.PubKey,
			VotingPower: val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &genesis.Genesis{
		Timestamp:  uint64(time.Now().Unix()),
		Validators: validators,
	}, privValidators
}
