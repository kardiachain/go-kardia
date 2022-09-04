package trace

import (
	"context"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type Backend interface {
	BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block
	BlockByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Block, error)
	BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo
	Config() *configs.ChainConfig
	GetHeader(hash common.Hash, height uint64) *types.Header
	HeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Header, error)
	ReadCanonicalHash(ctx context.Context, height uint64) common.Hash
	ReadHeadBlockHash(ctx context.Context) common.Hash
	ReadHeaderHeight(ctx context.Context, hash common.Hash) uint64
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.BlockContext, *state.StateDB, error)
	TxnLookup(ctx context.Context, txHash common.Hash) (uint64, bool)
}
