package trace

import (
	"context"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type Backend interface {
	BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block
	BlockByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Block, error)
	Config() *configs.ChainConfig
	GetHeader(hash common.Hash, height uint64) *types.Header
	HeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Header, error)
	TxnLookup(ctx context.Context, txHash common.Hash) (uint64, bool)
}
