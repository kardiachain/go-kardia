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

package kvstore

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/accounts/abi"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
)

type StoreDB struct {
	db kaidb.Database // LevelDB instance
}

// NewLDBStore returns a LevelDB wrapped object.
func NewStoreDB(db kaidb.Database) *StoreDB {
	return &StoreDB{
		db: db,
	}
}

// ReadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (s *StoreDB) ReadBlockMeta(height uint64) *types.BlockMeta {
	return ReadBlockMeta(s.db, height)
}

// ReadBlock returns the Block for the given height
func (s *StoreDB) ReadBlock(height uint64) *types.Block {
	return ReadBlock(s.db, height)
}

// ReadBlockPart returns the block part fo the given height and index
func (s *StoreDB) ReadBlockPart(height uint64, index int) *types.Part {
	return ReadBlockPart(s.db, height, index)
}

// WriteBlock write block to database
func (s *StoreDB) WriteBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	WriteBlock(s.db, block, blockParts, seenCommit)
}

// WriteChainConfig writes the chain config settings to the database.
func (s *StoreDB) WriteChainConfig(hash common.Hash, cfg *configs.ChainConfig) {
	WriteChainConfig(s.db, hash, cfg)
}

// WriteBlockInfo stores block info belonging to a block.
func (s *StoreDB) WriteBlockInfo(hash common.Hash, height uint64, blockInfo *types.BlockInfo) {
	WriteBlockInfo(s.db, hash, height, blockInfo)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (s *StoreDB) WriteCanonicalHash(hash common.Hash, height uint64) {
	WriteCanonicalHash(s.db, hash, height)
}

// WriteEvent stores KardiaSmartContract to db
func (s *StoreDB) WriteEvent(smc *types.KardiaSmartcontract) {
	WriteEvent(s.db, smc)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (s *StoreDB) WriteTxLookupEntries(block *types.Block, receipts types.Receipts) {
	WriteTxLookupEntries(s.db, block, receipts)
}

// WriteHeadBlockHash stores head blockhash to db
func (s *StoreDB) WriteHeadBlockHash(hash common.Hash) {
	WriteHeadBlockHash(s.db, hash)
}

// WriteAppHash stores app hash to db
func (s *StoreDB) WriteAppHash(height uint64, hash common.Hash) {
	WriteAppHash(s.db, height, hash)
}

// ReadSmartContractAbi gets smart contract abi by smart contract address
func (s *StoreDB) ReadSmartContractAbi(address string) *abi.ABI {
	return ReadSmartContractAbi(s.db, address)
}

// ReadEvent gets watcher action by smart contract address and method
func (s *StoreDB) ReadEvent(address string, method string) *types.Watcher {
	return ReadEvent(s.db, address, method)
}

// ReadEvents returns a list of watcher action by smart contract address
func (s *StoreDB) ReadEvents(address string) (string, []*types.Watcher) {
	return ReadEvents(s.db, address)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (s *StoreDB) ReadCanonicalHash(height uint64) common.Hash {
	return ReadCanonicalHash(s.db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (s *StoreDB) ReadChainConfig(hash common.Hash) *configs.ChainConfig {
	return ReadChainConfig(s.db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (s *StoreDB) ReadBody(height uint64) *types.Body {
	return ReadBody(s.db, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (s *StoreDB) ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return ReadBodyRLP(s.db, hash, height)
}

func (s *StoreDB) DB() kaidb.Database {
	return s.db
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (s *StoreDB) ReadHeadBlockHash() common.Hash {
	return ReadHeadBlockHash(s.db)
}

// ReadBody retrieves the commit at a given height.
func (s *StoreDB) ReadCommit(height uint64) *types.Commit {
	return ReadCommit(s.db, height)
}

// ReadBody retrieves the commit at a given height.
func (s *StoreDB) ReadSeenCommit(height uint64) *types.Commit {
	return ReadSeenCommit(s.db, height)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (s *StoreDB) ReadHeader(height uint64) *types.Header {
	return ReadHeader(s.db, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (s *StoreDB) ReadHeaderHeight(hash common.Hash) *uint64 {
	return ReadHeaderHeight(s.db, hash)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (s *StoreDB) ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return ReadTransaction(s.db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (s *StoreDB) ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return ReadDualEventLookupEntry(s.db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (s *StoreDB) ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return ReadDualEvent(s.db, hash)
}

// ReadBlockInfo retrieves block info belonging to a block.
func (s *StoreDB) ReadBlockInfo(hash common.Hash, number uint64, config *configs.ChainConfig) *types.BlockInfo {
	return ReadBlockInfo(s.db, hash, number, config)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (s *StoreDB) ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return ReadTxLookupEntry(s.db, hash)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (s *StoreDB) ReadAppHash(height uint64) common.Hash {
	return ReadAppHash(s.db, height)
}

// DeleteBody removes all block body data associated with a hash.
func (s *StoreDB) DeleteBody(hash common.Hash, height uint64) {
	DeleteBody(s.db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (s *StoreDB) DeleteHeader(hash common.Hash, height uint64) {
	DeleteHeader(s.db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (s *StoreDB) DeleteCanonicalHash(number uint64) {
	DeleteCanonicalHash(s.db, number)
}

func (s *StoreDB) DeleteBlockMeta(height uint64) error {
	if err := s.db.Delete(blockMetaKey(height)); err != nil {
		return err
	}
	return nil
}

func (s *StoreDB) DeleteBlockPart(height uint64) error {
	blockMeta := s.ReadBlockMeta(height)
	for i := 0; i < int(blockMeta.BlockID.PartsHeader.Total); i++ {
		if err := s.db.Delete(blockPartKey(height, i)); err != nil {
			return err
		}
	}
	return nil
}
