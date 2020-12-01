/*
 *  Copyright 2020 KardiaChain
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
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
)

type StoreDB interface {
	WriteChainConfig(hash common.Hash, cfg *configs.ChainConfig)
	WriteBlock(*Block, *PartSet, *Commit)
	WriteBlockInfo(hash common.Hash, height uint64, blockInfo *BlockInfo)
	WriteCanonicalHash(hash common.Hash, height uint64)
	WriteHeadBlockHash(common.Hash)
	WriteEvent(smartcontract *KardiaSmartcontract)
	WriteTxLookupEntries(block *Block)
	StoreTxHash(hash *common.Hash)
	StoreHash(hash *common.Hash)

	DB() kaidb.Database

	ReadCanonicalHash(height uint64) common.Hash
	ReadChainConfig(hash common.Hash) *configs.ChainConfig
	ReadBlock(height uint64) *Block
	ReadHeader(height uint64) *Header
	ReadBody(height uint64) *Body
	ReadBlockMeta(uint64) *BlockMeta
	ReadBlockPart(height uint64, index int) *Part
	ReadHeadBlockHash() common.Hash
	ReadHeaderHeight(hash common.Hash) *uint64
	ReadHeadHeaderHash() common.Hash
	ReadCommit(height uint64) *Commit
	ReadSeenCommit(height uint64) *Commit
	ReadTransaction(hash common.Hash) (*Transaction, common.Hash, uint64, uint64)
	//ReadDualEvent(hash common.Hash) (*DualEvent, common.Hash, uint64, uint64)
	//ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64)
	ReadHeaderNumber(hash common.Hash) *uint64
	ReadBlockInfo(hash common.Hash, number uint64) *BlockInfo
	ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64)
	ReadSmartContractAbi(address string) *abi.ABI
	ReadEvent(address string, method string) *Watcher
	ReadEvents(address string) (string, []*Watcher)
	CheckHash(hash *common.Hash) bool
	CheckTxHash(hash *common.Hash) bool

	// Delete
	DeleteBlockMeta(height uint64) error
	DeleteBlockPart(height uint64) error
	DeleteCanonicalHash(height uint64)
}
