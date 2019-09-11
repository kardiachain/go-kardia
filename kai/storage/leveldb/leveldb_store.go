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

package leveldb

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
	"sync"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type LDBStore struct {
	fn string      // filename for reporting
	db *leveldb.DB // LevelDB instance

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database

	log log.Logger // Contextual logger tracking the database path
}

// NewLDBStore returns a LevelDB wrapped object.
func NewLDBStore(file string, cache int, handles int) (*LDBStore, error) {
	logger := log.New("database", file)

	// Ensure we have some minimal caching and file guarantees
	if cache < 16 {
		cache = 16
	}
	if handles < 16 {
		handles = 16
	}
	logger.Info("Allocated cache and file handles", "cache", cache, "handles", handles)

	// Open the db and recover any potential corruptions
	db, err := leveldb.OpenFile(file, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	// (Re)check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}
	return &LDBStore{
		fn:  file,
		db:  db,
		log: logger,
	}, nil
}

// Path returns the path to the database directory.
func (db *LDBStore) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *LDBStore) Put(key, value interface{}) error {
	switch value.(type) {
	case rlp.RawValue:
		return db.db.Put(key.([]byte), value.(rlp.RawValue), nil)
	default:
		return db.db.Put(key.([]byte), value.([]byte), nil)
	}

}

// WriteBody stores a block body into the database.
func (db *LDBStore)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	CommonWriteBody(db, hash, height, body)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *LDBStore)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	CommonWriteBodyRLP(db, hash, height, rlp)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *LDBStore)WriteHeader(header *types.Header) {
	CommonWriteHeader(db, header)
}

// WriteChainConfig writes the chain config settings to the database.
func (db *LDBStore)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	CommonWriteChainConfig(db, hash, cfg)
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *LDBStore)WriteBlock(block *types.Block) {
	CommonWriteBlock(db, block)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *LDBStore)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	CommonWriteReceipts(db, hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *LDBStore)WriteCanonicalHash(hash common.Hash, height uint64) {
	CommonWriteCanonicalHash(db, hash, height)
}

// WriteHeadBlockHash stores the head block's hash.
func (db *LDBStore)WriteHeadBlockHash(hash common.Hash) {
	CommonWriteHeadBlockHash(db, hash)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *LDBStore)WriteHeadHeaderHash(hash common.Hash) {
	CommonWriteHeadHeaderHash(db, hash)
}

// WriteCommit stores a commit into the database.
func (db *LDBStore)WriteCommit(height uint64, commit *types.Commit) {
	CommonWriteCommit(db, height, commit)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *LDBStore)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	CommonWriteCommitRLP(db, height, rlp)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *LDBStore)WriteTxLookupEntries(block *types.Block) {
	CommonWriteTxLookupEntries(db, block)
}

// Stores a hash into the database.
func (db *LDBStore)StoreHash(hash *common.Hash) {
	CommonStoreHash(db, hash)
}

// Stores a tx hash into the database.
func (db *LDBStore)StoreTxHash(hash *common.Hash) {
	CommonStoreTxHash(db, hash)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *LDBStore)ReadCanonicalHash(height uint64) common.Hash {
	return CommonReadCanonicalHash(db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *LDBStore)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return CommonReadChainConfig(db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *LDBStore)ReadBody(hash common.Hash, height uint64) *types.Body {
	return CommonReadBody(db, hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *LDBStore)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadBodyRLP(db, hash, height)
}

func (db *LDBStore) LDB() *leveldb.DB {
	return db.db
}

func (db *LDBStore)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	return CommonReadBlock(logger, db, hash, height)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *LDBStore)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadHeaderRLP(db, hash, height)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *LDBStore)ReadHeadBlockHash() common.Hash {
	return CommonReadHeadBlockHash(db)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *LDBStore)ReadHeadHeaderHash() common.Hash {
	return CommonReadHeadHeaderHash(db)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *LDBStore)ReadCommitRLP(height uint64) rlp.RawValue {
	return CommonReadCommitRLP(db, height)
}

// ReadBody retrieves the commit at a given height.
func (db *LDBStore)ReadCommit(height uint64) *types.Commit {
	return CommonReadCommit(db, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *LDBStore)ReadHeaderHeight(hash common.Hash) *uint64 {
	return CommonReadHeaderHeight(db, hash)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *LDBStore)ReadHeader(hash common.Hash, height uint64) *types.Header {
	return CommonReadHeader(db, hash, height)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *LDBStore)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return CommonReadTransaction(db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *LDBStore)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadDualEventLookupEntry(db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *LDBStore)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return CommonReadDualEvent(db, hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *LDBStore)ReadHeaderNumber(hash common.Hash) *uint64 {
	return CommonReadHeaderNumber(db, hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *LDBStore)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return CommonReadReceipts(db, hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *LDBStore)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadTxLookupEntry(db, hash)
}

// Returns true if a hash already exists in the database.
func (db *LDBStore)CheckHash(hash *common.Hash) bool {
	return CommonCheckHash(db, hash)
}

// Returns true if a tx hash already exists in the database.
func (db *LDBStore)CheckTxHash(hash *common.Hash) bool {
	return CommonCheckTxHash(db, hash)
}

// DeleteBody removes all block body data associated with a hash.
func (db *LDBStore)DeleteBody(hash common.Hash, height uint64) {
	CommonDeleteBody(db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (db *LDBStore)DeleteHeader(hash common.Hash, height uint64) {
	CommonDeleteHeader(db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *LDBStore)DeleteCanonicalHash(number uint64) {
	CommonDeleteCanonicalHash(db, number)
}

func (db *LDBStore) Has(key interface{}) (bool, error) {
	return db.db.Has(key.([]byte), nil)
}

// Get returns the given key if it's present.
func (db *LDBStore) Get(key interface{}) (interface{}, error) {
	dat, err := db.db.Get(key.([]byte), nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// Delete deletes the key from the queue and database
func (db *LDBStore) Delete(key interface{}) error {
	return db.db.Delete(key.([]byte), nil)
}

func (db *LDBStore) NewIterator() iterator.Iterator {
	return db.db.NewIterator(nil, nil)
}

func (db *LDBStore) Close() {
	// Stop the metrics collection to avoid internal database races
	db.quitLock.Lock()
	defer db.quitLock.Unlock()

	if db.quitChan != nil {
		errc := make(chan error)
		db.quitChan <- errc
		if err := <-errc; err != nil {
			db.log.Error("Metrics collection failed", "err", err)
		}
		db.quitChan = nil
	}
	err := db.db.Close()
	if err == nil {
		db.log.Info("Database closed")
	} else {
		db.log.Error("Failed to close database", "err", err)
	}
}

func (db *LDBStore) NewBatch() types.Batch {
	return &ldbBatch{db: db.db, b: new(leveldb.Batch)}
}

type ldbBatch struct {
	db   *leveldb.DB
	b    *leveldb.Batch
	size int
}

func (b *ldbBatch) Put(key, value interface{}) error {
	switch value.(type) {
	case rlp.RawValue:
		b.b.Put(key.([]byte), value.(rlp.RawValue))
		b.size += len(value.(rlp.RawValue))
	default:
		b.b.Put(key.([]byte), value.([]byte))
		b.size += len(value.([]byte))
	}
	return nil
}

func (b *ldbBatch) Has(key interface{}) (bool, error) {
	return b.db.Has(key.([]byte), nil)
}

// Get returns the given key if it's present.
func (b *ldbBatch) Get(key interface{}) (interface{}, error) {
	dat, err := b.db.Get(key.([]byte), nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// WriteBody stores a block body into the database.
func (db *ldbBatch)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	CommonWriteBody(db, hash, height, body)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *ldbBatch)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	CommonWriteBodyRLP(db, hash, height, rlp)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *ldbBatch)WriteHeader(header *types.Header) {
	CommonWriteHeader(db, header)
}

// WriteChainConfig writes the chain config settings to the database.
func (db *ldbBatch)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	CommonWriteChainConfig(db, hash, cfg)
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *ldbBatch)WriteBlock(block *types.Block) {
	CommonWriteBlock(db, block)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *ldbBatch)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	CommonWriteReceipts(db, hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *ldbBatch)WriteCanonicalHash(hash common.Hash, height uint64) {
	CommonWriteCanonicalHash(db, hash, height)
}

// WriteHeadBlockHash stores the head block's hash.
func (db *ldbBatch)WriteHeadBlockHash(hash common.Hash) {
	CommonWriteHeadBlockHash(db, hash)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *ldbBatch)WriteHeadHeaderHash(hash common.Hash) {
	CommonWriteHeadHeaderHash(db, hash)
}

// WriteCommit stores a commit into the database.
func (db *ldbBatch)WriteCommit(height uint64, commit *types.Commit) {
	CommonWriteCommit(db, height, commit)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *ldbBatch)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	CommonWriteCommitRLP(db, height, rlp)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *ldbBatch)WriteTxLookupEntries(block *types.Block) {
	CommonWriteTxLookupEntries(db, block)
}

// Stores a hash into the database.
func (db *ldbBatch)StoreHash(hash *common.Hash) {
	CommonStoreHash(db, hash)
}

// Stores a tx hash into the database.
func (db *ldbBatch)StoreTxHash(hash *common.Hash) {
	CommonStoreTxHash(db, hash)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *ldbBatch)ReadCanonicalHash(height uint64) common.Hash {
	return CommonReadCanonicalHash(db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *ldbBatch)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return CommonReadChainConfig(db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *ldbBatch)ReadBody(hash common.Hash, height uint64) *types.Body {
	return CommonReadBody(db, hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *ldbBatch)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadBodyRLP(db, hash, height)
}

func (db *ldbBatch) LDB() *leveldb.DB {
	return db.db
}

func (db *ldbBatch)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	return CommonReadBlock(logger, db, hash, height)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *ldbBatch)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadHeaderRLP(db, hash, height)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *ldbBatch)ReadHeadBlockHash() common.Hash {
	return CommonReadHeadBlockHash(db)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *ldbBatch)ReadHeadHeaderHash() common.Hash {
	return CommonReadHeadHeaderHash(db)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *ldbBatch)ReadCommitRLP(height uint64) rlp.RawValue {
	return CommonReadCommitRLP(db, height)
}

// ReadBody retrieves the commit at a given height.
func (db *ldbBatch)ReadCommit(height uint64) *types.Commit {
	return CommonReadCommit(db, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *ldbBatch)ReadHeaderHeight(hash common.Hash) *uint64 {
	return CommonReadHeaderHeight(db, hash)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *ldbBatch)ReadHeader(hash common.Hash, height uint64) *types.Header {
	return CommonReadHeader(db, hash, height)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *ldbBatch)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return CommonReadTransaction(db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *ldbBatch)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadDualEventLookupEntry(db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *ldbBatch)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return CommonReadDualEvent(db, hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *ldbBatch)ReadHeaderNumber(hash common.Hash) *uint64 {
	return CommonReadHeaderNumber(db, hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *ldbBatch)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return CommonReadReceipts(db, hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *ldbBatch)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadTxLookupEntry(db, hash)
}

// Returns true if a hash already exists in the database.
func (db *ldbBatch)CheckHash(hash *common.Hash) bool {
	return CommonCheckHash(db, hash)
}

// Returns true if a tx hash already exists in the database.
func (db *ldbBatch)CheckTxHash(hash *common.Hash) bool {
	return CommonCheckTxHash(db, hash)
}

// DeleteBody removes all block body data associated with a hash.
func (db *ldbBatch)DeleteBody(hash common.Hash, height uint64) {
	CommonDeleteBody(db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (db *ldbBatch)DeleteHeader(hash common.Hash, height uint64) {
	CommonDeleteHeader(db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *ldbBatch)DeleteCanonicalHash(number uint64) {
	CommonDeleteCanonicalHash(db, number)
}

func (b *ldbBatch) Delete(key interface{}) error {
	b.b.Delete(key.([]byte))
	b.size += 1
	return nil
}

func (b *ldbBatch) Write() error {
	return b.db.Write(b.b, nil)
}

func (b *ldbBatch) ValueSize() int {
	return b.size
}

func (b *ldbBatch) Reset() {
	b.b.Reset()
	b.size = 0
}
