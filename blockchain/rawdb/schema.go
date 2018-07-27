// Package rawdb contains a collection of low level database accessors.
package rawdb

import (
	"encoding/binary"
	"github.com/kardiachain/go-kardia/lib/common"
)

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

	blockBodyPrefix = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body

	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	configPrefix = []byte("kardia-config-") // config prefix for the db
)

// encodeBlockHeight encodes a block height as big endian uint64
func encodeBlockHeight(height uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, height)
	return enc
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

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}
