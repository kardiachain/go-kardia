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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/kardiachain/go-kardia/lib/abi"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
)

type SmartContract struct {
	Address string
	ABI     string
}

// CommonReadCanonicalHash retrieves the hash assigned to a canonical block height.
func CommonReadCanonicalHash(db types.DatabaseReader, height uint64) common.Hash {
	data, _ := db.Get(headerHashKey(height))
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// CommonReadChainConfig retrieves the consensus settings based on the given genesis hash.
func CommonReadChainConfig(db types.DatabaseReader, hash common.Hash) *types.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	var config types.ChainConfig
	if err := json.Unmarshal(data.([]byte), &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// CommonWriteChainConfig writes the chain config settings to the database.
func CommonWriteChainConfig(db types.DatabaseWriter, hash common.Hash, cfg *types.ChainConfig) {
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

// CommonWriteBlock serializes a block into the database, header and body separately.
func CommonWriteBlock(db types.DatabaseWriter, block *types.Block) {
	db.WriteBody(block.Hash(), block.Height(),block.Body())
	db.WriteHeader(block.Header())
}

// WriteBody stores a block body into the database.
func CommonWriteBody(db types.DatabaseWriter, hash common.Hash, height uint64, body *types.Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	db.WriteBodyRLP(hash, height, data)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func CommonWriteBodyRLP(db types.DatabaseWriter, hash common.Hash, height uint64, rlp rlp.RawValue) {
	if err := db.Put(blockBodyKey(height, hash), rlp); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func CommonWriteHeader(db types.DatabaseWriter, header *types.Header) {
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
func CommonWriteReceipts(db types.DatabaseWriter, hash common.Hash, height uint64, receipts types.Receipts) {
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

// CommonWriteCanonicalHash stores the hash assigned to a canonical block height.
func CommonWriteCanonicalHash(db types.DatabaseWriter, hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// CommonWriteHeadBlockHash stores the head block's hash.
func CommonWriteHeadBlockHash(db types.DatabaseWriter, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// CommonWriteHeadHeaderHash stores the hash of the current canonical head header.
func CommonWriteHeadHeaderHash(db types.DatabaseWriter, hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// CommonWriteEvent stores all events from watched smart contract to db.
func CommonWriteEvent(db types.DatabaseWriter, smc *types.KardiaSmartcontract) {
	// Write contract abi
	smartContract := SmartContract{
		Address: smc.SmcAddress,
		ABI:     smc.SmcAbi,
	}
	encodedData, err := rlp.EncodeToBytes(smartContract)
	if err != nil {
		log.Error("failed to encode smartContract Data")
	}
	abiKey := contractAbiKey(smc.SmcAddress)
	if err := db.Put(abiKey, encodedData); err != nil {
		log.Error("Failed to store dualAction", "err", err)
	}

	// Add dual actions
	// Note: dual action must be unique and be generated by kvm when apply KSML
	for _, action := range smc.DualActions {
		if err := db.Put(dualActionKey(action.Name), abiKey); err != nil {
			log.Error("Failed to store dualAction", "err", err)
		}
	}

	events := make([]string, 0)

	// Add watcher action to db
	for _, event := range smc.WatcherActions {
		method := event.Method
		data, err := rlp.EncodeToBytes(event)
		if err != nil {
			log.Error("Failed to encode event", "err", err, "method", method, "contract", smc.SmcAddress)
		}
		key := eventKey(smc.SmcAddress, method)
		if err := db.Put(key, data); err != nil {
			log.Error("Failed to store last header's hash", "err", err)
		}
		events = append(events, common.Bytes2Hex(key))
	}

	// Add list events to db
	if len(events) > 0 {
		encodedEvents, err := rlp.EncodeToBytes(events)
		if err != nil {
			log.Error("Failed to encode events list", "err", err, "contract", smc.SmcAddress)
		}
		if err := db.Put(eventsKey(smc.SmcAddress), encodedEvents); err != nil {
			log.Error("Failed to store last header's hash", "err", err)
		}
	}

}

// CommonWriteCommit stores a commit into the database.
func CommonWriteCommit(db types.DatabaseWriter, height uint64, commit *types.Commit) {
	data, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Crit("Failed to RLP encode commit", "err", err)
	}
	db.WriteCommitRLP(height, data)
}

// CommonWriteCommitRLP stores an RLP encoded commit into the database.
func CommonWriteCommitRLP(db types.DatabaseWriter, height uint64, rlp rlp.RawValue) {
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
func CommonReadBlock(logger log.Logger, db types.DatabaseReader, hash common.Hash, height uint64) *types.Block {
	header := db.ReadHeader(hash, height)
	if header == nil {
		return nil
	}
	body := db.ReadBody(hash, height)
	if body == nil {
		return nil
	}
	return types.NewBlockWithHeader(logger, header).WithBody(body)
}

// CommonReadHeader retrieves the block header corresponding to the hash.
func CommonReadHeader(db types.DatabaseReader, hash common.Hash, height uint64) *types.Header {
	data := db.ReadHeaderRLP(hash, height)
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

// CommonReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func CommonReadHeaderRLP(db types.DatabaseReader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(height, hash))
	return data.([]byte)
}

// CommonReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func CommonReadBodyRLP(db types.DatabaseReader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data.([]byte)
}

// CommonReadBody retrieves the block body corresponding to the hash.
func CommonReadBody(db types.DatabaseReader, hash common.Hash, height uint64) *types.Body {
	data := db.ReadBodyRLP(hash, height)
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

// CommonReadHeadBlockHash retrieves the hash of the current canonical head block.
func CommonReadHeadBlockHash(db types.DatabaseReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// CommonReadHeaderheight returns the header height assigned to a hash.
func CommonReadHeaderHeight(db types.DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data.([]byte))
	return &height
}

// CommonReadHeadHeaderHash retrieves the hash of the current canonical head header.
func CommonReadHeadHeaderHash(db types.DatabaseReader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if data == nil || len(data.([]byte)) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data.([]byte))
}

// CommonReadCommitRLP retrieves the commit in RLP encoding.
func CommonReadCommitRLP(db types.DatabaseReader, height uint64) rlp.RawValue {
	data, _ := db.Get(commitKey(height))
	return data.([]byte)
}

// CommonReadBody retrieves the commit at a given height.
func CommonReadCommit(db types.DatabaseReader, height uint64) *types.Commit {
	data := db.ReadCommitRLP(height)
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

// CommonDeleteBody removes all block body data associated with a hash.
func CommonDeleteBody(db types.DatabaseDeleter, hash common.Hash, height uint64) {
	if err := db.Delete(blockBodyKey(height, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// CommonDeleteHeader removes all block header data associated with a hash.
func CommonDeleteHeader(db types.DatabaseDeleter, hash common.Hash, height uint64) {
	if err := db.Delete(headerKey(height, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerHeightKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// CommonDeleteCanonicalHash removes the number to hash canonical mapping.
func CommonDeleteCanonicalHash(db types.DatabaseDeleter, number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// CommonReadReceipts retrieves all the transaction receipts belonging to a block.
func CommonReadReceipts(db types.DatabaseReader, hash common.Hash, number uint64) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if data == nil || len(data.([]byte)) == 0 {
		return nil
	}
	// Convert the revceipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data.([]byte), &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

// CommonReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func CommonReadTxLookupEntry(db types.DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
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

// CommonWriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func CommonWriteTxLookupEntries(db types.DatabaseWriter, block *types.Block) {
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

// CommonDeleteTxLookupEntry removes all transaction data associated with a hash.
func CommonDeleteTxLookupEntry(db types.DatabaseDeleter, hash common.Hash) {
	db.Delete(txLookupKey(hash))
}

// CommonReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func CommonReadTransaction(db types.DatabaseReader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
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
func CommonReadDualEventLookupEntry(db types.DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
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
func CommonReadDualEvent(db types.DatabaseReader, hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
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

// CommonReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func CommonReadReceipt(db types.DatabaseReader, hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
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

func CommonReadEventFromDualAction(db types.DatabaseReader, action string) (string, *abi.ABI) {
	key, err := db.Get(dualActionKey(action))
	if err != nil || key == nil {
		return "", nil
	}

	data, err := db.Get(key.([]byte))
	if err != nil || data == nil {
		return "", nil
	}

	var entry SmartContract
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return "", nil
	}

	// replace ' to "
	if entry.ABI != "" {
		abiStr := strings.Replace(entry.ABI, "'", "\"", -1)
		a, err := abi.JSON(strings.NewReader(abiStr))
		if err != nil {
			log.Error("error while decoding abi", "err", err, "abi", entry.ABI)
			return entry.Address, nil
		}
		return entry.Address, &a
	}
	return entry.Address, nil
}

// CommonReadEvent gets event data from contract address and method
func CommonReadEvent(db types.DatabaseReader, address string, method string) *types.WatcherAction {
	data, err := db.Get(eventKey(address, method))
	if err != nil {
		log.Error("error while get event", "err", err, "address", address, "method", method)
		return nil
	}
	var entry types.WatcherAction
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}
	return &entry
}

// CommonReadEvents gets events data from contract address
func CommonReadEvents(db types.DatabaseReader, address string) []*types.WatcherAction {
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

	watcherActions := make([]*types.WatcherAction, 0)
	if len(entries) > 0 {
		for _, entry := range entries {
			// get watched event from entry
			evtData, err := db.Get(common.Hex2Bytes(entry))
			if err != nil {
				log.Error("Cannot get event data", "err", err, "eventData", entry)
				continue
			}
			var action types.WatcherAction
			if err := rlp.DecodeBytes(evtData.([]byte), &action); err != nil {
				log.Error("Invalid watcherAction", "err", err)
				continue
			}
			watcherActions = append(watcherActions, &action)
		}
	}
	return watcherActions
}

// CommonReadSmartContractAbi gets watched smart contract abi
func CommonReadSmartContractAbi(db types.DatabaseReader, address string) *abi.ABI {
	data, err := db.Get(contractAbiKey(address))
	if err != nil || data == nil {
		log.Error("error while get abi from contract address", "err", err, "address", address)
		return nil
	}
	var entry SmartContract
	if err := rlp.DecodeBytes(data.([]byte), &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}
	// replace ' to "
	if entry.ABI != "" {
		abiStr := strings.Replace(entry.ABI, "'", "\"", -1)
		a, err := abi.JSON(strings.NewReader(abiStr))
		if err != nil {
			log.Error("error while decoding abi", "err", err, "abi", entry.ABI)
			return nil
		}
		return &a
	}
	return nil
}

// CommonReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func CommonReadBloomBits(db types.DatabaseReader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	data, err := db.Get(bloomBitsKey(bit, section, head))
	if err != nil || data == nil || len(data.([]byte)) == 0 {
		return nil, err
	}
	return data.([]byte), err
}

// CommonWriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func CommonWriteBloomBits(db types.DatabaseWriter, bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

// CommonReadHeaderNumber returns the header number assigned to a hash.
func CommonReadHeaderNumber(db types.DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data.([]byte)) == 0 || len(data.([]byte)) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data.([]byte))
	return &number
}

// CommonStores a hash into the database.
func CommonStoreHash(db types.DatabaseWriter, hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a hash already exists in the database.
func CommonCheckHash(db types.DatabaseReader, hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}

// Stores a tx hash into the database.
func CommonStoreTxHash(db types.DatabaseWriter, hash *common.Hash) {
	if err := db.Put(txHashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a tx hash already exists in the database.
func CommonCheckTxHash(db types.DatabaseReader, hash *common.Hash) bool {
	data, _ := db.Get(txHashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data.([]byte))
}
