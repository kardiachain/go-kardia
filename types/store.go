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
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

// Code using batches should try to add this much data to the batch.
// The value was determined empirically.
const IdealBatchSize = 100 * 1024

// Putter wraps the database write operation supported by both batches and regular databases.
type Putter interface {
	Put(key interface{}, value interface{}) error
}

// Deleter wraps the database delete operation supported by both batches and regular databases.
type Deleter interface {
	Delete(key interface{}) error
}

type Reader interface {
	Has(key interface{}) (bool, error)
	Get(key interface{}) (interface{}, error)
}

type Accessor interface {
	ReadAccessor
	WriteAccessor
	DeleteAccessor
}

// Database wraps all database operations. All methods are safe for concurrent use.
type Database interface {
	Putter
	Deleter
	Accessor
	Reader
	Close()
	NewBatch() Batch
}

// Batch is a write-only database that commits changes to its host database
// when Write is called. Batch cannot be used concurrently.
type Batch interface {
	Putter
	Deleter
	Accessor
	ValueSize() int // amount of data in the batch
	Write() error
	// Reset resets the batch for reuse
	Reset()
}

// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Reader
	ReadAccessor
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Putter
	WriteAccessor
}

// DatabaseDeleter wraps the Delete method of a backing data store.
type DatabaseDeleter interface {
	Deleter
	DeleteAccessor
}

type DeleteAccessor interface {
	DeleteBody(hash common.Hash, height uint64)
	DeleteHeader(hash common.Hash, height uint64)
	DeleteCanonicalHash(number uint64)
}

type WriteAccessor interface {
	WriteBody(hash common.Hash, height uint64, body *Body)
	WriteHeader(header *Header)
	WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue)
	WriteChainConfig(hash common.Hash, cfg *ChainConfig)
	WriteBlock(block *Block, blockParts *PartSet, seenCommit *Commit)
	WriteReceipts(hash common.Hash, height uint64, receipts Receipts)
	WriteCanonicalHash(hash common.Hash, height uint64)
	WriteHeadBlockHash(hash common.Hash)
	WriteHeadHeaderHash(hash common.Hash)
	WriteEvent(smartcontract *KardiaSmartcontract)
	WriteTxLookupEntries(block *Block)
	StoreTxHash(hash *common.Hash)
	StoreHash(hash *common.Hash)
}

type ReadAccessor interface {
	ReadCanonicalHash(height uint64) common.Hash
	ReadChainConfig(hash common.Hash) *ChainConfig
	ReadBlock(logger log.Logger, hash common.Hash, height uint64) *Block
	ReadHeader(hash common.Hash, height uint64) *Header
	ReadBody(hash common.Hash, height uint64) *Body
	ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue
	ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue
	ReadHeadBlockHash() common.Hash
	ReadHeaderHeight(hash common.Hash) *uint64
	ReadHeadHeaderHash() common.Hash
	ReadCommitRLP(height uint64) rlp.RawValue
	ReadCommit(height uint64) *Commit
	ReadTransaction(hash common.Hash) (*Transaction, common.Hash, uint64, uint64)
	ReadDualEvent(hash common.Hash) (*DualEvent, common.Hash, uint64, uint64)
	ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64)
	ReadHeaderNumber(hash common.Hash) *uint64
	ReadReceipts(hash common.Hash, number uint64) Receipts
	ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64)
	ReadSmartContractAbi(address string) *abi.ABI
	ReadSmartContractFromDualAction(action string) (string, *abi.ABI)
	ReadEvent(address string, method string) *WatcherAction
	ReadEvents(address string) []*WatcherAction
	ReadBlockPart(height uint64, index int) *Part
	CheckHash(hash *common.Hash) bool
	CheckTxHash(hash *common.Hash) bool
}
