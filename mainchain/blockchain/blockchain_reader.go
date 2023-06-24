/*
 *  Copyright 2023 KardiaChain
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
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/types"
)

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

// LoadBlockPart ...
func (bc *BlockChain) LoadBlockPart(height uint64, index int) *types.Part {
	return rawdb.ReadBlockPart(bc.db, height, index)
}

// LoadBlockMeta ...
func (bc *BlockChain) LoadBlockMeta(height uint64) *types.BlockMeta {
	return rawdb.ReadBlockMeta(bc.db, height)
}

// LoadBlockCommit ...
func (bc *BlockChain) LoadBlockCommit(height uint64) *types.Commit {
	return rawdb.ReadCommit(bc.db, height)
}

// LoadSeenCommit ...
func (bc *BlockChain) LoadSeenCommit(height uint64) *types.Commit {
	return rawdb.ReadSeenCommit(bc.db, height)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := rawdb.ReadBlock(bc.db, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByHeight retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByHeight(height uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(bc.db, height)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, height)
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	height := bc.hc.GetBlockHeight(hash)
	if height == nil {
		return nil
	}
	return bc.GetBlock(hash, *height)
}

// GetHeader retrieves a block header from the database by hash and height,
// caching it if found.
func (bc *BlockChain) GetHeader(hash common.Hash, height uint64) *types.Header {
	return bc.hc.GetHeader(hash, height)
}

// GetHeaderByHash retrieves a block header from the database by height, caching it if
// found.
func (bc *BlockChain) GetHeaderByHeight(height uint64) *types.Header {
	return bc.hc.GetHeaderByHeight(height)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return bc.hc.GetHeaderByHash(hash)
}

// State returns a new mutatable state at head block.
func (bc *BlockChain) State() (*state.StateDB, error) {
	return bc.StateAt(bc.CurrentBlock().Height())
}

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(height uint64) (*state.StateDB, error) {
	root := rawdb.ReadAppHash(bc.db, height)
	return state.New(root, bc.stateCache, bc.snaps)
}

// HasState checks if state trie is fully present in the database or not.
func (bc *BlockChain) HasState(hash common.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
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

func (bc *BlockChain) Processor() *StateProcessor {
	return bc.processor
}

func (bc *BlockChain) DB() kaidb.Database {
	return bc.db
}

// Config retrieves the blockchain's chain configuration.
func (bc *BlockChain) Config() *configs.ChainConfig { return bc.chainConfig }

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}
