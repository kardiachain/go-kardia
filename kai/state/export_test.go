package state

import (
	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/types"
)

const ValSetCheckpointInterval = valSetCheckpointInterval

// SaveValidatorsInfo is an alias for the private saveValidatorsInfo method in
// store.go, exported exclusively and explicitly for testing.
func SaveValidatorsInfo(db kaidb.Database, height, lastHeightChanged uint64, valSet *types.ValidatorSet) {
	saveValidatorsInfo(db, height, lastHeightChanged, valSet)
}
