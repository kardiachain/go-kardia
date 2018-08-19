package rawdb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"

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
	log.Debug(fmt.Sprintf("headerKey=%v", headerHashKey(height)))
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
func ReadBlock(db DatabaseReader, hash common.Hash, height uint64) *types.Block {
	header := ReadHeader(db, hash, height)
	if header == nil {
		return nil
	}
	body := ReadBody(db, hash, height)
	if body == nil {
		return nil
	}
	return types.NewBlockWithHeader(header).WithBody(body)
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
