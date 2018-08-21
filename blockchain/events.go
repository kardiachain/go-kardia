package blockchain

import "github.com/kardiachain/go-kardia/types"

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*types.Transaction }

// ChainHeadEvent is posted when a new head block is saved to the block chain.
type ChainHeadEvent struct{ Block *types.Block }
