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
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
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
	"github.com/kardiachain/go-kardia/lib/prque"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
)

const (
	blockCacheLimit = 256
	maxFutureBlocks = 256
	TriesInMemory   = 128
)

var (
	ErrNoGenesis    = errors.New("Genesis not found in chain")
	errChainStopped = errors.New("blockchain is stopped")
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
	staking     *staking.StakingSmcUtil

	db            kaidb.Database                   // Low level persistent database to store final content in
	snaps         *snapshot.Tree                   // Snapshot tree for fast trie leaf access
	triegc        *prque.Prque[int64, common.Hash] // Priority queue mapping block numbers to tries to gc
	gcproc        time.Duration                    // Accumulates canonical block processing for trie dumping
	lastWrite     uint64                           // Last block when the state was flushed
	flushInterval atomic.Int64                     // Time interval (processing time) after which to flush a state
	triedb        *trie.Database                   // The database handler for maintaining trie nodes.
	stateCache    state.Database                   // State database to reuse between imports (contains state cache)

	hc            *HeaderChain
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	mu sync.RWMutex // global mutex for locking chain operations

	currentBlock atomic.Value // Current head of the block chain

	blockCache   *lru.Cache // Cache for the most recent entire blocks
	futureBlocks *lru.Cache // future blocks are blocks added for later processing

	quit     chan struct{} // blockchain quit channel
	stopping atomic.Bool   // false if chain is running, true when stopped

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

	// Setup the genesis block, commit the provided genesis specification
	// to database if the genesis block is not present yet, or load the
	// stored one from database.
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
		triegc:       prque.New[int64, common.Hash](nil),
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

	// Make sure the state associated with the block is available
	head := bc.CurrentBlock()
	root := rawdb.ReadAppHash(bc.db, head.Height())
	if !bc.HasState(root) {
		// Head state is missing, before the state recovery, find out the
		// disk layer point of snapshot(if it's enabled). Make sure the
		// rewound point is lower than disk layer.
		var diskRoot common.Hash
		if bc.cacheConfig.SnapshotLimit > 0 {
			diskRoot = rawdb.ReadSnapshotRoot(bc.db)
		}
		if diskRoot != (common.Hash{}) {
			log.Warn("Head state missing, repairing", "number", head.Height(), "hash", head.Hash(), "snaproot", diskRoot)

			snapDisk, err := bc.setHeadBeyondRoot(head.Height(), diskRoot, true)
			if err != nil {
				return nil, err
			}
			// Chain rewound, persist old snapshot number to indicate recovery procedure
			if snapDisk != 0 {
				rawdb.WriteSnapshotRecoveryNumber(bc.db, snapDisk)
			}
		} else {
			log.Warn("Head state missing, repairing", "number", head.Height(), "hash", head.Hash())
			if _, err := bc.setHeadBeyondRoot(head.Height(), common.Hash{}, true); err != nil {
				return nil, err
			}
		}
	}

	// Load any existing snapshot, regenerating it if loading failed
	if bc.cacheConfig.SnapshotLimit > 0 {
		// If the chain was rewound past the snapshot persistent layer (causing
		// a recovery block number to be persisted to disk), check if we're still
		// in recovery mode and in that case, don't invalidate the snapshot on a
		// head mismatch.
		var recover bool

		head := bc.CurrentBlock()
		root := rawdb.ReadAppHash(bc.db, head.Height())

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
		bc.snaps, _ = snapshot.New(snapconfig, bc.db, bc.stateCache.TrieDB(), root)
	}

	return bc, nil
}

func (bc *BlockChain) Stop() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if !bc.stopping.CompareAndSwap(false, true) {
		return
	}

	appHash := rawdb.ReadAppHash(bc.db, bc.CurrentBlock().Height())
	log.Info("Stopping blockchain", "height", bc.CurrentBlock().Height(), "hash", bc.CurrentBlock().Hash(), "app hash", bc.CurrentBlock().AppHash(), "root", appHash)

	// Ensure that the entirety of the state snapshot is journalled to disk.
	var snapBase common.Hash
	if bc.snaps != nil {
		var err error
		// Read the app hash corresponding to height, instead of block.AppHash()
		if snapBase, err = bc.snaps.Journal(appHash); err != nil {
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
				// Read the app hash corresponding to height, instead of block.AppHash()
				appHash := rawdb.ReadAppHash(bc.db, recent.Height())

				log.Info("Writing cached state to disk", "block", recent.Height(), "hash", recent.Hash(), "root", appHash)
				if err := triedb.Commit(appHash, true); err != nil {
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
	bc.writeHeadBlock(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())

	return nil
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
		if !bc.HasState(root) {
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

// setHeadBeyondRoot rewinds the local chain to a new head with the extra condition
// that the rewind must pass the specified state root. This method is meant to be
// used when rewinding with snapshots enabled to ensure that we go back further than
// persistent disk layer. Depending on whether the node was fast synced or full, and
// in which state, the method will try to delete minimal data from disk whilst
// retaining chain consistency.
//
// The method returns the block number where the requested root cap was found.
func (bc *BlockChain) setHeadBeyondRoot(head uint64, root common.Hash, repair bool) (uint64, error) {
	if !bc.mu.TryLock() {
		return 0, errChainStopped
	}
	defer bc.mu.Unlock()

	// Track the block number of the requested root hash
	var rootNumber uint64 // (no root == always 0)

	// Retrieve the last pivot block to short circuit rollbacks beyond it and the
	// current freezer limit to start nuking id underflown
	pivot := rawdb.ReadLastPivotNumber(bc.db)

	header := rawdb.ReadHeader(bc.db, head)

	// Rewind the blockchain, ensuring we don't end up with a stateless head
	// block. Note, depth equality is permitted to allow using SetHead as a
	// chain reparation mechanism without deleting any data!
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && header.Height <= currentBlock.Height() {
		newHeadBlock := bc.GetBlock(header.Hash(), header.Height)
		if newHeadBlock == nil {
			log.Error("Gap in the chain, rewinding to genesis", "number", header.Height, "hash", header.Hash())
			newHeadBlock = bc.genesisBlock
		} else {
			// Block exists, keep rewinding until we find one with state,
			// keeping rewinding until we exceed the optional threshold
			// root hash
			beyondRoot := (root == common.Hash{}) // Flag whether we're beyond the requested root (no root, always true)

			for {
				// If a root threshold was requested but not yet crossed, check
				//
				appHash := rawdb.ReadAppHash(bc.db, newHeadBlock.Height())
				if root != (common.Hash{}) && !beyondRoot && appHash == root {
					beyondRoot, rootNumber = true, newHeadBlock.Height()
				}
				if !bc.HasState(appHash) {
					log.Trace("Block state missing, rewinding further", "number", newHeadBlock.Height(), "hash", newHeadBlock.Hash())
					if pivot == nil || newHeadBlock.Height() > *pivot {
						parent := bc.GetBlock(newHeadBlock.LastBlockHash(), newHeadBlock.Height()-1)
						if parent != nil {
							newHeadBlock = parent
							continue
						}
						log.Error("Missing block in the middle, aiming genesis", "number", newHeadBlock.Height()-1, "hash", newHeadBlock.LastBlockHash())
						newHeadBlock = bc.genesisBlock
					} else {
						log.Trace("Rewind passed pivot, aiming genesis", "number", newHeadBlock.Height(), "hash", newHeadBlock.Hash(), "pivot", *pivot)
						newHeadBlock = bc.genesisBlock
					}
				}
				if beyondRoot || newHeadBlock.Height() == 0 {
					log.Debug("Rewound to block with state", "number", newHeadBlock.Height(), "hash", newHeadBlock.Hash())
					break
				}

				// delete rewounded block data
				rawdb.DeleteBody(bc.db, newHeadBlock.Hash(), newHeadBlock.Height())
				rawdb.DeleteBlockMeta(bc.db, newHeadBlock.Height())
				rawdb.DeleteBlockPart(bc.db, newHeadBlock.Height())

				log.Debug("Skipping block with threshold state", "number", newHeadBlock.Height(), "hash", newHeadBlock.Hash(), "root", appHash)
				newHeadBlock = bc.GetBlock(newHeadBlock.LastBlockHash(), newHeadBlock.Height()-1) // Keep rewinding

			}
		}
		rawdb.WriteHeadBlockHash(bc.db, newHeadBlock.Hash())

		// Degrade the chain markers if they are explicitly reverted.
		// In theory we should update all in-memory markers in the
		// last step, however the direction of SetHead is from high
		// to low, so it's safe to update in-memory markers directly.
		bc.currentBlock.Store(newHeadBlock)
	}

	// Clear out any stale content from the caches
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()

	err := bc.loadLastState()

	return rootNumber, err
}

// WriteBlockWithState writes the block and all associated state to the database.
func (bc *BlockChain) WriteBlockAndSetHead(block *types.Block, blockInfo *types.BlockInfo, state *state.StateDB) error {
	if !bc.mu.TryLock() {
		return errChainStopped
	}
	defer bc.mu.Unlock()

	return bc.writeBlockAndSetHead(block, blockInfo, state)
}

// writeBlockAndSetHead is the internal implementation of WriteBlockAndSetHead.
// This function expects the chain mutex to be held.
func (bc *BlockChain) writeBlockAndSetHead(block *types.Block, blockInfo *types.BlockInfo, state *state.StateDB) error {
	if err := bc.writeBlockWithState(block, blockInfo, state); err != nil {
		return err
	}

	bc.writeHeadBlock(block)

	bc.futureBlocks.Remove(block.Hash())
	bc.chainHeadFeed.Send(events.ChainHeadEvent{Block: block})

	return nil
}

// writeBlockWithState writes block, metadata and corresponding state data to the
// database.
func (bc *BlockChain) writeBlockWithState(block *types.Block, blockInfo *types.BlockInfo, state *state.StateDB) error {
	// Send logs of emitted events to logs feed for collecting
	var logs []*types.Log
	for _, r := range blockInfo.Receipts {
		logs = append(logs, r.Logs...)
	}
	bc.logsFeed.Send(logs)

	// Commit all cached state changes into underlying memory database.
	root, err := state.Commit(true)
	if err != nil {
		return err
	}

	blockBatch := bc.db.NewBatch()

	rawdb.WriteBlockInfo(blockBatch, block.Hash(), block.Header().Height, blockInfo)
	rawdb.WriteCanonicalHash(blockBatch, block.Hash(), block.Height())
	rawdb.WritePreimages(blockBatch, state.Preimages())
	// There is known bug of a block stores app hash of previous block height
	// That is, block at height H stores the app hash of height H-1
	// But in local database, we store a mapping of height and app hash of that height
	rawdb.WriteAppHash(blockBatch, block.Height(), root)

	if err := blockBatch.Write(); err != nil {
		log.Crit("Failed to write block into disk", "err", err)
	}

	// If we're running an archive node, always flush
	if bc.cacheConfig.TrieDirtyDisabled {
		return bc.triedb.Commit(root, false)
	}
	// Full but not archive node, do proper garbage collection
	bc.triedb.Reference(root, common.Hash{}) // metadata reference to keep trie alive
	bc.triegc.Push(root, -int64(block.Height()))

	current := block.Height()
	// Flush limits are not considered for the first TriesInMemory blocks.
	if current <= TriesInMemory {
		return nil
	}
	// If we exceeded our memory allowance, flush matured singleton nodes to disk
	var (
		nodes, imgs = bc.triedb.Size()
		limit       = common.StorageSize(bc.cacheConfig.TrieDirtyLimit) * 1024 * 1024
	)
	if nodes > limit || imgs > 4*1024*1024 {
		bc.triedb.Cap(limit - ethdb.IdealBatchSize)
	}
	// Find the next state trie we need to commit
	chosen := current - TriesInMemory
	flushInterval := time.Duration(bc.flushInterval.Load())
	// If we exceeded time allowance, flush an entire trie to disk
	if bc.gcproc > flushInterval {
		header := bc.GetHeaderByHeight(chosen)
		// If we're exceeding limits but haven't reached a large enough memory gap,
		// warn the user that the system is becoming unstable.
		if chosen < bc.lastWrite+TriesInMemory && bc.gcproc >= 2*flushInterval {
			log.Info("State in memory for too long, committing", "time", bc.gcproc, "allowance", flushInterval, "optimum", float64(chosen-bc.lastWrite)/TriesInMemory)
		}
		// Flush an entire trie and restart the counters
		headerAppHash := rawdb.ReadAppHash(bc.db, header.Height)
		bc.triedb.Commit(headerAppHash, true)
		bc.lastWrite = chosen
		bc.gcproc = 0
	}
	// Garbage collect anything below our required write retention
	for !bc.triegc.Empty() {
		root, number := bc.triegc.Pop()
		if uint64(-number) > chosen {
			bc.triegc.Push(root, number)
			break
		}
		bc.triedb.Dereference(root)
	}
	return nil
}

// writeHeadBlock injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChain) writeHeadBlock(block *types.Block) {
	// Add the block to the canonical chain number scheme and mark as the head
	batch := bc.db.NewBatch()
	rawdb.WriteCanonicalHash(batch, block.Hash(), block.Height())
	rawdb.WriteTxLookupEntries(batch, block)
	rawdb.WriteHeadBlockHash(batch, block.Hash())
	// Flush the whole batch into the disk, exit the node if failed
	if err := batch.Write(); err != nil {
		log.Crit("Failed to update chain indexes and markers", "err", err)
	}

	bc.currentBlock.Store(block)
	bc.hc.SetCurrentHeader(block.Header())
}

func (bc *BlockChain) AccumulateGCProc(proctime time.Duration) error {
	if !bc.mu.TryLock() {
		return errChainStopped
	}
	defer bc.mu.Unlock()

	bc.gcproc += proctime

	return nil
}

func (bc *BlockChain) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	rawdb.WriteBlock(bc.db, block, blockParts, seenCommit)
}
