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

// Package rawdb contains a collection of low level database accessors.
package rawdb

import (
	"bytes"
	"encoding/binary"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/metrics"
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVersionKey tracks the current database version.
	databaseVersionKey = []byte("DatabaseVersion")

	// headBlockKey tracks the latest known full block's hash.
	headBlockKey = []byte("LastBlock")

	// snapshotDisabledKey flags that the snapshot should not be maintained due to initial sync.
	snapshotDisabledKey = []byte("SnapshotDisabled")

	// SnapshotRootKey tracks the hash of the last snapshot.
	SnapshotRootKey = []byte("SnapshotRoot")

	// snapshotJournalKey tracks the in-memory diff layers across restarts.
	snapshotJournalKey = []byte("SnapshotJournal")

	// snapshotGeneratorKey tracks the snapshot generation marker across restarts.
	snapshotGeneratorKey = []byte("SnapshotGenerator")

	// snapshotRecoveryKey tracks the snapshot recovery marker across restarts.
	snapshotRecoveryKey = []byte("SnapshotRecovery")

	// snapshotSyncStatusKey tracks the snapshot sync status across restarts.
	snapshotSyncStatusKey = []byte("SnapshotSyncStatus")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerHeightPrefix = []byte("H") // headerHeightPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix  = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockInfoPrefix  = []byte("i") // blockInfoPrefix + num (uint64 big endian) + hash -> block info
	blockPartPrefix  = []byte("p")
	blockMetaPrefix  = []byte("m")
	commitPrefix     = []byte("c")  // commitPrefix + num (uint64 big endian) -> commit
	seenCommitPrefix = []byte("sm") // seenCommitPrefix + num -> seen commit
	appHashPrefix    = []byte("ah") // appHashPrefix + num -> app hash

	eventPrefix           = []byte("event")  // event prefix + smartcontract address + method
	eventsPrefix          = []byte("events") // event prefix + smart contract address
	dualActionPrefix      = []byte("dualAction")
	dualEventLookupPrefix = []byte("de") // dualEventLookupPrefix + hash -> dual's event lookup metadata

	txLookupPrefix        = []byte("l") // txLookupPrefix + hash -> transaction/receipt lookup metadata
	bloomBitsPrefix       = []byte("B") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits
	SnapshotAccountPrefix = []byte("a") // SnapshotAccountPrefix + account hash -> account trie value
	SnapshotStoragePrefix = []byte("o") // SnapshotStoragePrefix + account hash + storage hash -> storage trie value
	contractAbiPrefix     = []byte("C")

	// Path-based storage scheme of merkle patricia trie.
	trieNodeAccountPrefix = []byte("A") // trieNodeAccountPrefix + hexPath -> trie node
	trieNodeStoragePrefix = []byte("O") // trieNodeStoragePrefix + accountHash + hexPath -> trie node

	PreimagePrefix = []byte("secure-key-")    // PreimagePrefix + hash -> preimage
	configPrefix   = []byte("kardia-config-") // config prefix for the db
	genesisPrefix  = []byte("kardia-genesis-") // genesis state prefix for the db

	// BloomBitsIndexPrefix is the data table of a chain indexer to track its progress
	BloomBitsIndexPrefix = []byte("iB") // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress

	preimageCounter    = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)
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

// encodeIndex encodes an index as uint32
func encodeIndex(index uint32) []byte {
	enc := make([]byte, 4)
	binary.BigEndian.PutUint32(enc, index)
	return enc
}

// headerKey = headerPrefix + hash + num (uint64 big endian)
func headerKey(height uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(height uint64) []byte {
	return append(append(headerPrefix, encodeBlockHeight(height)...), headerHashSuffix...)
}

// headerheightKey = headerheightPrefix + hash
func headerHeightKey(hash common.Hash) []byte {
	return append(headerHeightPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(height uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
}

// blockInfoKey = blockInfoPrefix + num (uint64 big endian) + hash
func blockInfoKey(height uint64, hash common.Hash) []byte {
	return append(append(blockInfoPrefix, encodeBlockHeight(height)...), hash.Bytes()...)
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

func eventKey(smartContractAddress string, method string) []byte {
	return append(append(eventPrefix, []byte(smartContractAddress)...), []byte(method)...)
}

func eventsKey(smartContractAddress string) []byte {
	return append(eventsPrefix, []byte(smartContractAddress)...)
}

func dualActionKey(action string) []byte {
	return append(dualActionPrefix, []byte(action)...)
}

func contractAbiKey(smartContractAddress string) []byte {
	return append(contractAbiPrefix, []byte(smartContractAddress)...)
}

func blockMetaKey(height uint64) []byte {
	return append(blockMetaPrefix, encodeBlockHeight(height)...)
}

func blockPartKey(height uint64, index int) []byte {
	return append(blockPartPrefix, append(encodeBlockHeight(height), encodeIndex(uint32(index))...)...)
}

func seenCommitKey(height uint64) []byte {
	return append(seenCommitPrefix, encodeBlockHeight(height)...)
}

func calcAppHashKey(height uint64) []byte {
	return append(appHashPrefix, encodeBlockHeight(height)...)
}

// preimageKey = PreimagePrefix + hash
func preimageKey(hash common.Hash) []byte {
	return append(PreimagePrefix, hash.Bytes()...)
}

// accountTrieNodeKey = trieNodeAccountPrefix + nodePath.
func accountTrieNodeKey(path []byte) []byte {
	return append(trieNodeAccountPrefix, path...)
}

// storageTrieNodeKey = trieNodeStoragePrefix + accountHash + nodePath.
func storageTrieNodeKey(accountHash common.Hash, path []byte) []byte {
	return append(append(trieNodeStoragePrefix, accountHash.Bytes()...), path...)
}

// accountSnapshotKey = SnapshotAccountPrefix + hash
func accountSnapshotKey(hash common.Hash) []byte {
	return append(SnapshotAccountPrefix, hash.Bytes()...)
}

// storageSnapshotKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotKey(accountHash, storageHash common.Hash) []byte {
	return append(append(SnapshotStoragePrefix, accountHash.Bytes()...), storageHash.Bytes()...)
}

// storageSnapshotsKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotsKey(accountHash common.Hash) []byte {
	return append(SnapshotStoragePrefix, accountHash.Bytes()...)
}

// IsCodeKey reports whether the given byte slice is the key of contract code,
// if so return the raw code hash as well.
func IsCodeKey(key []byte) (bool, []byte) {
	if bytes.HasPrefix(key, contractAbiPrefix) && len(key) == common.HashLength+len(contractAbiPrefix) {
		return true, key[len(contractAbiPrefix):]
	}
	return false, nil
}

// genesisStateSpecKey = genesisPrefix + hash
func genesisStateSpecKey(hash common.Hash) []byte {
	return append(genesisPrefix, hash.Bytes()...)
}
