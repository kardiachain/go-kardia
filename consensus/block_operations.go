package consensus

import (
	"github.com/kardiachain/go-kardia/vm"
	"math/big"
	"sync"
	"time"

	"fmt"
	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.

type BlockOperations struct {
	mtx sync.RWMutex

	blockchain *blockchain.BlockChain
	txPool     *blockchain.TxPool
	height     uint64
}

// NewBlockOperations returns a new BlockOperations with latest chain & ,
// initialized to the last height that was committed to the DB.
func NewBlockOperations(blockchain *blockchain.BlockChain, txPool *blockchain.TxPool) *BlockOperations {
	return &BlockOperations{
		blockchain: blockchain,
		txPool:     txPool,
		height:     blockchain.CurrentHeader().Height,
	}
}

func (bs *BlockOperations) Height() uint64 {
	return bs.height
}

// NewHeader creates new block header from given data.
// Some header fields are not ready at this point.
func (bs *BlockOperations) NewHeader(height int64, numTxs uint64, blockId types.BlockID, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		Time:           big.NewInt(time.Now().Unix()),
		NumTxs:         numTxs,
		LastBlockID:    blockId,
		ValidatorsHash: validatorsHash,
		GasLimit:       10000,
	}
}

// NewBlock creates new block from given data.
func (bs *BlockOperations) NewBlock(header *types.Header, txs []*types.Transaction, receipts types.Receipts, commit *types.Commit) *types.Block {
	block := types.NewBlock(header, txs, receipts, commit)

	// TODO(namdoh): Fill the missing header info: AppHash, ConsensusHash,
	// LastResultHash.

	return block
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockOperations) SaveBlock(block *types.Block, seenCommit *types.Commit) {
	if block == nil {
		common.PanicSanity("BlockStore can only save a non-nil block")
	}
	height := block.Height()
	if g, w := height, bs.Height()+1; g != w {
		common.PanicSanity(common.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// Save block
	if height != bs.Height()+1 {
		common.PanicSanity(common.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}

	// TODO(kiendn): WriteBlockWithoutState returns an error, write logic check if error appears
	if err := bs.blockchain.WriteBlockWithoutState(block); err != nil {
		common.PanicSanity(common.Fmt("WriteBlockWithoutState fails with error %v", err))
	}

	// Save block commit (duplicate and separate from the Block)
	bs.blockchain.WriteCommit(height-1, block.LastCommit())

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	bs.blockchain.WriteCommit(height, seenCommit)

	// TODO(thientn/namdoh): remove the committed transactions from tx pool.
	// @kiendn: remove all txs in block from tx pool
	txs := block.Transactions()
	for _, tx := range txs {
		bs.txPool.RemoveTx(tx.Hash(), true)
	}

	bs.mtx.Lock()
	bs.height = height
	bs.mtx.Unlock()
}

// CollectTransactions queries list of pending transactions from tx pool.
func (b *BlockOperations) CollectTransactions() []*types.Transaction {
	pending, err := b.txPool.Pending()
	if err != nil {
		log.Error("Fail to get pending txns", "err", err)
		return nil
	}

	// TODO: do basic verification & check with gas & sort by nonce
	// check code NewTransactionsByPriceAndNonce
	pendingTxns := make([]*types.Transaction, 0)
	for _, txns := range pending {
		for _, txn := range txns {
			pendingTxns = append(pendingTxns, txn)
		}
	}
	return pendingTxns
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (b *BlockOperations) LoadBlock(height uint64) *types.Block {
	return b.blockchain.GetBlockByHeight(height)
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (b *BlockOperations) LoadBlockCommit(height uint64) *types.Commit {
	block := b.blockchain.GetBlockByHeight(height + 1)
	if block == nil {
		return nil
	}

	return block.LastCommit()
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (b *BlockOperations) LoadSeenCommit(height uint64) *types.Commit {
	commit := b.blockchain.ReadCommit(height)
	if commit == nil {
		log.Error("LoadSeenCommit return nothing", "height", height)
	}

	return commit
}

// CommitTransactions executes the given transactions and commits the result stateDB to disk.
func (b *BlockOperations) CommitTransactions(txs types.Transactions, header *types.Header) (common.Hash, types.Receipts, error) {
	var (
		receipts = types.Receipts{}
		usedGas  = new(uint64)
	)
	counter := 0

	// Blockchain state at head block.
	state, err := b.blockchain.State()
	if err != nil {
		log.Error("Fail to get blockchain head state", "err", err)
		return common.Hash{}, nil, err
	}

	// GasPool
	gasPool := new(blockchain.GasPool).AddGas(header.GasLimit)

	// TODO(thientn): verifies the list is sorted by nonce so tx with lower nonce is execute first.
	for _, tx := range txs {
		state.Prepare(tx.Hash(), common.Hash{}, counter)
		snap := state.Snapshot()
		// TODO(thientn): confirms nil coinbase is acceptable.
		receipt, _, err := blockchain.ApplyTransaction(b.blockchain, nil, gasPool, state, header, tx, usedGas, vm.Config{})
		if err != nil {
			state.RevertToSnapshot(snap)
			// TODO(thientn): check error type and jump to next tx if possible
			return common.Hash{}, nil, err
		}
		counter++
		receipts = append(receipts, receipt)
	}
	root, err := state.Commit(true)

	if err != nil {
		log.Error("Fail to commit new statedb after txs", "err", err)
		return common.Hash{}, nil, err
	}
	err = b.blockchain.CommitTrie(root)
	if err != nil {
		log.Error("Fail to write statedb trie to disk", "err", err)
		return common.Hash{}, nil, err
	}

	// TODO(thientn): write receipts to rawdb separately
	// receipt require block hash which is not built yet. This should be a separate function.
	// rawdb.WriteReceipts(batch, block.Hash(), block.Header().Height, receipts)

	return root, receipts, nil
}

func (bs *BlockOperations) CommitAndValidateBlockTxs(block *types.Block) error {
	root, _, err := bs.CommitTransactions(block.Transactions(), block.Header())
	if err != nil {
		return err
	}
	if root != block.Root() {
		return fmt.Errorf("different new state root: Block root: %s, Execution result: %s", block.Root().Hex(), root.Hex())
	}
	// TODO(thientn): compare receipts.
	return nil
}
