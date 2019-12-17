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
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/pos"
	"math/big"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

const (
	blockCacheLimit = 256

	maxFutureBlocks     = 256
	maxTimeFutureBlocks = 30
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

// A blockchain to store events from external blockchains (e.g. Ether, Neo, etc.) or internal Karida's blockchain and
// associating transactions to be submitted to other blockchains.
type DualBlockChain struct {
	logger log.Logger

	chainConfig *types.ChainConfig // Chain & network configuration

	db types.StoreDB // Kai's database
	hc *DualHeaderChain

	chainHeadFeed event.Feed
	scope         event.SubscriptionScope

	genesisBlock *types.Block

	mu sync.RWMutex // global mutex for locking chain operations

	currentBlock atomic.Value // Current head of the block chain

	stateCache   state.Database // State database to reuse between imports (contains state cache)
	blockCache   *lru.Cache     // Cache for the most recent entire blocks
	futureBlocks *lru.Cache     // future blocks are blocks added for later processing

	quit chan struct{} // blockchain quit channel
}

// Genesis retrieves the chain's genesis block.
func (dbc *DualBlockChain) Genesis() *types.Block {
	return dbc.genesisBlock
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (dbc *DualBlockChain) CurrentHeader() *types.Header {
	return dbc.hc.CurrentHeader()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (dbc *DualBlockChain) CurrentBlock() *types.Block {
	return dbc.currentBlock.Load().(*types.Block)
}

//func (dbc *DualBlockChain) Processor() *StateProcessor {
//	return dbc.processor
//}

func (dbc *DualBlockChain) DB() types.StoreDB {
	return dbc.db
}

// Config retrieves the blockchain's chain configuration.
func (dbc *DualBlockChain) Config() *types.ChainConfig { return dbc.chainConfig }

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Kardia Validator and Processor.
func NewBlockChain(logger log.Logger, db types.StoreDB, chainConfig *types.ChainConfig) (*DualBlockChain, error) {
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)

	dbc := &DualBlockChain{
		logger:       logger,
		chainConfig:  chainConfig,
		db:           db,
		stateCache:   state.NewDatabase(db.DB()),
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
		quit:         make(chan struct{}),
	}
	var err error

	dbc.hc, err = NewHeaderChain(db, chainConfig)
	if err != nil {
		return nil, err
	}
	dbc.genesisBlock = dbc.GetBlockByHeight(0)
	if dbc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	if err := dbc.loadLastState(); err != nil {
		return nil, err
	}
	// Take ownership of this particular state
	//@huny go dbc.update()

	return dbc, nil
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (dbc *DualBlockChain) GetBlockByHeight(height uint64) *types.Block {
	hash := dbc.db.ReadCanonicalHash(height)
	if hash == (common.Hash{}) {
		return nil
	}
	return dbc.GetBlock(hash, height)
}

func (bc *DualBlockChain) LoadBlockPart(height uint64, index int) *types.Part {
	hash := bc.db.ReadCanonicalHash(height)
	part := bc.db.ReadBlockPart(hash, height, index)
	if hash == (common.Hash{}) {
		return nil
	}
	return part
}

func (bc *DualBlockChain) LoadBlockCommit(height uint64) *types.Commit {
	return bc.db.ReadCommit(height)
}

func (bc *DualBlockChain) LoadSeenCommit(height uint64) *types.Commit {
	return bc.db.ReadSeenCommit(height)
}

//
func (bc *DualBlockChain) LoadBlockMeta(height uint64) *types.BlockMeta {
	hash := bc.db.ReadCanonicalHash(height)
	return bc.db.ReadBlockMeta(hash, height)
}

func (bc *DualBlockChain) ReadAppHash(height uint64) common.Hash {
	return bc.db.ReadAppHash(height)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (dbc *DualBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := dbc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := dbc.db.ReadBlock(hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	dbc.blockCache.Add(block.Hash(), block)
	return block
}

// GetHeader retrieves a block header from the database by hash and height,
// caching it if found.
func (dbc *DualBlockChain) GetHeader(hash common.Hash, height uint64) *types.Header {
	return dbc.hc.GetHeader(hash, height)
}

// State returns a new mutatable state at head block.
func (dbc *DualBlockChain) State() (*state.StateDB, error) {
	return dbc.StateAt(dbc.CurrentHeader().Height)
}

// StateAt returns a new mutable state based on a particular point in time.
func (dbc *DualBlockChain) StateAt(height uint64) (*state.StateDB, error) {
	appHash := dbc.db.ReadAppHash(height)
	return state.New(dbc.logger, appHash, dbc.stateCache)
}

// CheckCommittedStateRoot returns true if the given state root is already committed and existed on trie database.
func (dbc *DualBlockChain) CheckCommittedStateRoot(root common.Hash) bool {
	_, err := dbc.stateCache.OpenTrie(root)
	return err == nil
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (dbc *DualBlockChain) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return dbc.scope.Track(dbc.chainHeadFeed.Subscribe(ch))
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (dbc *DualBlockChain) loadLastState() error {
	// Restore the last known head block
	head := dbc.db.ReadHeadBlockHash()
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		dbc.logger.Warn("Empty database, resetting chain")
		return dbc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := dbc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		dbc.logger.Warn("Head block missing, resetting chain", "hash", head)
		return dbc.Reset()
	}
	// Make sure the state associated with the block is available
	if _, err := state.New(dbc.logger, dbc.ReadAppHash(currentBlock.Height()), dbc.stateCache); err != nil {
		// Dangling block without a state associated, init from scratch
		dbc.logger.Warn("Head state missing, repairing chain", "height", currentBlock.Height(), "hash", currentBlock.Hash())
		if err := dbc.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	dbc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := dbc.db.ReadHeadHeaderHash(); head != (common.Hash{}) {
		if header := dbc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	dbc.hc.SetCurrentHeader(currentHeader)

	dbc.logger.Info("Loaded most recent local header", "height", currentHeader.Height, "hash", currentHeader.Hash())
	dbc.logger.Info("Loaded most recent local full block", "height", currentBlock.Height(), "hash", currentBlock.Hash())

	return nil
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (dbc *DualBlockChain) Reset() error {
	return dbc.ResetWithGenesisBlock(dbc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (dbc *DualBlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := dbc.SetHead(0); err != nil {
		return err
	}
	dbc.mu.Lock()
	defer dbc.mu.Unlock()

	dbc.db.WriteBlock(genesis, genesis.MakePartSet(types.BlockPartSizeBytes), &types.Commit{})

	dbc.genesisBlock = genesis
	dbc.insert(dbc.genesisBlock)
	dbc.currentBlock.Store(dbc.genesisBlock)
	dbc.hc.SetGenesis(dbc.genesisBlock.Header())
	dbc.hc.SetCurrentHeader(dbc.genesisBlock.Header())

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (dbc *DualBlockChain) repair(head **types.Block) error {
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New(dbc.logger, dbc.ReadAppHash((*head).Height()), dbc.stateCache); err == nil {
			dbc.logger.Info("Rewound blockchain to past state", "height", (*head).Height(), "hash", (*head).Hash())
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		(*head) = dbc.GetBlock((*head).LastCommitHash(), (*head).Height()-1)
	}
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (dbc *DualBlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	height := dbc.hc.GetBlockHeight(hash)
	if height == nil {
		return nil
	}
	return dbc.GetBlock(hash, *height)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (dbc *DualBlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return dbc.hc.GetHeaderByHash(hash)
}

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (dbc *DualBlockChain) SetHead(head uint64) error {
	dbc.logger.Warn("Rewinding blockchain", "target", head)

	dbc.mu.Lock()
	defer dbc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db types.StoreDB, hash common.Hash, height uint64) {
		db.DeleteBlockPart(hash, height)
	}
	dbc.hc.SetHead(head, delFn)
	currentHeader := dbc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	dbc.blockCache.Purge()
	dbc.futureBlocks.Purge()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := dbc.CurrentBlock(); currentBlock != nil && currentHeader.Height < currentBlock.Height() {
		dbc.currentBlock.Store(dbc.GetBlock(currentHeader.Hash(), currentHeader.Height))
	}
	if currentBlock := dbc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(dbc.logger, dbc.ReadAppHash(currentBlock.Height()), dbc.stateCache); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			dbc.currentBlock.Store(dbc.genesisBlock)
		}
	}

	// If either blocks reached nil, reset to the genesis state
	if currentBlock := dbc.CurrentBlock(); currentBlock == nil {
		dbc.currentBlock.Store(dbc.genesisBlock)
	}

	currentBlock := dbc.CurrentBlock()

	dbc.db.WriteHeadBlockHash(currentBlock.Hash())

	return dbc.loadLastState()
}

// WriteBlockWithoutState writes only new block to database.
func (dbc *DualBlockChain) WriteBlockWithoutState(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) error {
	// Makes sure no inconsistent state is leaked during insertion
	dbc.mu.Lock()
	defer dbc.mu.Unlock()
	// Write block data in batch
	dbc.db.WriteBlock(block, blockParts, seenCommit)

	// Convert all txs into txLookupEntries and store to db
	dbc.db.WriteTxLookupEntries(block)

	// StateDb for this block should be already written.

	dbc.insert(block)
	dbc.futureBlocks.Remove(block.Hash())

	// Sends new head event
	dbc.chainHeadFeed.Send(events.ChainHeadEvent{Block: block})
	return nil
}

// WriteReceipts writes the transactions receipt from execution of the transactions in the given block.
func (dbc *DualBlockChain) WriteReceipts(receipts types.Receipts, block *types.Block) {
	dbc.mu.Lock()
	defer dbc.mu.Unlock()

	dbc.db.WriteReceipts(block.Hash(), block.Header().Height, receipts)
}

// CommitTrie commits trie node such as statedb forcefully to disk.
func (dbc *DualBlockChain) CommitTrie(root common.Hash) error {
	triedb := dbc.stateCache.TrieDB()
	return triedb.Commit(root, false)
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (dbc *DualBlockChain) insert(block *types.Block) {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := dbc.db.ReadCanonicalHash(block.Height()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	dbc.db.WriteCanonicalHash(block.Hash(), block.Height())
	dbc.db.WriteHeadBlockHash(block.Hash())

	dbc.currentBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		dbc.hc.SetCurrentHeader(block.Header())
	}
}

func (dbc *DualBlockChain) WriteBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	dbc.db.WriteBlock(block, blockParts, seenCommit)
}

// Reads commit from db.
func (dbc *DualBlockChain) ReadCommit(height uint64) *types.Commit {
	return dbc.db.ReadCommit(height)
}

// Writes a hash into db.
// TODO(namdoh@): The hashKey is primarily used for persistently store a tx hash in db, so we
// quickly check if a tx has been seen in the past. When the scope of this key extends beyond
// tx hash, it's probably cleaner to refactor this into a separate API (instead of grouping
// it under chaindb).
func (dbc *DualBlockChain) StoreHash(hash *common.Hash) {
	dbc.db.StoreHash(hash)
}

// Returns true if a hash already exists.
// TODO(namdoh@): The hashKey is primarily used for persistently store a tx hash in db, so we
// quickly check if a tx has been seen in the past. When the scope of this key extends beyond
// tx hash, it's probably cleaner to refactor this into a separate API (instead of grouping
// it under chaindb).
func (dbc *DualBlockChain) CheckHash(hash *common.Hash) bool {
	return dbc.db.CheckHash(hash)
}

// Writes a tx hash into db.
// TODO(namdoh@): The hashKey is primarily used for persistently store a tx hash in db, so we
// quickly check if a tx has been seen in the past. When the scope of this key extends beyond
// tx hash, it's probably cleaner to refactor this into a separate API (instead of grouping
// it under chaindb).
func (dbc *DualBlockChain) StoreTxHash(hash *common.Hash) {
	dbc.db.StoreTxHash(hash)
}

// Returns true if a tx hash already exists.
// TODO(namdoh@): The hashKey is primarily used for persistently store a tx hash in db, so we
// quickly check if a tx has been seen in the past. When the scope of this key extends beyond
// tx hash, it's probably cleaner to refactor this into a separate API (instead of grouping
// it under chaindb).
func (dbc *DualBlockChain) CheckTxHash(hash *common.Hash) bool {
	return dbc.db.CheckTxHash(hash)
}

func (dbc *DualBlockChain) ZeroFee() bool {
	return false
}

func (dbc *DualBlockChain) ApplyMessage(vm base.KVM, msg types.Message, gp *types.GasPool) ([]byte, uint64, bool, error) {
	return nil, 0, false, fmt.Errorf("this function is not applied for dual blockchain")
}

func (dbc *DualBlockChain) GetBlockReward() *big.Int {
	return nil
}

func (dbc *DualBlockChain) GetConsensusMasterSmartContract() pos.MasterSmartContract {
	return pos.MasterSmartContract{}
}

func (dbc *DualBlockChain) GetConsensusNodeAbi() string {
	return ""
}

func (dbc *DualBlockChain) GetConsensusStakerAbi() string {
	return ""
}

func (dbc *DualBlockChain) GetFetchNewValidatorsTime() uint64 { return 0 }
