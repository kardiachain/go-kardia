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
	"sync"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/types"

	"github.com/kardiachain/go-kardiamain/lib/log"
)

type StoreDB struct {
	fn string         // filename for reporting
	db kaidb.Database // LevelDB instance

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database

	log log.Logger // Contextual logger tracking the database path
}

// NewLDBStore returns a LevelDB wrapped object.
func NewStoreDB(db kaidb.Database) *StoreDB {
	return &StoreDB{
		db: db,
	}
}

// ReadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (s *StoreDB) ReadBlockMeta(hash common.Hash, height uint64) *types.BlockMeta {
	return ReadBlockMeta(s.db, hash, height)
}

// ReadBlock returns the Block for the given height
func (s *StoreDB) ReadBlock(hash common.Hash, height uint64) *types.Block {
	return ReadBlock(s.db, hash, height)
}

// ReadBlockPart returns the block part fo the given height and index
func (s *StoreDB) ReadBlockPart(hash common.Hash, height uint64, index int) *types.Part {
	return ReadBlockPart(s.db, hash, height, index)
}

// WriteBlock write block to database
func (s *StoreDB) WriteBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	WriteBlock(s.db, block, blockParts, seenCommit)
}

// WriteChainConfig writes the chain config settings to the database.
func (s *StoreDB) WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	CommonWriteChainConfig(s.db, hash, cfg)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (s *StoreDB) WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	CommonWriteReceipts(s.db, hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (s *StoreDB) WriteCanonicalHash(hash common.Hash, height uint64) {
	CommonWriteCanonicalHash(s.db, hash, height)
}

// WriteEvent stores KardiaSmartContract to db
func (s *StoreDB) WriteEvent(smc *types.KardiaSmartcontract) {
	CommonWriteEvent(s.db, smc)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (s *StoreDB) WriteTxLookupEntries(block *types.Block) {
	CommonWriteTxLookupEntries(s.db, block)
}

// Stores a hash into the database.
func (s *StoreDB) StoreHash(hash *common.Hash) {
	CommonStoreHash(s.db, hash)
}

// Stores a tx hash into the database.
func (s *StoreDB) StoreTxHash(hash *common.Hash) {
	CommonStoreTxHash(s.db, hash)
}

func (s *StoreDB) WriteHeadBlockHash(hash common.Hash) {
	CommonWriteHeadBlockHash(s.db, hash)
}

// ReadSmartContractAbi gets smart contract abi by smart contract address
func (s *StoreDB) ReadSmartContractAbi(address string) *abi.ABI {
	return CommonReadSmartContractAbi(s.db, address)
}

// ReadEvent gets watcher action by smart contract address and method
func (s *StoreDB) ReadEvent(address string, method string) *types.Watcher {
	return CommonReadEvent(s.db, address, method)
}

// ReadEvents returns a list of watcher action by smart contract address
func (s *StoreDB) ReadEvents(address string) (string, []*types.Watcher) {
	return CommonReadEvents(s.db, address)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (s *StoreDB) ReadCanonicalHash(height uint64) common.Hash {
	return CommonReadCanonicalHash(s.db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (s *StoreDB) ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return CommonReadChainConfig(s.db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (s *StoreDB) ReadBody(hash common.Hash, height uint64) *types.Body {
	return CommonReadBody(s.db, hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (s *StoreDB) ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadBodyRLP(s.db, hash, height)
}

func (s *StoreDB) DB() kaidb.Database {
	return s.db
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (s *StoreDB) ReadHeadBlockHash() common.Hash {
	return CommonReadHeadBlockHash(s.db)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (s *StoreDB) ReadHeadHeaderHash() common.Hash {
	return CommonReadHeadHeaderHash(s.db)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (s *StoreDB) ReadCommitRLP(height uint64) rlp.RawValue {
	return CommonReadCommitRLP(s.db, height)
}

// ReadBody retrieves the commit at a given height.
func (s *StoreDB) ReadCommit(height uint64) *types.Commit {
	return CommonReadCommit(s.db, height)
}

// ReadBody retrieves the commit at a given height.
func (s *StoreDB) ReadSeenCommit(height uint64) *types.Commit {
	return ReadSeenCommit(s.db, height)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (s *StoreDB) ReadHeader(hash common.Hash, height uint64) *types.Header {
	return CommonReadHeader(s.db, hash, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (s *StoreDB) ReadHeaderHeight(hash common.Hash) *uint64 {
	return CommonReadHeaderHeight(s.db, hash)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (s *StoreDB) ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return CommonReadTransaction(s.db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (s *StoreDB) ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadDualEventLookupEntry(s.db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (s *StoreDB) ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return CommonReadDualEvent(s.db, hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (s *StoreDB) ReadHeaderNumber(hash common.Hash) *uint64 {
	return CommonReadHeaderNumber(s.db, hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (s *StoreDB) ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return CommonReadReceipts(s.db, hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (s *StoreDB) ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadTxLookupEntry(s.db, hash)
}

// Returns true if a hash already exists in the database.
func (s *StoreDB) CheckHash(hash *common.Hash) bool {
	return CommonCheckHash(s.db, hash)
}

// Returns true if a tx hash already exists in the database.
func (s *StoreDB) CheckTxHash(hash *common.Hash) bool {
	return CommonCheckTxHash(s.db, hash)
}

// DeleteBody removes all block body data associated with a hash.
func (s *StoreDB) DeleteBody(hash common.Hash, height uint64) {
	CommonDeleteBody(s.db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (s *StoreDB) DeleteHeader(hash common.Hash, height uint64) {
	CommonDeleteHeader(s.db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (s *StoreDB) DeleteCanonicalHash(number uint64) {
	CommonDeleteCanonicalHash(s.db, number)
}

func (s *StoreDB) DeleteBlockMeta(hash common.Hash, height uint64) {
	s.db.Delete(blockMetaKey(hash, height))
}

func (s *StoreDB) DeleteBlockPart(hash common.Hash, height uint64) {
	blockMeta := s.ReadBlockMeta(hash, height)
	for i := 0; i < int(blockMeta.BlockID.PartsHeader.Total); i++ {
		s.db.Delete(blockPartKey(height, i))
	}
}
