package core

import (
	"sync/atomic"

	"github.com/kardiachain/go-kardia/params"
	"github.com/kardiachain/go-kardia/types"
)

// TODO(huny@): Add detailed description for Kardia blockchain
type BlockChain struct {
	chainConfig *params.ChainConfig // Chain & network configuration

	hc *HeaderChain

	genesisBlock *types.Block

	currentBlock atomic.Value // Current head of the block chain
}

// Genesis retrieves the chain's genesis block.
func (bc *BlockChain) Genesis() *types.Block {
	return bc.genesisBlock
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (bc *BlockChain) CurrentHeader() *types.Header {
	return bc.hc.CurrentHeader()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// Config retrieves the blockchain's chain configuration.
func (bc *BlockChain) Config() *params.ChainConfig { return bc.chainConfig }
