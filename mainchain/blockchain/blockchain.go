/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package blockchain

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/permissioned"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	blockCacheLimit = 256

	maxFutureBlocks = 256
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

// TODO(huny@): Add detailed description for Kardia blockchain
type BlockChain struct {
	logger log.Logger

	chainConfig *configs.ChainConfig // Chain & network configuration

	db types.StoreDB // Blockchain database
	hc *HeaderChain

	chainHeadFeed event.Feed
	scope         event.SubscriptionScope

	genesisBlock *types.Block

	mu sync.RWMutex // global mutex for locking chain operations

	currentBlock atomic.Value // Current head of the block chain

	stateCache   state.Database // State database to reuse between imports (contains state cache)
	blockCache   *lru.Cache     // Cache for the most recent entire blocks
	futureBlocks *lru.Cache     // future blocks are blocks added for later processing

	quit chan struct{} // blockchain quit channel

	processor *StateProcessor // block processor
	vmConfig  kvm.Config      // vm configurations

	// isPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	isPrivate bool

	// permissioned is used to call permissioned smartcontract to check whether a node has permission to access chain or not
	permissioned *permissioned.PermissionSmcUtil
}

func (bc *BlockChain) P2P() *configs.P2PConfig {
	return bc.P2P()
}

// GetVMConfig returns the block chain VM config.
func (bc *BlockChain) GetVMConfig() *kvm.Config {
	return &bc.vmConfig
}

// IsPrivate returns whether a blockchain is private or not
func (bc *BlockChain) IsPrivate() bool {
	return bc.isPrivate
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

func (bc *BlockChain) Processor() *StateProcessor {
	return bc.processor
}

func (bc *BlockChain) DB() types.StoreDB {
	return bc.db
}

// Config retrieves the blockchain's chain configuration.
func (bc *BlockChain) Config() *configs.ChainConfig { return bc.chainConfig }

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Kardia Validator and Processor.
func NewBlockChain(logger log.Logger, db types.StoreDB, chainConfig *configs.ChainConfig, isPrivate bool) (*BlockChain, error) {
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)

	bc := &BlockChain{
		logger:       logger,
		chainConfig:  chainConfig,
		db:           db,
		stateCache:   state.NewDatabase(db.DB()),
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
		quit:         make(chan struct{}),
		isPrivate:    isPrivate,
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

	if err := bc.loadLastState(); err != nil {
		return nil, err
	}

	// Take ownership of this particular state
	//@huny go bc.update()

	bc.processor = NewStateProcessor(logger, bc)
	// TODO @trinhdn: Remove this when clean up repo
	// bc.permissioned, err = permissioned.NewSmcPermissionUtil(bc)
	// if err != nil {
	// 	return nil, err
	// }

	return bc, nil
}

// GetBlockByHeight retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByHeight(height uint64) *types.Block {
	hash := bc.db.ReadCanonicalHash(height)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, height)
}

// LoadBlockPart ...
func (bc *BlockChain) LoadBlockPart(height uint64, index int) *types.Part {
	return bc.db.ReadBlockPart(height, index)
}

// LoadBlockMeta ...
func (bc *BlockChain) LoadBlockMeta(height uint64) *types.BlockMeta {
	return bc.db.ReadBlockMeta(height)
}

// LoadBlockCommit ...
func (bc *BlockChain) LoadBlockCommit(height uint64) *types.Commit {
	return bc.db.ReadCommit(height)
}

// LoadSeenCommit ...
func (bc *BlockChain) LoadSeenCommit(height uint64) *types.Commit {
	return bc.db.ReadSeenCommit(height)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := bc.db.ReadBlock(hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}

// GetHeader retrieves a block header from the database by hash and height,
// caching it if found.
func (bc *BlockChain) GetHeader(hash common.Hash, height uint64) *types.Header {
	return bc.hc.GetHeader(hash, height)
}

// State returns a new mutatable state at head block.
func (bc *BlockChain) State() (*state.StateDB, error) {
	return bc.StateAt(bc.CurrentBlock().Height())
}

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(height uint64) (*state.StateDB, error) {
	root := bc.DB().ReadAppHash(height)
	return state.New(bc.logger, root, bc.stateCache)
}

// CheckCommittedStateRoot returns true if the given state root is already committed and existed on trie database.
func (bc *BlockChain) CheckCommittedStateRoot(root common.Hash) bool {
	// TODO(thientn): Adds check trie function instead of using error handler as expected logic path.
	// Currently OpenTrie tries to load a trie obj from the memory cache and then trie db, return error if not found.
	_, err := bc.stateCache.OpenTrie(root)
	return err == nil
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChain) loadLastState() error {
	// Restore the last known head block
	hash := bc.db.ReadHeadBlockHash()
	if hash == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		bc.logger.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(hash)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		bc.logger.Warn("Head block missing, resetting chain", "hash", hash)
		return bc.Reset()
	}
	root := bc.DB().ReadAppHash(currentBlock.Height())
	// Make sure the state associated with the block is available
	if _, err := state.New(bc.logger, root, bc.stateCache); err != nil {
		// Dangling block without a state associated, init from scratch
		bc.logger.Warn("Head state missing, repairing chain", "height", currentBlock.Height(), "hash", currentBlock.Hash())
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	bc.hc.SetCurrentHeader(currentHeader)

	bc.logger.Info("Loaded most recent local header", "height", currentHeader.Height, "hash", currentHeader.Hash())
	bc.logger.Info("Loaded most recent local full block", "height", currentBlock.Height(), "hash", currentBlock.Hash())

	return nil
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *BlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *BlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.db.WriteBlock(genesis, genesis.MakePartSet(types.BlockPartSizeBytes), &types.Commit{})

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *BlockChain) repair(head **types.Block) error {
	for {
		root := bc.DB().ReadAppHash((*head).Height())
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New(bc.logger, root, bc.stateCache); err == nil {
			bc.logger.Info("Rewound blockchain to past state", "height", (*head).Height(), "hash", (*head).Hash())
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		lastCommitHash := (*head).LastCommitHash()
		lastHeight := (*head).Height() - 1
		block := bc.GetBlock(lastCommitHash, lastHeight)
		if block == nil {
			return fmt.Errorf("Missing block height: %d [%x]", lastHeight, lastCommitHash)
		}
		*head = block
	}
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	height := bc.hc.GetBlockHeight(hash)
	if height == nil {
		return nil
	}
	return bc.GetBlock(hash, *height)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return bc.hc.GetHeaderByHash(hash)
}

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *BlockChain) SetHead(head uint64) error {
	bc.logger.Warn("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db types.StoreDB, height uint64) {
		db.DeleteBlockPart(height)
	}
	bc.hc.SetHead(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Height < currentBlock.Height() {
		bc.currentBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Height))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		root := bc.DB().ReadAppHash(currentBlock.Height())
		if _, err := state.New(bc.logger, root, bc.stateCache); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}

	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}

	return bc.loadLastState()
}

// InsertHeadBlock inserts new head block to blockchain and send new head event.
// This function assumes block transactions & app hash are already committed.
func (bc *BlockChain) InsertHeadBlock(block *types.Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.insert(block)
	bc.chainHeadFeed.Send(events.ChainHeadEvent{Block: block})
}

// WriteReceipts writes the transactions receipt from execution of the transactions in the given block.
func (bc *BlockChain) WriteBlockInfo(block *types.Block, blockInfo *types.BlockInfo) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.db.WriteBlockInfo(block.Hash(), block.Header().Height, blockInfo)
}

// CommitTrie commits trie node such as statedb forcefully to disk.
func (bc BlockChain) CommitTrie(root common.Hash) error {
	triedb := bc.stateCache.TrieDB()
	return triedb.Commit(root, false)
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChain) insert(block *types.Block) {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := bc.db.ReadCanonicalHash(block.Height()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	bc.db.WriteCanonicalHash(block.Hash(), block.Height())

	bc.currentBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
	}
}

// Reads commit from db.
func (bc *BlockChain) ReadCommit(height uint64) *types.Commit {
	return bc.db.ReadCommit(height)
}

func (bc *BlockChain) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	bc.db.WriteBlock(block, blockParts, seenCommit)
}

func (bc *BlockChain) ApplyMessage(vm *kvm.KVM, msg types.Message, gp *types.GasPool) (*kvm.ExecutionResult, error) {
	return ApplyMessage(vm, msg, gp)
}
