package types

import (
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

type StoreDB interface {
	//WriteBody(hash common.Hash, height uint64, body *Body)
	//WriteHeader(header *Header)
	//WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue)
	WriteChainConfig(hash common.Hash, cfg *ChainConfig)
	WriteBlock(*Block, *PartSet, *Commit)
	WriteReceipts(hash common.Hash, height uint64, receipts Receipts)
	WriteCanonicalHash(hash common.Hash, height uint64)
	WriteHeadBlockHash(hash common.Hash)
	WriteHeadHeaderHash(hash common.Hash)
	WriteEvent(smartcontract *KardiaSmartcontract)
	//WriteCommit(height uint64, commit *Commit)
	//WriteCommitRLP(height uint64, rlp rlp.RawValue)
	WriteTxLookupEntries(block *Block)
	StoreTxHash(hash *common.Hash)
	StoreHash(hash *common.Hash)
	WriteAppHash(height uint64, hash common.Hash)

	DB() kaidb.Database

	ReadCanonicalHash(height uint64) common.Hash
	ReadChainConfig(hash common.Hash) *ChainConfig
	ReadBlock(hash common.Hash, height uint64) *Block
	ReadHeader(hash common.Hash, height uint64) *Header
	ReadBody(hash common.Hash, height uint64) *Body
	ReadBlockPart(hash common.Hash, height uint64, index int) *Part
	ReadAppHash(height uint64) common.Hash

	//ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue
	//ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue
	ReadBlockMeta(common.Hash, uint64) *BlockMeta
	ReadHeadBlockHash() common.Hash
	ReadHeaderHeight(hash common.Hash) *uint64
	ReadHeadHeaderHash() common.Hash
	ReadCommitRLP(height uint64) rlp.RawValue
	ReadCommit(height uint64) *Commit
	ReadSeenCommit(height uint64) *Commit
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
	CheckHash(hash *common.Hash) bool
	CheckTxHash(hash *common.Hash) bool

	// Delete
	DeleteBlockMeta(hash common.Hash, height uint64)
	DeleteBlockPart(hash common.Hash, height uint64)
	DeleteCanonicalHash(height uint64)
}
