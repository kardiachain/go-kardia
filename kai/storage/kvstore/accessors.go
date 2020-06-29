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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/syndtr/goleveldb/leveldb/errors"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/types"
)

type SmartContract struct {
	Address string
	ABI     string
}

type KardiaEvents struct {
	Events    []string
	MasterSmc string
}

// CommonReadCanonicalHash retrieves the hash assigned to a canonical block height.
func CommonReadCanonicalHash(db kaidb.Reader, height uint64) common.Hash {
	data, _ := db.Get(headerHashKey(height))
	if data == nil || len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// CommonReadChainConfig retrieves the consensus settings based on the given genesis hash.
func CommonReadChainConfig(db kaidb.Reader, hash common.Hash) *types.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config types.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		//panic(fmt.Errorf("Invalid chain config JSON hash:%s, err: %s", hash.Hex(), err))
	}
	return &config
}

// CommonWriteChainConfig writes the chain config settings to the database.
func CommonWriteChainConfig(db kaidb.Writer, hash common.Hash, cfg *types.ChainConfig) {
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

// WriteReceipts stores all the transaction receipts belonging to a block.
func CommonWriteReceipts(db kaidb.Writer, hash common.Hash, height uint64, receipts types.Receipts) {
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
func CommonWriteCanonicalHash(db kaidb.Writer, hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// CommonWriteHeadBlockHash stores the head block's hash.
func CommonWriteHeadBlockHash(db kaidb.Writer, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// CommonWriteHeadHeaderHash stores the hash of the current canonical head header.
func CommonWriteHeadHeaderHash(db kaidb.Writer, hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// CommonWriteEvent stores all events from watched smart contract to db.
func CommonWriteEvent(db kaidb.Writer, smc *types.KardiaSmartcontract) {
	if smc.SmcAbi != "" {
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
	}

	// Write master contract abi
	masterSmc := SmartContract{
		Address: smc.MasterSmc,
		ABI:     smc.MasterAbi,
	}
	encodedData, err := rlp.EncodeToBytes(masterSmc)
	if err != nil {
		log.Error("failed to encode smartContract Data")
	}
	abiKey := contractAbiKey(masterSmc.Address)
	if err := db.Put(abiKey, encodedData); err != nil {
		log.Error("Failed to store dualAction", "err", err)
	}

	events := make([]string, 0)

	// Add watcher action to db
	for _, event := range smc.Watchers {
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

	kaiEvent := KardiaEvents{
		Events:    events,
		MasterSmc: masterSmc.Address,
	}

	// Add list events to db
	if len(kaiEvent.Events) > 0 {
		encodedEvents, err := rlp.EncodeToBytes(kaiEvent)
		if err != nil {
			log.Error("Failed to encode events list", "err", err, "contract", smc.SmcAddress)
		}
		if err := db.Put(eventsKey(smc.SmcAddress), encodedEvents); err != nil {
			log.Error("Failed to store last header's hash", "err", err)
		}
	}

}

// CommonWriteCommit stores a commit into the database.
func CommonWriteCommit(db kaidb.Writer, height uint64, commit *types.Commit) {
	data, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Crit("Failed to RLP encode commit", "err", err)
	}
	if err := db.Put(commitKey(height), data); err != nil {
		log.Crit("Failed to store commit", "err", err)
	}
}

// CommonReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func CommonReadBodyRLP(db kaidb.Reader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data
}

// CommonReadBody retrieves the block body corresponding to the hash.
func CommonReadBody(db kaidb.Reader, hash common.Hash, height uint64) *types.Body {
	return ReadBlock(db, hash, height).Body()
}

// CommonReadHeadBlockHash retrieves the hash of the current canonical head block.
func CommonReadHeadBlockHash(db kaidb.Reader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if data == nil || len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// CommonReadHeaderheight returns the header height assigned to a hash.
func CommonReadHeaderHeight(db kaidb.Reader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data) == 0 || len(data) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data)
	return &height
}

// CommonReadHeadHeaderHash retrieves the hash of the current canonical head header.
func CommonReadHeadHeaderHash(db kaidb.Reader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if data == nil || len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// CommonReadCommitRLP retrieves the commit in RLP encoding.
func CommonReadCommitRLP(db kaidb.Reader, height uint64) rlp.RawValue {
	data, _ := db.Get(commitKey(height))
	return data
}

// CommonReadBody retrieves the commit at a given height.
func CommonReadCommit(db kaidb.Reader, height uint64) *types.Commit {
	data := CommonReadCommitRLP(db, height)
	if len(data) == 0 {
		return nil
	}
	commit := new(types.Commit)
	if err := rlp.Decode(bytes.NewReader(data), commit); err != nil {
		panic(fmt.Errorf("Decode read commit error: %s height: %d", err, height))
	}
	return commit
}

// CommonDeleteBody removes all block body data associated with a hash.
func CommonDeleteBody(db kaidb.KeyValueWriter, hash common.Hash, height uint64) {
	if err := db.Delete(blockBodyKey(height, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// CommonDeleteHeader removes all block header data associated with a hash.
func CommonDeleteHeader(db kaidb.KeyValueWriter, hash common.Hash, height uint64) {
	if err := db.Delete(headerKey(height, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerHeightKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// CommonDeleteCanonicalHash removes the number to hash canonical mapping.
func CommonDeleteCanonicalHash(db kaidb.KeyValueWriter, number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// CommonReadReceipts retrieves all the transaction receipts belonging to a block.
func CommonReadReceipts(db kaidb.Reader, hash common.Hash, number uint64) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if data == nil || len(data) == 0 {
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

// CommonReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func CommonReadTxLookupEntry(db kaidb.Reader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if data == nil || len(data) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// CommonWriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func CommonWriteTxLookupEntries(db kaidb.Writer, block *types.Block) {
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
func CommonDeleteTxLookupEntry(db kaidb.KeyValueWriter, hash common.Hash) {
	db.Delete(txLookupKey(hash))
}

// CommonReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func CommonReadTransaction(db kaidb.Reader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := CommonReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}

	body := CommonReadBody(db, blockHash, blockNumber)
	if body == nil || len(body.Transactions) <= int(txIndex) {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash, "index", txIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.Transactions[txIndex], blockHash, blockNumber, txIndex
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func CommonReadDualEventLookupEntry(db kaidb.Reader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(dualEventLookupKey(hash))
	if data == nil || len(data) == 0 {
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
func CommonReadDualEvent(db kaidb.Reader, hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	blockHash, blockNumber, eventIndex := CommonReadDualEventLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := CommonReadBody(db, blockHash, blockNumber)
	if body == nil || len(body.DualEvents) <= int(eventIndex) {
		log.Error("Dual event referenced missing", "number", blockNumber, "hash", blockHash, "index", eventIndex)
		return nil, common.Hash{}, 0, 0
	}
	return body.DualEvents[eventIndex], blockHash, blockNumber, eventIndex
}

// CommonReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func CommonReadReceipt(db kaidb.Reader, hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := CommonReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	receipts := CommonReadReceipts(db, blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", "number", blockNumber, "hash", blockHash, "index", receiptIndex)
		return nil, common.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// CommonReadEvent gets a watcher action from contract address and method
func CommonReadEvent(db kaidb.Reader, address string, method string) *types.Watcher {
	data, err := db.Get(eventKey(address, method))
	if err != nil {
		log.Trace("event not found", "err", err, "address", address, "method", method)
		return nil
	}
	var entry types.Watcher
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return nil
	}
	return &entry
}

// CommonReadEvents gets events data from contract address
func CommonReadEvents(db kaidb.Reader, address string) (string, []*types.Watcher) {
	data, err := db.Get(eventsKey(address))
	if err != nil {
		log.Trace("event not found", "err", err, "address", address)
		return "", nil
	}
	var events KardiaEvents
	if err := rlp.DecodeBytes(data, &events); err != nil {
		log.Error("Invalid event lookup rlp", "err", err)
		return "", nil
	}

	watcherActions := make([]*types.Watcher, 0)
	if len(events.Events) > 0 {
		for _, evt := range events.Events {
			// get watched event from entry
			var evtData interface{}
			if evtData, err = db.Get(common.Hex2Bytes(evt)); err != nil {
				log.Error("Cannot get event data", "err", err, "eventData", evt)
				continue
			}
			var action types.Watcher
			if err := rlp.DecodeBytes(evtData.([]byte), &action); err != nil {
				log.Error("Invalid watcherAction", "err", err)
				continue
			}
			watcherActions = append(watcherActions, &action)
		}
	}
	return events.MasterSmc, watcherActions
}

// CommonReadSmartContractAbi gets watched smart contract abi
func CommonReadSmartContractAbi(db kaidb.Reader, address string) *abi.ABI {
	data, err := db.Get(contractAbiKey(address))
	if err != nil || data == nil {
		log.Warn("error while get abi from contract address", "err", err, "address", address)
		return nil
	}
	var entry SmartContract
	if err := rlp.DecodeBytes(data, &entry); err != nil {
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
func CommonReadBloomBits(db kaidb.Reader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	data, err := db.Get(bloomBitsKey(bit, section, head))
	if err != nil || data == nil || len(data) == 0 {
		return nil, err
	}
	return data, err
}

// CommonWriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func CommonWriteBloomBits(db kaidb.Writer, bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

// CommonReadHeaderNumber returns the header number assigned to a hash.
func CommonReadHeaderNumber(db kaidb.Reader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data) == 0 || len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// CommonStores a hash into the database.
func CommonStoreHash(db kaidb.Writer, hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a hash already exists in the database.
func CommonCheckHash(db kaidb.Reader, hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data)
}

// Stores a tx hash into the database.
func CommonStoreTxHash(db kaidb.Writer, hash *common.Hash) {
	if err := db.Put(txHashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// Returns true if a tx hash already exists in the database.
func CommonCheckTxHash(db kaidb.Reader, hash *common.Hash) bool {
	data, _ := db.Get(txHashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data)
}

// ReadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func ReadBlockMeta(db kaidb.Reader, hash common.Hash, height uint64) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)
	metaBytes, _ := db.Get(blockMetaKey(hash, height))

	if len(metaBytes) == 0 {
		return nil
	}

	if err := rlp.DecodeBytes(metaBytes, blockMeta); err != nil {
		panic(errors.New("Reading block meta error"))
	}
	return blockMeta
}

func ReadSeenCommit(db kaidb.Reader, height uint64) *types.Commit {
	var commit = new(types.Commit)
	commitBytes, _ := db.Get(seenCommitKey(height))

	if len(commitBytes) == 0 {
		return nil
	}

	if err := rlp.DecodeBytes(commitBytes, commit); err != nil {
		panic(errors.New("Reading seen commit error"))
	}

	return commit
}

// ReadBlock returns the Block for the given height
func ReadBlock(db kaidb.Reader, hash common.Hash, height uint64) *types.Block {
	blockMeta := ReadBlockMeta(db, hash, height)

	if blockMeta == nil {
		return nil
	}

	buf := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total.Int32(); i++ {
		part := ReadBlockPart(db, hash, height, i)
		buf = append(buf, part.Bytes...)
	}

	block := new(types.Block)
	if err := rlp.DecodeBytes(buf, block); err != nil {
		panic(errors.New("Reading block error"))
	}
	return block
}

// CommonReadHeader retrieves the block header corresponding to the hash.
func CommonReadHeader(db kaidb.Reader, hash common.Hash, height uint64) *types.Header {
	blockMeta := ReadBlockMeta(db, hash, height)
	return blockMeta.Header
}

// CommonReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func CommonReadHeaderRLP(db kaidb.Reader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(height, hash))
	return data
}

// ReadBlockPart returns the block part fo the given height and index
func ReadBlockPart(db kaidb.Reader, hash common.Hash, height uint64, index int) *types.Part {
	part := new(types.Part)
	partBytes, _ := db.Get(blockPartKey(height, index))

	if len(partBytes) == 0 {
		return nil
	}

	if err := rlp.DecodeBytes(partBytes, part); err != nil {
		panic(fmt.Errorf("Decode block part error: %s", err))
	}
	return part
}

// WriteBlock write block to database
func WriteBlock(db kaidb.Database, block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	height := block.Height()
	hash := block.Hash()

	batch := db.NewBatch()

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)

	metaBytes, err := rlp.EncodeToBytes(blockMeta)

	if err != nil {
		panic(fmt.Errorf("encode block meta error: %s", err))
	}

	batch.Put(blockMetaKey(hash, height), metaBytes)

	// Save block part
	for i := 0; i < blockParts.Total(); i++ {
		part := blockParts.GetPart(i)
		writeBlockPart(batch, height, i, part)

	}

	// Save block commit (duplicate and separate from the Block)
	lastCommit := block.LastCommit()
	lastCommitBytes, err := rlp.EncodeToBytes(lastCommit)
	if err != nil {
		panic(fmt.Errorf("encode last commit error: %s", err))
	}
	batch.Put(commitKey(height-1), lastCommitBytes)

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	seenCommitBytes, err := rlp.EncodeToBytes(seenCommit)
	if err != nil {
		panic(fmt.Errorf("encode seen commit error: %s", err))
	}

	if err := batch.Put(seenCommitKey(height), seenCommitBytes); err != nil {
		panic(fmt.Errorf("Failed to store seen commit err: %s", err))
	}

	key := headerHeightKey(hash)
	if err := batch.Put(key, encodeBlockHeight(height)); err != nil {
		panic(fmt.Errorf("Failed to store hash to height mapping err: %s", err))
	}

	if err := batch.Write(); err != nil {
		panic(fmt.Errorf("Failed to store block error: %s", err))
	}

}

func writeBlockPart(db kaidb.Writer, height uint64, index int, part *types.Part) {
	partBytes, err := rlp.EncodeToBytes(part)
	if err != nil {
		panic(fmt.Errorf("encode block part error: %d, height :%d, index: %d", err, height, index))
	}
	db.Put(blockPartKey(height, index), partBytes)
}

func DeleteBlockMeta(db kaidb.Writer, hash common.Hash, height uint64) {
	db.Delete(blockMetaKey(hash, height))
}

func ReadAppHash(db kaidb.Reader, height uint64) common.Hash {
	b, _ := db.Get(appHashKey(height))
	if len(b) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(b)
}

func WriteAppHash(db kaidb.Writer, height uint64, hash common.Hash) {
	db.Put(appHashKey(height), hash.Bytes())
}
