/*
 *  Copyright 2021 KardiaChain
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
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus/misc"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

// BlockGen creates blocks for testing.
// See GenerateChain for a detailed explanation.
type BlockGen struct {
	i       int
	parent  *types.Block
	chain   []*types.Block
	header  *types.Header
	statedb *state.StateDB
	txIndex int
	gasUsed uint64

	gasPool  *types.GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt

	config  *configs.ChainConfig
	blockOp *BlockOperations
}

// SetProposer sets the coinbase of the generated block.
// It can be called at most once.
func (b *BlockGen) SetProposer(addr common.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("proposer must be set before adding transactions")
		}
		panic("proposer can only be set once")
	}
	b.header.ProposerAddress = addr
	b.gasPool = new(types.GasPool).AddGas(b.header.GasLimit)
}

// SetBlockOperations sets the block operator.
// It can be called at most once.
func (b *BlockGen) SetBlockOperations(blockOp *BlockOperations) {
	if b.blockOp != nil {
		panic("block operator can only be set once")
	}
	b.blockOp = blockOp
}

// AddTx adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTx panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. Notably, contract code relying on the BLOCKHASH instruction
// will panic during execution.
func (b *BlockGen) AddTx(tx *types.Transaction) {
	b.AddTxWithChain(b.blockOp.blockchain, tx)
}

// AddTxWithChain adds a transaction to the generated block. If no proposer has
// been set, the block's proposer is set to the zero address.
//
// AddTxWithChain panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. If contract code relies on the BLOCKHASH instruction,
// the block in chain will be returned.
func (b *BlockGen) AddTxWithChain(bc *BlockChain, tx *types.Transaction) {
	if b.gasPool == nil {
		b.SetProposer(common.Address{})
	}
	b.statedb.Prepare(tx.Hash(), common.Hash{}, b.txIndex)
	receipt, _, err := ApplyTransaction(bc.logger, bc, b.gasPool, b.statedb, b.header, tx, &b.gasUsed, kvm.Config{})
	if err != nil {
		panic(err)
	}
	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}

// GetBalance returns the balance of the given address at the generated block.
func (b *BlockGen) GetBalance(addr common.Address) *big.Int {
	return b.statedb.GetBalance(addr)
}

// AddUncheckedTx forcefully adds a transaction to the block without any
// validation.
//
// AddUncheckedTx will cause consensus failures when used during real
// chain processing. This is best used in conjunction with raw block insertion.
func (b *BlockGen) AddUncheckedTx(tx *types.Transaction) {
	b.txs = append(b.txs, tx)
}

// Height returns the block number of the block being generated.
func (b *BlockGen) Height() uint64 {
	return b.header.Height
}

// AddUncheckedReceipt forcefully adds a receipts to the block without a
// backing transaction.
//
// AddUncheckedReceipt will cause consensus failures when used during real
// chain processing. This is best used in conjunction with raw block insertion.
func (b *BlockGen) AddUncheckedReceipt(receipt *types.Receipt) {
	b.receipts = append(b.receipts, receipt)
}

// TxNonce returns the next valid transaction nonce for the
// account at addr. It panics if the account does not exist.
func (b *BlockGen) TxNonce(addr common.Address) uint64 {
	if !b.statedb.Exist(addr) {
		panic(fmt.Sprintf("account does not exist: %v", addr.Hex()))
	}
	return b.statedb.GetNonce(addr)
}

// PrevBlock returns a previously generated block by number. It panics if
// num is greater or equal to the number of the block being generated.
// For index -1, PrevBlock returns the parent block given to GenerateChain.
func (b *BlockGen) PrevBlock(index int) *types.Block {
	if index >= b.i {
		panic(fmt.Errorf("block index %d out of range (%d,%d)", index, -1, b.i))
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}

// GenerateChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their proposers will be the zero address.
//
// Blocks created by GenerateChain do not contain valid proof of stake
// values.
func GenerateChain(config *configs.ChainConfig, parent *types.Block, db kaidb.Database, n int, gen func(int, *BlockGen)) ([]*types.Block, []types.Receipts) {
	if config == nil {
		config = configs.TestChainConfig
	}
	blocks, receipts := make(types.Blocks, n), make([]types.Receipts, n)
	blockOp, err := initBlockOperations(db, parent)
	if err != nil {
		return nil, nil
	}
	genblock := func(i int, parent *types.Block, statedb *state.StateDB) (*types.Block, types.Receipts) {
		b := &BlockGen{i: i, chain: blocks, parent: parent, statedb: statedb, config: config, blockOp: blockOp}
		b.header = makeHeader(parent, statedb)

		// Mutate the state and block according to any hard-fork specs
		if config.MainnetV2Block != nil && *config.MainnetV2Block == b.header.Height {
			misc.ApplyMainnetV2HardFork(statedb, nil)
		}
		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}
		if b.blockOp != nil {
			// Finalize and seal the block
			_, root, blockInfo, _, err := b.blockOp.commitUnverifiedTransactions(b.txs, b.header)
			if err != nil {
				panic(fmt.Sprintf("commit transactions failed: %v", err))
			}
			// Write state changes to db
			//root, err = statedb.Commit(false)
			//if err != nil {
			//	panic(fmt.Sprintf("state write error: %v", err))
			//}
			//if err := statedb.Database().TrieDB().Commit(root, false); err != nil {
			//	panic(fmt.Sprintf("trie write error: %v", err))
			//}
			// construct the block with pre-determined AppHash
			b.header.AppHash = root
			newBlock := types.NewBlock(b.header, b.txs, &types.Commit{
				BlockID: types.BlockID{
					Hash: b.header.Hash(),
				},
				Signatures: nil,
				Height:     b.Height(),
				Round:      0,
			}, []types.Evidence{})
			newBlock.Hash()
			blockOp.saveBlockInfo(blockInfo, newBlock)
			blockOp.blockchain.DB().WriteHeadBlockHash(newBlock.Hash())
			blockOp.blockchain.DB().WriteTxLookupEntries(newBlock)
			blockOp.blockchain.DB().WriteAppHash(newBlock.Height(), root)
			blockOp.blockchain.InsertHeadBlock(newBlock)
			return newBlock, b.receipts
		}
		return nil, nil
	}
	for i := 1; i < n; i++ {
		statedb, err := state.New(log.New(), parent.AppHash(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}
	return blocks, receipts
}

func initBlockOperations(db kaidb.Database, genesisBlock *types.Block) (*BlockOperations, error) {
	logger := log.New()
	configs.AddDefaultContract()
	stakingUtil, err := staking.NewSmcStakingUtil()
	if err != nil {
		return nil, err
	}
	bc, err := NewBlockChain(logger, kvstore.NewStoreDB(db), configs.TestChainConfig)
	if err != nil {
		return nil, err
	}

	return NewBlockOperations(logger, bc, tx_pool.NewTxPool(tx_pool.DefaultTxPoolConfig, configs.TestChainConfig, bc), nil, stakingUtil), nil
}

func makeHeader(parent *types.Block, state *state.StateDB) *types.Header {
	var blockTime time.Time
	if parent.Time().Equal(time.Time{}) {
		blockTime = time.Now()
	} else {
		blockTime = parent.Time().Add(time.Second * 10) // block time is fixed at 10 seconds
	}
	header := &types.Header{
		LastBlockID:     types.BlockID{Hash: parent.Hash()},
		ProposerAddress: common.Address{},
		GasLimit:        parent.GasLimit(),
		Height:          parent.Height() + 1,
		Time:            blockTime,
	}
	return header
}

// makeHeaderChain creates a deterministic chain of headers rooted at parent.
func makeHeaderChain(parent *types.Header, n int, db kaidb.Database, seed int) []*types.Header {
	blocks := makeBlockChain(types.NewBlockWithHeader(parent), n, db, seed)
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	return headers
}

// makeBlockChain creates a deterministic chain of blocks rooted at parent.
func makeBlockChain(parent *types.Block, n int, db kaidb.Database, seed int) []*types.Block {
	blocks, _ := GenerateChain(configs.TestChainConfig, parent, db, n, func(i int, b *BlockGen) {
		b.SetProposer(common.Address{0: byte(seed), 19: byte(i)})
	})

	return blocks
}
