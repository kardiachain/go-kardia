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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/lib/abi"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
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
func CommonReadChainConfig(db kaidb.Reader, hash common.Hash) *configs.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if data == nil || len(data) == 0 {
		return nil
	}
	var config configs.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// CommonWriteChainConfig writes the chain config settings to the database.
func CommonWriteChainConfig(db kaidb.Writer, hash common.Hash, cfg *configs.ChainConfig) {
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

// CommonWriteBlockInfo stores block info  belonging to a block.
func CommonWriteBlockInfo(db kaidb.Writer, hash common.Hash, height uint64, blockInfo *types.BlockInfo) {
	bytes, err := rlp.EncodeToBytes(blockInfo)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	// Store the flattened receipt slice
	if err := db.Put(blockInfoKey(height, hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// CommonWriteCanonicalHash stores the hash assigned to a canonical block height.
func CommonWriteCanonicalHash(db kaidb.Writer, hash common.Hash, height uint64) {
	if err := db.Put(headerHashKey(height), hash.Bytes()); err != nil {
		log.Crit("Failed to store height to hash mapping", "err", err)
	}
}

// CommonWriteHeadBlockHash stores the hash of the current canonical head header.
func CommonWriteHeadBlockHash(db kaidb.Writer, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		panic(fmt.Sprintln("Failed to store last header's hash", "err", err))
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

// CommonReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func CommonReadBodyRLP(db kaidb.Reader, hash common.Hash, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, hash))
	return data
}

// CommonReadBody retrieves the block body corresponding to the hash.
func CommonReadBody(db kaidb.Reader, hash common.Hash, height uint64) *types.Body {
	return ReadBlock(db, height).Body()
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

// CommonReadBody retrieves the commit at a given height.
func CommonReadCommit(db kaidb.Reader, height uint64) *types.Commit {
	var pbc = new(kproto.Commit)
	bz, _ := db.Get(commitKey(height))
	if len(bz) == 0 {
		return nil
	}

	err := proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Errorf("error reading block commit: %w", err))
	}
	commit, err := types.CommitFromProto(pbc)
	if err != nil {
		panic(fmt.Sprintf("Error reading block commit: %v", err))
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

// CommonReadBlockInfo retrieves blockReward, gasUsed and all the transaction receipts belonging to a block.
func CommonReadBlockInfo(db kaidb.Reader, hash common.Hash, number uint64) *types.BlockInfo {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockInfoKey(number, hash))
	if data == nil || len(data) == 0 {
		return nil
	}
	blockInfo := &types.BlockInfo{}
	if err := rlp.DecodeBytes(data, &blockInfo); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	return blockInfo
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

// CommonReadDualEventLookupEntry Retrieves the positional metadata associated with a dual's event
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
		log.Debug("error while get abi from contract address", "err", err, "address", address)
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

// CommonReadHeaderNumber returns the header number assigned to a hash.
func CommonReadHeaderNumber(db kaidb.Reader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerHeightKey(hash))
	if data == nil || len(data) == 0 || len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// CommonStoreHash store a hash into the database.
func CommonStoreHash(db kaidb.Writer, hash *common.Hash) {
	if err := db.Put(hashKey(hash), encodeBoolean(true)); err != nil {
		log.Crit("Failed to store hash", "err", err)
	}
}

// CommonCheckHash returns true if a hash already exists in the database.
func CommonCheckHash(db kaidb.Reader, hash *common.Hash) bool {
	data, _ := db.Get(hashKey(hash))
	if data == nil {
		return false
	}
	return decodeBoolean(data)
}

// CommonStoreTxHash stores a tx hash into the database.
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
func ReadBlockMeta(db kaidb.Reader, height uint64) *types.BlockMeta {
	var pbbm = new(kproto.BlockMeta)
	metaBytes, _ := db.Get(blockMetaKey(height))

	if len(metaBytes) == 0 {
		return nil
	}

	err := proto.Unmarshal(metaBytes, pbbm)
	if err != nil {
		panic(fmt.Errorf("unmarshal to kproto.BlockMeta: %w", err))
	}
	blockMeta, err := types.BlockMetaFromProto(pbbm)
	if err != nil {
		panic(fmt.Errorf("error from proto blockMeta: %w", err))
	}

	return blockMeta
}

func ReadSeenCommit(db kaidb.Reader, height uint64) *types.Commit {
	var pbc = new(kproto.Commit)
	commitBytes, _ := db.Get(seenCommitKey(height))

	if len(commitBytes) == 0 {
		return nil
	}

	err := proto.Unmarshal(commitBytes, pbc)
	if err != nil {
		panic(fmt.Sprintf("error reading block seen commit: %v", err))
	}
	commit, err := types.CommitFromProto(pbc)
	if err != nil {
		panic(fmt.Errorf("error from proto commit: %w", err))
	}
	return commit
}

// ReadBlock returns the Block for the given height
func ReadBlock(db kaidb.Reader, height uint64) *types.Block {
	blockMeta := ReadBlockMeta(db, height)

	if blockMeta == nil {
		return nil
	}

	var buf []byte
	for i := 0; i < int(blockMeta.BlockID.PartsHeader.Total); i++ {
		part := ReadBlockPart(db, height, i)
		buf = append(buf, part.Bytes...)
	}
	pbb := new(kproto.Block)
	err := proto.Unmarshal(buf, pbb)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(fmt.Sprintf("Error reading block: %v", err))
	}

	block, err := types.BlockFromProto(pbb)
	if err != nil {
		panic(fmt.Errorf("error from proto block: %w", err))
	}

	return block
}

// CommonReadHeader retrieves the block header corresponding to the hash.
func CommonReadHeader(db kaidb.Reader, height uint64) *types.Header {
	blockMeta := ReadBlockMeta(db, height)
	return blockMeta.Header
}

// ReadBlockPart returns the block part fo the given height and index
func ReadBlockPart(db kaidb.Reader, height uint64, index int) *types.Part {
	var pbpart = new(kproto.Part)
	partBytes, _ := db.Get(blockPartKey(height, index))

	if len(partBytes) == 0 {
		return nil
	}

	err := proto.Unmarshal(partBytes, pbpart)
	if err != nil {
		panic(fmt.Errorf("unmarshal to kproto.Part failed: %w", err))
	}
	part, err := types.PartFromProto(pbpart)
	if err != nil {
		panic(fmt.Sprintf("Error reading block part: %v", err))
	}

	return part
}

// WriteBlock write block to database
func WriteBlock(db kaidb.Database, block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	if block == nil {
		panic("BlockStore can only save a non-nil block")
	}

	height := block.Height()
	hash := block.Hash()

	batch := db.NewBatch()

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	pbm := blockMeta.ToProto()
	if pbm == nil {
		panic("nil blockmeta")
	}
	metaBytes := mustEncode(pbm)
	_ = batch.Put(blockMetaKey(height), metaBytes)

	// Save block part
	for i := 0; i < int(blockParts.Total()); i++ {
		part := blockParts.GetPart(i)
		writeBlockPart(batch, height, i, part)

	}

	// Save block commit (duplicate and separate from the Block)
	pbc := block.LastCommit().ToProto()
	blockCommitBytes := mustEncode(pbc)
	_ = batch.Put(commitKey(height-1), blockCommitBytes)

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	pbsc := seenCommit.ToProto()
	seenCommitBytes := mustEncode(pbsc)
	if err := batch.Put(seenCommitKey(height), seenCommitBytes); err != nil {
		panic(fmt.Errorf("failed to store seen commit err: %s", err))
	}

	key := headerHeightKey(hash)
	if err := batch.Put(key, encodeBlockHeight(height)); err != nil {
		panic(fmt.Errorf("failed to store hash to height mapping err: %s", err))
	}

	CommonWriteCanonicalHash(batch, hash, height)
	if err := batch.Write(); err != nil {
		panic(fmt.Errorf("failed to store block error: %s", err))
	}
}

func writeBlockPart(db kaidb.Writer, height uint64, index int, part *types.Part) {
	pbp, err := part.ToProto()
	if err != nil {
		panic(fmt.Errorf("unable to make part into proto: %w", err))
	}
	partBytes := mustEncode(pbp)
	_ = db.Put(blockPartKey(height, index), partBytes)
}

// ReadAppHash ...
func ReadAppHash(db kaidb.KeyValueReader, height uint64) common.Hash {
	b, _ := db.Get(calcAppHashKey(height))
	if len(b) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(b)
}

// WriteAppHash ...
func WriteAppHash(db kaidb.KeyValueWriter, height uint64, hash common.Hash) {
	_ = db.Put(calcAppHashKey(height), hash.Bytes())
}

// mustEncode proto encodes a proto.message and panics if fails
func mustEncode(pb proto.Message) []byte {
	bz, err := proto.Marshal(pb)
	if err != nil {
		panic(fmt.Errorf("unable to marshal: %w", err))
	}
	return bz
}
