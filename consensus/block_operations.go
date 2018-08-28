package consensus

import (
	"sync"

	"github.com/kardiachain/go-kardia/blockchain"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"fmt"
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

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockOperations) SaveBlock(block *types.Block, seenCommit *types.Commit) {
	if block == nil {
		cmn.PanicSanity("BlockStore can only save a non-nil block")
	}
	height := block.Height()
	if g, w := height, bs.Height()+1; g != w {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// Save block
	if height != bs.Height()+1 {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}

	// TODO(kiendn): WriteBlockWithoutState returns an error, write logic check if error appears
	if err := bs.blockchain.WriteBlockWithoutState(block); err != nil {
		cmn.PanicSanity(cmn.Fmt("WriteBlockWithoutState fails with error %v", err))
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
	// Done!
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

// GenerateNewAccountStates generates new accountStates by executing given txns on the account state of blockchain head.
func (b *BlockOperations) GenerateNewAccountStates(txns []*types.Transaction) (types.AccountStates, error) {
	// use accountState of latest block
	if len(txns) > 0 {
		accounts := b.blockchain.CurrentBlock().Accounts()
		return blockchain.ApplyTransactionsToAccountState(txns, accounts)
	}
	return nil, fmt.Errorf("transactions list is empty")
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (b *BlockOperations) LoadBlock(height uint64) *types.Block {
	return b.blockchain.GetBlockByHeight(height)
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
