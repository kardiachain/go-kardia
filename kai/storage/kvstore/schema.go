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
package kvstore

import (
	"encoding/binary"

	"github.com/kardiachain/go-kardia/lib/common"
)

// The fields below define the low level database schema prefixing.
var (
	// headBlockKey tracks the latest known full block's hash.
	headBlockKey = []byte("LastBlock")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerHeightPrefix = []byte("H") // headerHeightPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix = []byte("b")   // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockInfoPrefix = []byte("bin") // blockInfoPrefix + num (uint64 big endian) + hash -> block info

	blockPartPrefix = []byte("p")
	blockMetaPrefix = []byte("bm")

	commitPrefix     = []byte("c")  // commitPrefix + num (uint64 big endian) -> commit
	seenCommitPrefix = []byte("sm") // seenCommitPrefix + num -> seen commit
	appHashPrefix    = []byte("ah") // appHashPrefix + num -> app hash

	configPrefix          = []byte("kardia-config-") // config prefix for the db
	txLookupPrefix        = []byte("l")              // txLookupPrefix + hash -> transaction/receipt lookup metadata
	dualEventLookupPrefix = []byte("de")             // dualEventLookupPrefix + hash -> dual's event lookup metadata
	bloomBitsPrefix       = []byte("B")              // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits

	eventPrefix       = []byte("event")  // event prefix + smartcontract address + method
	eventsPrefix      = []byte("events") // event prefix + smart contract address
	dualActionPrefix  = []byte("dualAction")
	contractAbiPrefix = []byte("abi")

	// Chain index prefixes (use `i` + single byte to avoid mixing data types).
	BloomBitsIndexPrefix = []byte("iB") // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress
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
