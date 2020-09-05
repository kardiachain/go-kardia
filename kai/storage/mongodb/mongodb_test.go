package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

func TestToBlock(t *testing.T) {
	b := types.NewBlock(&types.Header{
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: *common.NewBigInt64(0),
			},
		},
	}, nil, nil, nil)
	mgoB := NewBlock(b).ToBlock()
	assert.Equal(t, mgoB.Hash(), b.Hash())

}
