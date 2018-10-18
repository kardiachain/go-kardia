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

package chaindb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
)

// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Put(key []byte, value []byte) error
}

// DatabaseDeleter wraps the Delete method of a backing data store.
type DatabaseDeleter interface {
	Delete(key []byte) error
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func ReadCanonicalHash(db DatabaseReader, height uint64) common.Hash {
	data, _ := db.Get(headerHashKey(height))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db DatabaseReader, hash common.Hash) *configs.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config configs.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db DatabaseWriter, hash common.Hash, cfg *configs.ChainConfig) {
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
func WriteBlock(db DatabaseWriter, block *types.Block) {
	WriteBody(db, block.Hash(), block.Height(), block.Body())
	WriteHeader(db, block.Header())
}

// WriteBody stores a block body into the database.
func WriteBody(db DatabaseWriter, hash common.Hash, height uint64, body *types.Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	WriteBodyRLP(db, hash, height, data)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func WriteBodyRLP(db DatabaseWriter, hash common.Hash, height uint64, rlp rlp.RawValue) {
	if err := db.Put(blockBodyKey(height, hash), rlp); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func WriteHeader(db DatabaseWriter, header *types.Header) {
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
func WriteReceipts(db DatabaseWriter, hash common.Hash, height uint64, receipts types.Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
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
func WriteCanonicalHash(db DatabaseWriter, hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func WriteHeadHeaderHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// WriteCommit stores a commit into the database.
func WriteCommit(db DatabaseWriter, height uint64, commit *types.Commit) {
	data, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Crit("Failed to RLP encode commit", "err", err)
	}
	WriteCommitRLP(db, height, data)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func WriteCommitRLP(db DatabaseWriter, height uint64, rlp rlp.RawValue) {
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
func ReadBlock(logger log.Logger, db DatabaseReader, hash common.Hash, height uint64) *types.Block {
	header := ReadHeader(db, hash, height)
	if header == nil {
		return nil
	}
	body := ReadBody(db, hash, height)
	if body == nil {
		return nil
	}
	return types.NewBlockWithHeader(logger, header).WithBody(body)
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadHeader(db DatabaseReader, hash common.Hash, height uint64) *types.Header {
	data := ReadHeaderRLP(db, hash, height)
	if len(data) == 0 {
		return nil
	}
	header := new(types.Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return header
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func ReadHeaderRLP(db DatabaseReader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(height, hash))
	return data
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func ReadBodyRLP(db DatabaseReader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data
}

// ReadBody retrieves the block body corresponding to the hash.
func ReadBody(db DatabaseReader, hash common.Hash, height uint64) *types.Body {
	data := ReadBodyRLP(db, hash, height)
	if len(data) == 0 {
		return nil
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		log.Error("Invalid block body RLP", "hash", hash, "err", err)
		return nil
	}

	return body
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// ReadHeaderheight returns the header height assigned to a hash.
func ReadHeaderHeight(db DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if len(data) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data)
	return &height
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadHeadHeaderHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func ReadCommitRLP(db DatabaseReader, height uint64) rlp.RawValue {
	data, _ := db.Get(commitKey(height))
	return data
}

// ReadBody retrieves the commit at a given height.
func ReadCommit(db DatabaseReader, height uint64) *types.Commit {
	data := ReadCommitRLP(db, height)
	if len(data) == 0 {
		return nil
	}
	commit := new(types.Commit)
	if err := rlp.Decode(bytes.NewReader(data), commit); err != nil {
		log.Error("Invalid commit RLP", "err", err)
		return nil
	}
	commit.MakeEmptyNil()
	return commit
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBody(db DatabaseDeleter, hash common.Hash, height uint64) {
	if err := db.Delete(blockBodyKey(height, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteHeader(db DatabaseDeleter, hash common.Hash, height uint64) {
	if err := db.Delete(headerKey(height, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerHeightKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db DatabaseDeleter, number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func ReadReceipts(db DatabaseReader, hash common.Hash, number uint64) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	// Convert the revceipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadTxLookupEntry(db DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if len(data) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteTxLookupEntries(db DatabaseWriter, block *types.Block) {
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
func DeleteTxLookupEntry(db DatabaseDeleter, hash common.Hash) {
	db.Delete(txLookupKey(hash))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db DatabaseReader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, blockNumber)
	if body == nil || len(body.Transactions) <= int(txIndex) {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash, "index", txIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.Transactions[txIndex], blockHash, blockNumber, txIndex
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func ReadDualEventLookupEntry(db DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(dualEventLookupKey(hash))
	if len(data) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry DualEventLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid dual's event lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func ReadDualEvent(db DatabaseReader, hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	blockHash, blockNumber, eventIndex := ReadDualEventLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, blockNumber)
	if body == nil || len(body.DualEvents) <= int(eventIndex) {
		log.Error("Dual event referenced missing", "number", blockNumber, "hash", blockHash, "index", eventIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.DualEvents[eventIndex], blockHash, blockNumber, eventIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func ReadReceipt(db DatabaseReader, hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	receipts := ReadReceipts(db, blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", "number", blockNumber, "hash", blockHash, "index", receiptIndex)
		return nil, common.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db DatabaseReader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	return db.Get(bloomBitsKey(bit, section, head))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db DatabaseWriter, bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

// ReadHeaderNumber returns the header number assigned to a hash.
func ReadHeaderNumber(db DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// Stores a hash into the database.
func StoreHash(db DatabaseWriter, hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a hash already exists in the database.
func CheckHash(db DatabaseReader, hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	return decodeBoolean(data)
}
