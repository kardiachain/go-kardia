package consensus

import (
	"sync"

	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	cmn "github.com/kardiachain/go-kardia/lib/common"
)

type BlockStore struct {
	mtx    sync.RWMutex

	blockchain *blockchain.BlockChain
	height uint64
}

// NewBlockStore returns a new BlockStore with the given DB,
// initialized to the last height that was committed to the DB.
func NewBlockStore(blockchain *blockchain.BlockChain) *BlockStore {
	return &BlockStore{
		blockchain:     blockchain,
		height: blockchain.CurrentHeader().Height,
	}
}

func (bs *BlockStore) Height() uint64 {
    return bs.height
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, seenCommit *types.Commit) {
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
	// TODO(namdoh): Save block to db here.
	log.Error("Implement save block here.")

	// Save block commit (duplicate and separate from the Block)
	bs.blockchain.WriteCommit(height-1, block.LastCommit())

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	bs.blockchain.WriteCommit(height, seenCommit)

	// Done!
	bs.mtx.Lock()
	bs.height = height
	bs.mtx.Unlock()
}

