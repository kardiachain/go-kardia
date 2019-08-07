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

package storage

import (
	"errors"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
	"sync"

	"github.com/kardiachain/go-kardia/lib/common"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type MemStore struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func NewMemStore() *MemStore {
	return &MemStore{
		db: make(map[string][]byte),
	}
}

func NewMemStoreWithCap(size int) *MemStore {
	return &MemStore{
		db: make(map[string][]byte, size),
	}
}

func (db *MemStore) Put(key, value interface{}) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.db[string(key.([]byte))] = common.CopyBytes(value.([]byte))
	return nil
}

func (db *MemStore) Has(key interface{}) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.db[string(key.([]byte))]
	return ok, nil
}

func (db *MemStore) Get(key interface{}) (interface{}, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if entry, ok := db.db[string(key.([]byte))]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, errors.New("not found")
}

func (db *MemStore) Keys() [][]byte {
	db.lock.RLock()
	defer db.lock.RUnlock()

	keys := [][]byte{}
	for key := range db.db {
		keys = append(keys, []byte(key))
	}
	return keys
}

func (db *MemStore) Delete(key interface{}) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	delete(db.db, string(key.([]byte)))
	return nil
}

func (db *MemStore) Close() {}

func (db *MemStore) NewBatch() types.Batch {
	return &memBatch{db: db}
}

func (db *MemStore) Len() int { return len(db.db) }

// WriteBody stores a block body into the database.
func (db *MemStore)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	CommonWriteBody(db, hash, height, body)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *MemStore)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	CommonWriteBodyRLP(db, hash, height, rlp)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *MemStore)WriteHeader(header *types.Header) {
	CommonWriteHeader(db, header)
}

// WriteChainConfig writes the chain config settings to the database.
func (db *MemStore)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	CommonWriteChainConfig(db, hash, cfg)
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *MemStore)WriteBlock(block *types.Block) {
	CommonWriteBlock(db, block)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *MemStore)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	CommonWriteReceipts(db, hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *MemStore)WriteCanonicalHash(hash common.Hash, height uint64) {
	CommonWriteCanonicalHash(db, hash, height)
}

// WriteHeadBlockHash stores the head block's hash.
func (db *MemStore)WriteHeadBlockHash(hash common.Hash) {
	CommonWriteHeadBlockHash(db, hash)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *MemStore)WriteHeadHeaderHash(hash common.Hash) {
	CommonWriteHeadHeaderHash(db, hash)
}

// WriteCommit stores a commit into the database.
func (db *MemStore)WriteCommit(height uint64, commit *types.Commit) {
	CommonWriteCommit(db, height, commit)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *MemStore)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	CommonWriteCommitRLP(db, height, rlp)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *MemStore)WriteTxLookupEntries(block *types.Block) {
	CommonWriteTxLookupEntries(db, block)
}

// Stores a hash into the database.
func (db *MemStore)StoreHash(hash *common.Hash) {
	CommonStoreHash(db, hash)
}

// Stores a tx hash into the database.
func (db *MemStore)StoreTxHash(hash *common.Hash) {
	CommonStoreTxHash(db, hash)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *MemStore)ReadCanonicalHash(height uint64) common.Hash {
	return CommonReadCanonicalHash(db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *MemStore)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return CommonReadChainConfig(db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *MemStore)ReadBody(hash common.Hash, height uint64) *types.Body {
	return CommonReadBody(db, hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *MemStore)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadBodyRLP(db, hash, height)
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *MemStore)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	return CommonReadBlock(logger, db, hash, height)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *MemStore)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadHeaderRLP(db, hash, height)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *MemStore)ReadHeadBlockHash() common.Hash {
	return CommonReadHeadBlockHash(db)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *MemStore)ReadHeadHeaderHash() common.Hash {
	return CommonReadHeadHeaderHash(db)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *MemStore)ReadCommitRLP(height uint64) rlp.RawValue {
	return CommonReadCommitRLP(db, height)
}

// ReadBody retrieves the commit at a given height.
func (db *MemStore)ReadCommit(height uint64) *types.Commit {
	return CommonReadCommit(db, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *MemStore)ReadHeaderHeight(hash common.Hash) *uint64 {
	return CommonReadHeaderHeight(db, hash)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *MemStore)ReadHeader(hash common.Hash, height uint64) *types.Header {
	return CommonReadHeader(db, hash, height)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *MemStore)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return CommonReadTransaction(db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *MemStore)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadDualEventLookupEntry(db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *MemStore)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return CommonReadDualEvent(db, hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *MemStore)ReadHeaderNumber(hash common.Hash) *uint64 {
	return CommonReadHeaderNumber(db, hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *MemStore)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return CommonReadReceipts(db, hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *MemStore)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadTxLookupEntry(db, hash)
}

// Returns true if a hash already exists in the database.
func (db *MemStore)CheckHash(hash *common.Hash) bool {
	return CommonCheckHash(db, hash)
}

// Returns true if a tx hash already exists in the database.
func (db *MemStore)CheckTxHash(hash *common.Hash) bool {
	return CommonCheckTxHash(db, hash)
}

// DeleteBody removes all block body data associated with a hash.
func (db *MemStore)DeleteBody(hash common.Hash, height uint64) {
	CommonDeleteBody(db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (db *MemStore)DeleteHeader(hash common.Hash, height uint64) {
	CommonDeleteHeader(db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *MemStore)DeleteCanonicalHash(number uint64) {
	CommonDeleteCanonicalHash(db, number)
}

type kv struct{ k, v []byte }

type memBatch struct {
	db     *MemStore
	writes []kv
	size   int
}

func (b *memBatch) Put(key, value interface{}) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key.([]byte)), common.CopyBytes(value.([]byte))})
	b.size += len(value.([]byte))
	return nil
}

func (b *memBatch) Delete(key interface{}) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key.([]byte)), nil})
	return nil
}

func (b *memBatch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, kv := range b.writes {
		if kv.v == nil {
			delete(b.db.db, string(kv.k))
			continue
		}
		b.db.db[string(kv.k)] = kv.v
	}
	return nil
}

func (b *memBatch) ValueSize() int {
	return b.size
}

func (b *memBatch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

func (db *memBatch) Has(key interface{}) (bool, error) {
	db.db.lock.RLock()
	defer db.db.lock.RUnlock()

	return db.db.Has(key.([]byte))
}

func (db *memBatch) Get(key interface{}) (interface{}, error) {
	db.db.lock.RLock()
	defer db.db.lock.RUnlock()

	return db.db.Get(key.([]byte))
}

// WriteBody stores a block body into the database.
func (db *memBatch)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	CommonWriteBody(db, hash, height, body)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *memBatch)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	CommonWriteBodyRLP(db, hash, height, rlp)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *memBatch)WriteHeader(header *types.Header) {
	CommonWriteHeader(db, header)
}

// WriteChainConfig writes the chain config settings to the database.
func (db *memBatch)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	CommonWriteChainConfig(db, hash, cfg)
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *memBatch)WriteBlock(block *types.Block) {
	CommonWriteBlock(db, block)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *memBatch)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	CommonWriteReceipts(db, hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *memBatch)WriteCanonicalHash(hash common.Hash, height uint64) {
	CommonWriteCanonicalHash(db, hash, height)
}

// WriteHeadBlockHash stores the head block's hash.
func (db *memBatch)WriteHeadBlockHash(hash common.Hash) {
	CommonWriteHeadBlockHash(db, hash)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *memBatch)WriteHeadHeaderHash(hash common.Hash) {
	CommonWriteHeadHeaderHash(db, hash)
}

// WriteCommit stores a commit into the database.
func (db *memBatch)WriteCommit(height uint64, commit *types.Commit) {
	CommonWriteCommit(db, height, commit)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *memBatch)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	CommonWriteCommitRLP(db, height, rlp)
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *memBatch)WriteTxLookupEntries(block *types.Block) {
	CommonWriteTxLookupEntries(db, block)
}

// Stores a hash into the database.
func (db *memBatch)StoreHash(hash *common.Hash) {
	CommonStoreHash(db, hash)
}

// Stores a tx hash into the database.
func (db *memBatch)StoreTxHash(hash *common.Hash) {
	CommonStoreTxHash(db, hash)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *memBatch)ReadCanonicalHash(height uint64) common.Hash {
	return CommonReadCanonicalHash(db, height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *memBatch)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return CommonReadChainConfig(db, hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *memBatch)ReadBody(hash common.Hash, height uint64) *types.Body {
	return CommonReadBody(db, hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *memBatch)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadBodyRLP(db, hash, height)
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *memBatch)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	return CommonReadBlock(logger, db, hash, height)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *memBatch)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	return CommonReadHeaderRLP(db, hash, height)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *memBatch)ReadHeadBlockHash() common.Hash {
	return CommonReadHeadBlockHash(db)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *memBatch)ReadHeadHeaderHash() common.Hash {
	return CommonReadHeadHeaderHash(db)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *memBatch)ReadCommitRLP(height uint64) rlp.RawValue {
	return CommonReadCommitRLP(db, height)
}

// ReadBody retrieves the commit at a given height.
func (db *memBatch)ReadCommit(height uint64) *types.Commit {
	return CommonReadCommit(db, height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *memBatch)ReadHeaderHeight(hash common.Hash) *uint64 {
	return CommonReadHeaderHeight(db, hash)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *memBatch)ReadHeader(hash common.Hash, height uint64) *types.Header {
	return CommonReadHeader(db, hash, height)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *memBatch)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return CommonReadTransaction(db, hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *memBatch)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadDualEventLookupEntry(db, hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *memBatch)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return CommonReadDualEvent(db, hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *memBatch)ReadHeaderNumber(hash common.Hash) *uint64 {
	return CommonReadHeaderNumber(db, hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *memBatch)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return CommonReadReceipts(db, hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *memBatch)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return CommonReadTxLookupEntry(db, hash)
}

// Returns true if a hash already exists in the database.
func (db *memBatch)CheckHash(hash *common.Hash) bool {
	return CommonCheckHash(db, hash)
}

// Returns true if a tx hash already exists in the database.
func (db *memBatch)CheckTxHash(hash *common.Hash) bool {
	return CommonCheckTxHash(db, hash)
}

// DeleteBody removes all block body data associated with a hash.
func (db *memBatch)DeleteBody(hash common.Hash, height uint64) {
	CommonDeleteBody(db, hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (db *memBatch)DeleteHeader(hash common.Hash, height uint64) {
	CommonDeleteHeader(db, hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *memBatch)DeleteCanonicalHash(number uint64) {
	CommonDeleteCanonicalHash(db, number)
}
