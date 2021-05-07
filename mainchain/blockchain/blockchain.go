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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

const (
	blockCacheLimit    = 256
	receiptsCacheLimit = 32
	txLookupCacheLimit = 1
	maxFutureBlocks    = 256
	TriesInMemory      = 128
)

var ()

// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	TrieCleanLimit      int           // Memory allowance (MB) to use for caching trie nodes in memory
	TrieCleanJournal    string        // Disk journal for saving clean cache entries.
	TrieCleanRejournal  time.Duration // Time interval to dump clean cache to disk periodically
	TrieCleanNoPrefetch bool          // Whether to disable heuristic state prefetching for followup blocks
	TrieDirtyLimit      int           // Memory limit (MB) at which to start flushing dirty trie nodes to disk
	TrieDirtyDisabled   bool          // Whether to disable trie write caching and GC altogether (archive node)
	TrieTimeLimit       time.Duration // Time limit after which to flush the current in-memory trie to disk
	Preimages           bool          // Whether to store preimage of trie key to the disk
}

// defaultCacheConfig are the default caching values if none are specified by the
// user (also used during testing).
var defaultCacheConfig = &CacheConfig{
	TrieCleanLimit: 256,
	TrieDirtyLimit: 256,
	TrieTimeLimit:  5 * time.Minute,
}

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
// defined by the two stage Validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the Validator. Failing results in aborting of
// the import.
//
// The BlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type BlockChain struct {
	logger log.Logger

	chainConfig *configs.ChainConfig // Chain & network configuration
	cacheConfig *CacheConfig         // Cache configuration for pruning

	// txLookupLimit is the maximum number of blocks from head whose tx indices
	// are reserved:
	//  * 0:   means no limit and regenerate any missing indexes
	//  * N:   means N block limit [HEAD-N+1, HEAD] and delete extra indexes
	//  * nil: disable tx reindexer/deleter, but still index new blocks
	txLookupLimit uint64

	db types.StoreDB // Blockchain database
	hc *HeaderChain

	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope

	genesisBlock *types.Block

	mu sync.RWMutex // global mutex for locking chain operations

	currentBlock atomic.Value // Current head of the block chain

	stateCache    state.Database // State database to reuse between imports (contains state cache)
	receiptsCache *lru.Cache     // Cache for the most recent receipts per block
	blockCache    *lru.Cache     // Cache for the most recent entire blocks
	txLookupCache *lru.Cache     // Cache for the most recent transaction lookup data.
	futureBlocks  *lru.Cache     // future blocks are blocks added for later processing

	quit chan struct{}  // blockchain quit channel
	wg   sync.WaitGroup // chain processing wait group for shutting down

	processor *StateProcessor // block processor
	vmConfig  kvm.Config      // vm configurations
}

func (bc *BlockChain) P2P() *configs.P2PConfig {
	return bc.P2P()
}

// GetVMConfig returns the block chain VM config.
func (bc *BlockChain) GetVMConfig() *kvm.Config {
	return &bc.vmConfig
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
func NewBlockChain(logger log.Logger, db types.StoreDB, chainConfig *configs.ChainConfig, cacheConfig *CacheConfig, txLookupLimit *uint64) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = defaultCacheConfig
	}
	blockCache, _ := lru.New(blockCacheLimit)
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	txLookupCache, _ := lru.New(txLookupCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)

	bc := &BlockChain{
		logger:        logger,
		chainConfig:   chainConfig,
		cacheConfig:   cacheConfig,
		db:            db,
		stateCache:    state.NewDatabase(db.DB()),
		receiptsCache: receiptsCache,
		blockCache:    blockCache,
		futureBlocks:  futureBlocks,
		txLookupCache: txLookupCache,
		quit:          make(chan struct{}),
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

	// Initialize the chain with ancient data if it isn't empty.
	var txIndexBlock uint64

	// Take ownership of this particular state
	// go bc.update()
	if txLookupLimit != nil {
		bc.txLookupLimit = *txLookupLimit

		bc.wg.Add(1)
		go bc.maintainTxIndex(txIndexBlock)
	}
	// If periodic cache journal is required, spin it up.
	if bc.cacheConfig.TrieCleanRejournal > 0 {
		if bc.cacheConfig.TrieCleanRejournal < time.Minute {
			log.Warn("Sanitizing invalid trie cache journal time", "provided", bc.cacheConfig.TrieCleanRejournal, "updated", time.Minute)
			bc.cacheConfig.TrieCleanRejournal = time.Minute
		}
		triedb := bc.stateCache.TrieDB()
		bc.wg.Add(1)
		go func() {
			defer bc.wg.Done()
			triedb.SaveCachePeriodically(bc.cacheConfig.TrieCleanJournal, bc.cacheConfig.TrieCleanRejournal, bc.quit)
		}()
	}

	bc.processor = NewStateProcessor(logger, bc)

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
	block := bc.db.ReadBlock(number)
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

// GetHeaderByHash retrieves a block header from the database by height, caching it if
// found.
func (bc *BlockChain) GetHeaderByHeight(height uint64) *types.Header {
	return bc.hc.GetHeaderByHeight(height)
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
	bc.receiptsCache.Purge()
	bc.blockCache.Purge()
	bc.txLookupCache.Purge()
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
func (bc *BlockChain) CommitTrie(root common.Hash) error {
	triedb := bc.stateCache.TrieDB()
	return triedb.Commit(root, false, nil)
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

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

// maintainTxIndex is responsible for the construction and deletion of the
// transaction index.
//
// User can use flag `txlookuplimit` to specify a "recentness" block, below
// which ancient tx indices get deleted. If `txlookuplimit` is 0, it means
// all tx indices will be reserved.
//
// The user can adjust the txlookuplimit value for each launch after fast
// sync, Geth will automatically construct the missing indices and delete
// the extra indices.
func (bc *BlockChain) maintainTxIndex(ancients uint64) {
	defer bc.wg.Done()

	// Before starting the actual maintenance, we need to handle a special case,
	// where user might init Geth with an external ancient database. If so, we
	// need to reindex all necessary transactions before starting to process any
	// pruning requests.
	if ancients > 0 {
		var from = uint64(0)
		if bc.txLookupLimit != 0 && ancients > bc.txLookupLimit {
			from = ancients - bc.txLookupLimit
		}
		kvstore.IndexTransactions(bc.db.DB(), from, ancients, bc.quit)
	}
	// indexBlocks reindexes or unindexes transactions depending on user configuration
	indexBlocks := func(tail *uint64, head uint64, done chan struct{}) {
		defer func() { done <- struct{}{} }()

		// If the user just upgraded Geth to a new version which supports transaction
		// index pruning, write the new tail and remove anything older.
		if tail == nil {
			if bc.txLookupLimit == 0 || head < bc.txLookupLimit {
				// Nothing to delete, write the tail and return
				kvstore.WriteTxIndexTail(bc.db.DB(), 0)
			} else {
				// Prune all stale tx indices and record the tx index tail
				kvstore.UnindexTransactions(bc.db.DB(), 0, head-bc.txLookupLimit+1, bc.quit)
			}
			return
		}
		// If a previous indexing existed, make sure that we fill in any missing entries
		if bc.txLookupLimit == 0 || head < bc.txLookupLimit {
			if *tail > 0 {
				kvstore.IndexTransactions(bc.db.DB(), 0, *tail, bc.quit)
			}
			return
		}
		// Update the transaction index to the new chain state
		if head-bc.txLookupLimit+1 < *tail {
			// Reindex a part of missing indices and rewind index tail to HEAD-limit
			kvstore.IndexTransactions(bc.db.DB(), head-bc.txLookupLimit+1, *tail, bc.quit)
		} else {
			// Unindex a part of stale indices and forward index tail to HEAD-limit
			kvstore.UnindexTransactions(bc.db.DB(), *tail, head-bc.txLookupLimit+1, bc.quit)
		}
	}
	// Any reindexing done, start listening to chain events and moving the index window
	var (
		done   chan struct{}                         // Non-nil if background unindexing or reindexing routine is active.
		headCh = make(chan events.ChainHeadEvent, 1) // Buffered to avoid locking up the event feed
	)
	sub := bc.SubscribeChainHeadEvent(headCh)
	if sub == nil {
		return
	}
	defer sub.Unsubscribe()

	for {
		select {
		case head := <-headCh:
			if done == nil {
				done = make(chan struct{})
				go indexBlocks(kvstore.ReadTxIndexTail(bc.db.DB()), head.Block.Height(), done)
			}
		case <-done:
			done = nil
		case <-bc.quit:
			if done != nil {
				log.Info("Waiting background transaction indexer to exit")
				<-done
			}
			return
		}
	}
}
