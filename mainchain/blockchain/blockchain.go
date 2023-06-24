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
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/state/snapshot"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
)

const (
	blockCacheLimit = 256
	maxFutureBlocks = 256
	TriesInMemory   = 128
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

// CacheConfig contains the configuration values for the trie database
// that's resident in a blockchain.
type CacheConfig struct {
	TrieCleanLimit      int           // Memory allowance (MB) to use for caching trie nodes in memory
	TrieCleanJournal    string        // Disk journal for saving clean cache entries.
	TrieCleanRejournal  time.Duration // Time interval to dump clean cache to disk periodically
	TrieCleanNoPrefetch bool          // Whether to disable heuristic state prefetching for followup blocks
	TrieDirtyLimit      int           // Memory limit (MB) at which to start flushing dirty trie nodes to disk
	TrieDirtyDisabled   bool          // Whether to disable trie write caching and GC altogether (archive node)
	TrieTimeLimit       time.Duration // Time limit after which to flush the current in-memory trie to disk
	SnapshotLimit       int           // Memory allowance (MB) to use for caching snapshot entries in memory
	Preimages           bool          // Whether to store preimage of trie key to the disk

	SnapshotNoBuild bool // Whether the background generation is allowed
	SnapshotWait    bool // Wait for snapshot construction on startup. TODO(karalabe): This is a dirty hack for testing, nuke it
}

// defaultCacheConfig are the default caching values if none are specified by the
// user (also used during testing).
var defaultCacheConfig = &CacheConfig{
	TrieCleanLimit: 256,
	TrieDirtyLimit: 256,
	TrieTimeLimit:  5 * time.Minute,
	SnapshotLimit:  256,
	SnapshotWait:   true,
}

type BlockChain struct {
	chainConfig *configs.ChainConfig // Chain & network configuration
	cacheConfig *CacheConfig         // Cache configuration for pruning

	db            kaidb.Database // Low level persistent database to store final content in
	snaps         *snapshot.Tree // Snapshot tree for fast trie leaf access
	lastWrite     uint64         // Last block when the state was flushed
	flushInterval atomic.Int64   // Time interval (processing time) after which to flush a state
	triedb        *trie.Database // The database handler for maintaining trie nodes.
	stateCache    state.Database // State database to reuse between imports (contains state cache)

	hc            *HeaderChain
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	mu sync.RWMutex // global mutex for locking chain operations

	currentBlock atomic.Value // Current head of the block chain

	blockCache   *lru.Cache // Cache for the most recent entire blocks
	futureBlocks *lru.Cache // future blocks are blocks added for later processing

	quit          chan struct{} // blockchain quit channel
	stopping      atomic.Bool   // false if chain is running, true when stopped
	procInterrupt atomic.Bool   // interrupt signaler for block processing

	processor *StateProcessor // block processor
	vmConfig  kvm.Config      // vm configurations
}

// NewBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Kardia Validator and Processor.
func NewBlockChain(db kaidb.Database, cacheConfig *CacheConfig, gs *genesis.Genesis) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = defaultCacheConfig
	}
	// Open trie database with provided config
	triedb := trie.NewDatabaseWithConfig(db, &trie.Config{
		Cache:     cacheConfig.TrieCleanLimit,
		Journal:   cacheConfig.TrieCleanJournal,
		Preimages: cacheConfig.Preimages,
	})

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(db, gs)
	if genesisErr != nil {
		return nil, genesisErr
	}

	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)

	bc := &BlockChain{
		chainConfig:  chainConfig,
		cacheConfig:  cacheConfig,
		db:           db,
		triedb:       triedb,
		quit:         make(chan struct{}),
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
	}
	bc.flushInterval.Store(int64(cacheConfig.TrieTimeLimit))
	bc.stateCache = state.NewDatabaseWithNodeDB(bc.db, bc.triedb)
	bc.processor = NewStateProcessor(bc)

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByHeight(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	// Load blockchain states from disk
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}

	// Load any existing snapshot, regenerating it if loading failed
	if bc.cacheConfig.SnapshotLimit > 0 {
		// If the chain was rewound past the snapshot persistent layer (causing
		// a recovery block number to be persisted to disk), check if we're still
		// in recovery mode and in that case, don't invalidate the snapshot on a
		// head mismatch.
		var recover bool

		head := bc.CurrentBlock()
		if layer := rawdb.ReadSnapshotRecoveryNumber(bc.db); layer != nil && *layer >= head.Height() {
			log.Warn("Enabling snapshot recovery", "chainhead", head.Height(), "diskbase", *layer)
			recover = true
		}
		snapconfig := snapshot.Config{
			CacheSize:  bc.cacheConfig.SnapshotLimit,
			Recovery:   recover,
			NoBuild:    bc.cacheConfig.SnapshotNoBuild,
			AsyncBuild: !bc.cacheConfig.SnapshotWait,
		}
		bc.snaps, _ = snapshot.New(snapconfig, bc.db, bc.stateCache.TrieDB(), head.AppHash())
	}

	return bc, nil
}

// StopInsert interrupts all insertion methods, causing them to return
// errInsertionInterrupted as soon as possible. Insertion is permanently disabled after
// calling this method.
func (bc *BlockChain) StopInsert() {
	bc.procInterrupt.Store(true)
}

// insertStopped returns true after StopInsert has been called.
func (bc *BlockChain) insertStopped() bool {
	return bc.procInterrupt.Load()
}

func (bc *BlockChain) Stop() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if !bc.stopping.CompareAndSwap(false, true) {
		return
	}

	log.Info("Stopping blockchain", "height", bc.CurrentBlock().Height(), "hash", bc.CurrentBlock().Hash(), "app hash", bc.CurrentBlock().AppHash())

	// Ensure that the entirety of the state snapshot is journalled to disk.
	var snapBase common.Hash
	if bc.snaps != nil {
		var err error
		if snapBase, err = bc.snaps.Journal(bc.CurrentBlock().AppHash()); err != nil {
			log.Error("Failed to journal state snapshot", "err", err)
		}
	}

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	if !bc.cacheConfig.TrieDirtyDisabled {
		triedb := bc.triedb

		for _, offset := range []uint64{0, 1, TriesInMemory - 1} {
			if number := bc.CurrentBlock().Height(); number > offset {
				recent := bc.GetBlockByHeight(number - offset)

				log.Info("Writing cached state to disk", "block", recent.Height(), "hash", recent.Hash(), "root", recent.AppHash())
				if err := triedb.Commit(recent.AppHash(), true); err != nil {
					log.Error("Failed to commit recent state trie", "err", err)
				}
			}
		}
		if snapBase != (common.Hash{}) {
			log.Info("Writing snapshot state to disk", "root", snapBase)
			if err := triedb.Commit(snapBase, true); err != nil {
				log.Error("Failed to commit recent state trie", "err", err)
			}
		}
		if size, _ := triedb.Size(); size != 0 {
			log.Error("Dangling trie nodes after full cleanup")
		}
	}
	// Flush the collected preimages to disk
	if err := bc.stateCache.TrieDB().Close(); err != nil {
		log.Error("Failed to close trie db", "err", err)
	}
	// Ensure all live cached entries be saved into disk, so that we can skip
	// cache warmup when node restarts.
	if bc.cacheConfig.TrieCleanJournal != "" {
		bc.triedb.SaveCache(bc.cacheConfig.TrieCleanJournal)
	}
	log.Info("Blockchain stopped")
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChain) loadLastState() error {
	// Restore the last known head block
	hash := rawdb.ReadHeadBlockHash(bc.db)
	if hash == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	headBlock := bc.GetBlockByHash(hash)
	if headBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", hash)
		return bc.Reset()
	}
	root := rawdb.ReadAppHash(bc.db, headBlock.Height())
	// Make sure the state associated with the block is available
	if _, err := state.New(root, bc.stateCache, nil); err != nil {
		// Dangling block without a state associated, init from scratch
		log.Warn("Head state missing, repairing chain", "height", headBlock.Height(), "hash", headBlock.Hash())
		if err := bc.repair(&headBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(headBlock)

	// Restore the last known head header
	headHeader := headBlock.Header()
	bc.hc.SetCurrentHeader(headHeader)

	log.Info("Loaded most recent local header", "height", headHeader.Height, "hash", headHeader.Hash())
	log.Info("Loaded most recent local full block", "height", headBlock.Height(), "hash", headBlock.Hash())

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

	rawdb.WriteBlock(bc.db, genesis, genesis.MakePartSet(types.BlockPartSizeBytes), &types.Commit{})

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
		root := rawdb.ReadAppHash(bc.db, (*head).Height())
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New(root, bc.stateCache, nil); err == nil {
			log.Info("Rewound blockchain to past state", "height", (*head).Height(), "hash", (*head).Hash())
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		lastBlockHash := (*head).LastBlockHash()
		lastHeight := (*head).Height() - 1
		block := bc.GetBlock(lastBlockHash, lastHeight)
		if block == nil {
			return fmt.Errorf("missing block height: %d [%x]", lastHeight, lastBlockHash)
		}
		*head = block
	}
}

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *BlockChain) SetHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db kaidb.Database, height uint64) {
		rawdb.DeleteBlockMeta(bc.db, height)
		rawdb.DeleteBlockPart(bc.db, height)
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
		root := rawdb.ReadAppHash(bc.db, currentBlock.Height())
		if _, err := state.New(root, bc.stateCache, nil); err != nil {
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

	if bc.insertStopped() {
		log.Info("Blockchain stopped, skip insert block", block.Height())
	}

	bc.insert(block)
	bc.chainHeadFeed.Send(events.ChainHeadEvent{Block: block})
}

// WriteReceipts writes the transactions receipt from execution of the transactions in the given block.
func (bc *BlockChain) WriteBlockInfo(block *types.Block, blockInfo *types.BlockInfo) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	rawdb.WriteBlockInfo(bc.db, block.Hash(), block.Header().Height, blockInfo)
}

// CommitTrie commits trie node such as statedb forcefully to disk.
func (bc *BlockChain) CommitTrie(root common.Hash) error {
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
	updateHeads := rawdb.ReadCanonicalHash(bc.db, block.Height()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteCanonicalHash(bc.db, block.Hash(), block.Height())

	bc.currentBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
	}
}

func (bc *BlockChain) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	rawdb.WriteBlock(bc.db, block, blockParts, seenCommit)
}
