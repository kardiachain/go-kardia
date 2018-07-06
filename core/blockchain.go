package core

import (
	"errors"

	"sync/atomic"

	"github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/core/rawdb"
	kaidb "github.com/kardiachain/go-kardia/database"
	"github.com/kardiachain/go-kardia/params"
	"github.com/kardiachain/go-kardia/types"
)

const (
	blockCacheLimit = 256
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

// TODO(huny@): Add detailed description for Kardia blockchain
type BlockChain struct {
	chainConfig *params.ChainConfig // Chain & network configuration

	db kaidb.Database // Blockchain database
	hc *HeaderChain

	genesisBlock *types.Block

	currentBlock atomic.Value // Current head of the block chain

	blockCache *lru.Cache // Cache for the most recent entire blocks

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

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Ethereum Validator and
// Processor.
func NewBlockChain(db kaidb.Database, chainConfig *params.ChainConfig) (*BlockChain, error) {
	blockCache, _ := lru.New(blockCacheLimit)

	bc := &BlockChain{
		chainConfig: chainConfig,
		db:          db,
		blockCache:  blockCache,
	}

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByHeight(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	// TODO(huny@): Fully initialization required loadLastState and take ownership
	/*
		if err := bc.loadLastState(); err != nil {
			return nil, err
		}

		// Take ownership of this particular state
		go bc.update()
	*/
	return bc, nil
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByHeight(height uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(bc.db, height)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, height)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := rawdb.ReadBlock(bc.db, hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}
