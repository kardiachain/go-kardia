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

package types

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"strings"
	"sync"

	"github.com/kardiachain/go-kardia/lib/common"
)

type smartContract struct {
	Address string
	ABI     string
}

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

	switch value.(type) {
	case rlp.RawValue:
		db.db[string(key.([]byte))] = common.CopyBytes(value.(rlp.RawValue))
	default:
		db.db[string(key.([]byte))] = common.CopyBytes(value.([]byte))
	}
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

func (db *MemStore) NewBatch() Batch {
	return &memBatch{db: db}
}

func (db *MemStore) Len() int { return len(db.db) }

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *MemStore) ReadCanonicalHash(height uint64) common.Hash {
	data, _ := db.Get(headerHashKey(height))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *MemStore) ReadChainConfig(hash common.Hash) *ChainConfig {
	data, _ := db.Get(configKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	var config ChainConfig
	if err := json.Unmarshal(data.([]byte), &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func (db *MemStore) WriteChainConfig(hash common.Hash, cfg *ChainConfig) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Crit("Failed to JSON encode chain config", "err", err)
	}
	if err := db.Put(configKey(hash), data); err != nil {
		log.Crit("Failed to store chain config", "err", err)
	}
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *MemStore) WriteBlock(block *Block) {
	db.WriteBody(block.Hash(), block.Height(),block.Body())
	db.WriteHeader(block.Header())
}

// WriteBody stores a block body into the database.
func (db *MemStore) WriteBody(hash common.Hash, height uint64, body *Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	db.WriteBodyRLP(hash, height, data)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *MemStore) WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	if err := db.Put(blockBodyKey(height, hash), rlp); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *MemStore) WriteHeader(header *Header) {
	// Write the hash -> height mapping
	var (
		hash    = header.Hash()
		height  = header.Height
		encoded = encodeBlockHeight(height)
	)
	key := headerHeightKey(hash)
	if err := db.Put(key, encoded); err != nil {
		log.Crit("Failed to store hash to height mapping", "err", err)
	}
	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	key = headerKey(height, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *MemStore) WriteReceipts(hash common.Hash, height uint64, receipts Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(height, hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *MemStore) WriteCanonicalHash(hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// WriteHeadBlockHash stores the head block's hash.
func (db *MemStore) WriteHeadBlockHash(hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *MemStore) WriteHeadHeaderHash(hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// WriteCommit stores a commit into the database.
func (db *MemStore) WriteCommit(height uint64, commit *Commit) {
	data, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Crit("Failed to RLP encode commit", "err", err)
	}
	db.WriteCommitRLP(height, data)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *MemStore) WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	if err := db.Put(commitKey(height), rlp); err != nil {
		log.Crit("Failed to store commit", "err", err)
	}
}

func (db *MemStore) WriteEvent(smc *KardiaSmartcontract) {
	address := smc.SmcAddress

	// Write contract abi
	smartContract := smartContract{
		Address: smc.SmcAddress,
		ABI:     smc.SmcAbi,
	}
	encodedData, err := rlp.EncodeToBytes(smartContract)
	if err != nil {
		log.Error("failed to encode smartContract Data")
	}
	if err := db.Put(contractAbiKey(address), encodedData); err != nil {
		log.Crit("Failed to store dualAction", "err", err)
	}

	// Add dual actions
	// Note: dual action must be unique and be generated by kvm when apply KSML
	for _, action := range smc.DualActions {
		if err := db.Put(dualActionKey(action.Name), contractAbiKey(address)); err != nil {
			log.Crit("Failed to store dualAction", "err", err)
		}
	}

	// Add watcher action to db
	for _, event := range smc.WatcherActions {
		method := event.Method
		data, err := rlp.EncodeToBytes(event)
		if err != nil {
			log.Crit("Failed to encode event", "err", err, "method", method, "contract", smc.SmcAddress)
		}
		key := eventKey(address, method)
		if err := db.Put(key, data); err != nil {
			log.Crit("Failed to store last header's hash", "err", err)
		}
	}
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *MemStore) ReadBlock(logger log.Logger, hash common.Hash, height uint64) *Block {
	header := db.ReadHeader(hash, height)
	if header == nil {
		return nil
	}
	body := db.ReadBody(hash, height)
	if body == nil {
		return nil
	}
	return NewBlockWithHeader(logger, header).WithBody(body)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *MemStore) ReadHeader(hash common.Hash, height uint64) *Header {
	data := db.ReadHeaderRLP(hash, height)
	if len(data) == 0 {
		return nil
	}
	header := new(Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return header
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *MemStore) ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(height, hash))
	return data.([]byte)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *MemStore) ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data.([]byte)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *MemStore) ReadBody(hash common.Hash, height uint64) *Body {
	data := db.ReadBodyRLP(hash, height)
	if len(data) == 0 {
		return nil
	}
	body := new(Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		log.Error("Invalid block body RLP", "hash", hash, "err", err)
		return nil
	}

	return body
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *MemStore) ReadHeadBlockHash() common.Hash {
	data, _ := db.Get(headBlockKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *MemStore) ReadHeaderHeight(hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data.([]byte))
	return &height
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *MemStore) ReadHeadHeaderHash() common.Hash {
	data, _ := db.Get(headHeaderKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *MemStore) ReadCommitRLP(height uint64) rlp.RawValue {
	data, _ := db.Get(commitKey(height))
	return data.([]byte)
}

// ReadBody retrieves the commit at a given height.
func (db *MemStore) ReadCommit(height uint64) *Commit {
	data := db.ReadCommitRLP(height)
	if len(data) == 0 {
		return nil
	}
	commit := new(Commit)
	if err := rlp.Decode(bytes.NewReader(data), commit); err != nil {
		log.Error("Invalid commit RLP", "err", err)
		return nil
	}
	commit.MakeEmptyNil()
	return commit
}

// DeleteBody removes all block body data associated with a hash.
func (db *MemStore) DeleteBody(hash common.Hash, height uint64) {
	if err := db.Delete(blockBodyKey(height, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// DeleteHeader removes all block header data associated with a hash.
func (db *MemStore) DeleteHeader(hash common.Hash, height uint64) {
	if err := db.Delete(headerKey(height, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerHeightKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *MemStore) DeleteCanonicalHash(number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *MemStore) ReadReceipts(hash common.Hash, number uint64) Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	// Convert the revceipts from their storage form to their internal representation
	storageReceipts := []*ReceiptForStorage{}
	if err := rlp.DecodeBytes(data.([]byte), &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*Receipt)(receipt)
	}
	return receipts
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *MemStore) ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}


func (db *MemStore) ReadEvent(address string, method string) *WatcherAction {
	data, err := db.Get(eventKey(address, method))
	if err != nil {
		log.Error("error while get event", "err", err, "address", address, "method", method)
		return nil
	}
	var entry WatcherAction
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}
	return &entry
}

func (db *MemStore) ReadEvents(address string) []*WatcherAction {
	data, err := db.Get(eventsKey(address))
	if err != nil {
		log.Error("error while get event", "err", err, "address", address)
		return nil
	}
	var entries []string
	if err := rlp.DecodeBytes(data.([]byte), &entries); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}

	watcherActions := make([]*WatcherAction, 0)
	if len(entries) > 0 {
		for _, entry := range entries {
			// get watched event from entry
			evtData, err := db.Get(common.Hex2Bytes(entry))
			if err != nil {
				log.Error("Cannot get event data", "err", err, "eventData", entry)
				continue
			}
			var action WatcherAction
			if err := rlp.DecodeBytes(evtData.([]byte), &action); err != nil {
				log.Error("Invalid watcherAction", "err", err)
				continue
			}
			watcherActions = append(watcherActions, &action)
		}
	}
	return watcherActions
}

func (db *MemStore) ReadSmartContractFromDualAction(action string) (string, *abi.ABI) {
	key, err := db.Get(dualActionKey(action))
	if err != nil || key == nil {
		return "", nil
	}

	data, err := db.Get(key)
	if err != nil || data == nil {
		return "", nil
	}

	var entry smartContract
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return "", nil
	}

	a, err := abi.JSON(strings.NewReader(entry.ABI))
	if err != nil {
		return "", nil
	}
	return entry.Address, &a
}


func (db *MemStore) ReadSmartContractAbi(address string) *abi.ABI {
	data, err := db.Get(contractAbiKey(address))
	if err != nil || data == nil {
		log.Error("error while get abi from contract address", "err", err, "address", address)
		return nil
	}
	abiStr := string(data.([]byte))
	a, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		log.Error("cannot get abi from smart contract", "err", err, "address", address)
		return nil
	}
	return &a
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *MemStore) WriteTxLookupEntries(block *Block) {
	for i, tx := range block.Transactions() {
		entry := TxLookupEntry{
			BlockHash:  block.Hash(),
			BlockIndex: block.Height(),
			Index:      uint64(i),
		}
		data, err := rlp.EncodeToBytes(entry)
		if err != nil {
			log.Crit("Failed to encode transaction lookup entry", "err", err)
		}
		if err := db.Put(txLookupKey(tx.Hash()), data); err != nil {
			log.Crit("Failed to store transaction lookup entry", "err", err)
		}
	}
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func (db *MemStore) DeleteTxLookupEntry(hash common.Hash) {
	db.Delete(txLookupKey(hash))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *MemStore) ReadTransaction(hash common.Hash) (*Transaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := db.ReadTxLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := db.ReadBody(blockHash, blockNumber)
	if body == nil || len(body.Transactions) <= int(txIndex) {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash, "index", txIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.Transactions[txIndex], blockHash, blockNumber, txIndex
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *MemStore) ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(dualEventLookupKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry DualEventLookupEntry
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid dual's event lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *MemStore) ReadDualEvent(hash common.Hash) (*DualEvent, common.Hash, uint64, uint64) {
	blockHash, blockNumber, eventIndex := db.ReadDualEventLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := db.ReadBody(blockHash, blockNumber)
	if body == nil || len(body.DualEvents) <= int(eventIndex) {
		log.Error("Dual event referenced missing", "number", blockNumber, "hash", blockHash, "index", eventIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.DualEvents[eventIndex], blockHash, blockNumber, eventIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func (db *MemStore) ReadReceipt(hash common.Hash) (*Receipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := db.ReadTxLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	receipts := db.ReadReceipts(blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", "number", blockNumber, "hash", blockHash, "index", receiptIndex)
		return nil, common.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func (db *MemStore) ReadBloomBits(bit uint, section uint64, head common.Hash) ([]byte, error) {
	data, err := db.Get(bloomBitsKey(bit, section, head))
	if err != nil || data == nil || len(data.([]byte)) == 0 {
		return nil, err
	}
	return data.([]byte), err
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func (db *MemStore) WriteBloomBits(bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *MemStore) ReadHeaderNumber(hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data.([]byte))
	return &number
}

// Stores a hash into the database.
func (db *MemStore) StoreHash(hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a hash already exists in the database.
func (db *MemStore) CheckHash(hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}

// Stores a tx hash into the database.
func (db *MemStore) StoreTxHash(hash *common.Hash) {
	if err := db.Put(txHashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a tx hash already exists in the database.
func (db *MemStore) CheckTxHash(hash *common.Hash) bool {
	data, _ := db.Get(txHashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}

type keyValue struct{ k, v []byte }

type memBatch struct {
	db     *MemStore
	writes []keyValue
	size   int
}

func (b *memBatch) Put(key, value interface{}) error {
	b.writes = append(b.writes, keyValue{common.CopyBytes(key.([]byte)), common.CopyBytes(value.([]byte))})
	b.size += len(value.([]byte))
	return nil
}

func (b *memBatch) Delete(key interface{}) error {
	b.writes = append(b.writes, keyValue{common.CopyBytes(key.([]byte)), nil})
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

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *memBatch) ReadCanonicalHash(height uint64) common.Hash {
	data, _ := db.Get(headerHashKey(height))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *memBatch) ReadChainConfig(hash common.Hash) *ChainConfig {
	data, _ := db.Get(configKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	var config ChainConfig
	if err := json.Unmarshal(data.([]byte), &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func (db *memBatch) WriteChainConfig(hash common.Hash, cfg *ChainConfig) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Crit("Failed to JSON encode chain config", "err", err)
	}
	if err := db.Put(configKey(hash), data); err != nil {
		log.Crit("Failed to store chain config", "err", err)
	}
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *memBatch) WriteBlock(block *Block) {
	db.WriteBody(block.Hash(), block.Height(),block.Body())
	db.WriteHeader(block.Header())
}

// WriteBody stores a block body into the database.
func (db *memBatch) WriteBody(hash common.Hash, height uint64, body *Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	db.WriteBodyRLP(hash, height, data)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *memBatch) WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	if err := db.Put(blockBodyKey(height, hash), rlp); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *memBatch) WriteHeader(header *Header) {
	// Write the hash -> height mapping
	var (
		hash    = header.Hash()
		height  = header.Height
		encoded = encodeBlockHeight(height)
	)
	key := headerHeightKey(hash)
	if err := db.Put(key, encoded); err != nil {
		log.Crit("Failed to store hash to height mapping", "err", err)
	}
	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	key = headerKey(height, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *memBatch) WriteReceipts(hash common.Hash, height uint64, receipts Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(height, hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *memBatch) WriteCanonicalHash(hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// WriteHeadBlockHash stores the head block's hash.
func (db *memBatch) WriteHeadBlockHash(hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *memBatch) WriteHeadHeaderHash(hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// WriteCommit stores a commit into the database.
func (db *memBatch) WriteCommit(height uint64, commit *Commit) {
	data, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Crit("Failed to RLP encode commit", "err", err)
	}
	db.WriteCommitRLP(height, data)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *memBatch) WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	if err := db.Put(commitKey(height), rlp); err != nil {
		log.Crit("Failed to store commit", "err", err)
	}
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *memBatch) ReadBlock(logger log.Logger, hash common.Hash, height uint64) *Block {
	header := db.ReadHeader(hash, height)
	if header == nil {
		return nil
	}
	body := db.ReadBody(hash, height)
	if body == nil {
		return nil
	}
	return NewBlockWithHeader(logger, header).WithBody(body)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *memBatch) ReadHeader(hash common.Hash, height uint64) *Header {
	data := db.ReadHeaderRLP(hash, height)
	if len(data) == 0 {
		return nil
	}
	header := new(Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return header
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *memBatch) ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(height, hash))
	return data.([]byte)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *memBatch) ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data.([]byte)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *memBatch) ReadBody(hash common.Hash, height uint64) *Body {
	data := db.ReadBodyRLP(hash, height)
	if len(data) == 0 {
		return nil
	}
	body := new(Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		log.Error("Invalid block body RLP", "hash", hash, "err", err)
		return nil
	}

	return body
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *memBatch) ReadHeadBlockHash() common.Hash {
	data, _ := db.Get(headBlockKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *memBatch) ReadHeaderHeight(hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data.([]byte))
	return &height
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *memBatch) ReadHeadHeaderHash() common.Hash {
	data, _ := db.Get(headHeaderKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *memBatch) ReadCommitRLP(height uint64) rlp.RawValue {
	data, _ := db.Get(commitKey(height))
	return data.([]byte)
}

// ReadBody retrieves the commit at a given height.
func (db *memBatch) ReadCommit(height uint64) *Commit {
	data := db.ReadCommitRLP(height)
	if len(data) == 0 {
		return nil
	}
	commit := new(Commit)
	if err := rlp.Decode(bytes.NewReader(data), commit); err != nil {
		log.Error("Invalid commit RLP", "err", err)
		return nil
	}
	commit.MakeEmptyNil()
	return commit
}

// DeleteBody removes all block body data associated with a hash.
func (db *memBatch) DeleteBody(hash common.Hash, height uint64) {
	if err := db.Delete(blockBodyKey(height, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// DeleteHeader removes all block header data associated with a hash.
func (db *memBatch) DeleteHeader(hash common.Hash, height uint64) {
	if err := db.Delete(headerKey(height, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerHeightKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *memBatch) DeleteCanonicalHash(number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *memBatch) ReadReceipts(hash common.Hash, number uint64) Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	// Convert the revceipts from their storage form to their internal representation
	storageReceipts := []*ReceiptForStorage{}
	if err := rlp.DecodeBytes(data.([]byte), &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*Receipt)(receipt)
	}
	return receipts
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *memBatch) ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *memBatch) WriteTxLookupEntries(block *Block) {
	for i, tx := range block.Transactions() {
		entry := TxLookupEntry{
			BlockHash:  block.Hash(),
			BlockIndex: block.Height(),
			Index:      uint64(i),
		}
		data, err := rlp.EncodeToBytes(entry)
		if err != nil {
			log.Crit("Failed to encode transaction lookup entry", "err", err)
		}
		if err := db.Put(txLookupKey(tx.Hash()), data); err != nil {
			log.Crit("Failed to store transaction lookup entry", "err", err)
		}
	}
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func (db *memBatch) DeleteTxLookupEntry(hash common.Hash) {
	db.Delete(txLookupKey(hash))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *memBatch) ReadTransaction(hash common.Hash) (*Transaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := db.ReadTxLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := db.ReadBody(blockHash, blockNumber)
	if body == nil || len(body.Transactions) <= int(txIndex) {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash, "index", txIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.Transactions[txIndex], blockHash, blockNumber, txIndex
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *memBatch) ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(dualEventLookupKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry DualEventLookupEntry
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid dual's event lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *memBatch) ReadDualEvent(hash common.Hash) (*DualEvent, common.Hash, uint64, uint64) {
	blockHash, blockNumber, eventIndex := db.ReadDualEventLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := db.ReadBody(blockHash, blockNumber)
	if body == nil || len(body.DualEvents) <= int(eventIndex) {
		log.Error("Dual event referenced missing", "number", blockNumber, "hash", blockHash, "index", eventIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.DualEvents[eventIndex], blockHash, blockNumber, eventIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func (db *memBatch) ReadReceipt(hash common.Hash) (*Receipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := db.ReadTxLookupEntry(hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	receipts := db.ReadReceipts(blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", "number", blockNumber, "hash", blockHash, "index", receiptIndex)
		return nil, common.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func (db *memBatch) ReadBloomBits(bit uint, section uint64, head common.Hash) ([]byte, error) {
	data, err := db.Get(bloomBitsKey(bit, section, head))
	if err != nil || data == nil || len(data.([]byte)) == 0 {
		return nil, err
	}
	return data.([]byte), err
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func (db *memBatch) WriteBloomBits(bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *memBatch) ReadHeaderNumber(hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data.([]byte))
	return &number
}

// Stores a hash into the database.
func (db *memBatch) StoreHash(hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a hash already exists in the database.
func (db *memBatch) CheckHash(hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}

// Stores a tx hash into the database.
func (db *memBatch) StoreTxHash(hash *common.Hash) {
	if err := db.Put(txHashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a tx hash already exists in the database.
func (db *memBatch) CheckTxHash(hash *common.Hash) bool {
	data, _ := db.Get(txHashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}


func (db *memBatch) ReadEvent(address string, method string) *WatcherAction {
	data, err := db.Get(eventKey(address, method))
	if err != nil {
		log.Error("error while get event", "err", err, "address", address, "method", method)
		return nil
	}
	var entry WatcherAction
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}
	return &entry
}

func (db *memBatch) ReadEvents(address string) []*WatcherAction {
	data, err := db.Get(eventsKey(address))
	if err != nil {
		log.Error("error while get event", "err", err, "address", address)
		return nil
	}
	var entries []string
	if err := rlp.DecodeBytes(data.([]byte), &entries); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}

	watcherActions := make([]*WatcherAction, 0)
	if len(entries) > 0 {
		for _, entry := range entries {
			// get watched event from entry
			evtData, err := db.Get(common.Hex2Bytes(entry))
			if err != nil {
				log.Error("Cannot get event data", "err", err, "eventData", entry)
				continue
			}
			var action WatcherAction
			if err := rlp.DecodeBytes(evtData.([]byte), &action); err != nil {
				log.Error("Invalid watcherAction", "err", err)
				continue
			}
			watcherActions = append(watcherActions, &action)
		}
	}
	return watcherActions
}

func (db *memBatch) ReadSmartContractFromDualAction(action string) (string, *abi.ABI) {
	key, err := db.Get(dualActionKey(action))
	if err != nil || key == nil {
		return "", nil
	}

	data, err := db.Get(key)
	if err != nil || data == nil {
		return "", nil
	}

	var entry smartContract
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return "", nil
	}

	a, err := abi.JSON(strings.NewReader(entry.ABI))
	if err != nil {
		return "", nil
	}
	return entry.Address, &a
}


func (db *memBatch) ReadSmartContractAbi(address string) *abi.ABI {
	data, err := db.Get(contractAbiKey(address))
	if err != nil || data == nil {
		log.Error("error while get abi from contract address", "err", err, "address", address)
		return nil
	}
	abiStr := string(data.([]byte))
	a, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		log.Error("cannot get abi from smart contract", "err", err, "address", address)
		return nil
	}
	return &a
}

func (db *memBatch) WriteEvent(smc *KardiaSmartcontract) {
	address := smc.SmcAddress

	// Write contract abi
	smartContract := smartContract{
		Address: smc.SmcAddress,
		ABI:     smc.SmcAbi,
	}
	encodedData, err := rlp.EncodeToBytes(smartContract)
	if err != nil {
		log.Error("failed to encode smartContract Data")
	}
	if err := db.Put(contractAbiKey(address), encodedData); err != nil {
		log.Crit("Failed to store dualAction", "err", err)
	}

	// Add dual actions
	// Note: dual action must be unique and be generated by kvm when apply KSML
	for _, action := range smc.DualActions {
		if err := db.Put(dualActionKey(action.Name), contractAbiKey(address)); err != nil {
			log.Crit("Failed to store dualAction", "err", err)
		}
	}

	// Add watcher action to db
	for _, event := range smc.WatcherActions {
		method := event.Method
		data, err := rlp.EncodeToBytes(event)
		if err != nil {
			log.Crit("Failed to encode event", "err", err, "method", method, "contract", smc.SmcAddress)
		}
		key := eventKey(address, method)
		if err := db.Put(key, data); err != nil {
			log.Crit("Failed to store last header's hash", "err", err)
		}
	}
}


// The fields below define the low level database schema prefixing.
var (
	// headHeaderKey tracks the latest know header's hash.
	headHeaderKey = []byte("LastHeader")

	// headBlockKey tracks the latest know full block's hash.
	headBlockKey = []byte("LastBlock")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerHeightPrefix = []byte("H") // headerHeightPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix     = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	commitPrefix = []byte("c") // commitPrefix + num (uint64 big endian) -> commit

	// TODO(namdoh@): The hashKey is primarily used for persistently store a tx hash in db, so we
	// quickly check if a tx has been seen in the past. When the scope of this key extends beyond
	// tx hash, it's probably cleaner to refactor this into a separate API (instead of grouping
	// it under chaindb).
	hashPrefix   = []byte("hash")   // hashPrefix + hash -> hash key
	txHashPrefix = []byte("txHash") // txHashPrefix + hash -> hash key

	configPrefix          = []byte("kardia-config-") // config prefix for the db
	txLookupPrefix        = []byte("l")              // txLookupPrefix + hash -> transaction/receipt lookup metadata
	dualEventLookupPrefix = []byte("de")             // dualEventLookupPrefix + hash -> dual's event lookup metadata
	bloomBitsPrefix       = []byte("B")              // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits

	eventPrefix           = []byte("event")              // event prefix + smartcontract address + method
	eventsPrefix          = []byte("events")             // event prefix + smart contract address
	dualActionPrefix      = []byte("dualAction")
	contractAbiPrefix     = []byte("abi")
)

// A positional metadata to help looking up the data content of
// a dual's event given only its hash.
type DualEventLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// TxLookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// encodeBlockHeight encodes a block height as big endian uint64
func encodeBlockHeight(height uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, height)
	return enc
}

// Encodes a boolean value as big endian uint16
func encodeBoolean(val bool) []byte {
	encoded := make([]byte, 2)
	if val {
		binary.BigEndian.PutUint16(encoded, 1)
	} else {
		binary.BigEndian.PutUint16(encoded, 0)
	}
	return encoded
}

// Decodes a big endian uint16 as boolean value
func decodeBoolean(data []byte) bool {
	if len(data) != 2 {
		return false
	}
	decoded := binary.BigEndian.Uint16(data)
	if decoded == 0 {
		return false
	}
	return true
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(height uint64) []byte {
	return append(append(headerPrefix, encodeBlockHeight(height)...), headerHashSuffix...)
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(height uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
}

// headerheightKey = headerheightPrefix + hash
func headerHeightKey(hash common.Hash) []byte {
	return append(headerHeightPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(height uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(height uint64, hash common.Hash) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
}

// commitKey = commitPrefix + ":" + height
func commitKey(height uint64) []byte {
	return append(commitPrefix, encodeBlockHeight(height)...)
}

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}

// txLookupKey = txLookupPrefix + hash
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

// dualEventLookupKey = dualEventLookupPrefix + hash
func dualEventLookupKey(hash common.Hash) []byte {
	return append(dualEventLookupPrefix, hash.Bytes()...)
}

// bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash
func bloomBitsKey(bit uint, section uint64, hash common.Hash) []byte {
	key := append(append(bloomBitsPrefix, make([]byte, 10)...), hash.Bytes()...)

	binary.BigEndian.PutUint16(key[1:], uint16(bit))
	binary.BigEndian.PutUint64(key[3:], section)

	return key
}

// hashKey = hashPrefix + hash
func hashKey(hash *common.Hash) []byte {
	return append(hashPrefix, hash.Bytes()...)
}

// txHashKey = txHashPrefix + hash
func txHashKey(hash *common.Hash) []byte {
	return append(txHashPrefix, hash.Bytes()...)
}

func eventKey(smartContractAddress string, method string) []byte {
	return append(append(eventPrefix, []byte(smartContractAddress)...), []byte(method)...)
}

func dualActionKey(action string) []byte {
	return append(dualActionPrefix, []byte(action)...)
}

func contractAbiKey(smartContractAddress string) []byte {
	return append(contractAbiPrefix, []byte(smartContractAddress)...)
}

func eventsKey(smartContractAddress string) []byte {
	return append(eventsPrefix, []byte(smartContractAddress)...)
}

