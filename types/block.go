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
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"math/big"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto/sha3"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/trie"
)

var (
	EmptyRootHash = DeriveSha(Transactions{})
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Kardia blockchain.
type Header struct {
	// basic block info
	Height uint64   `json:"height"       gencodec:"required"`
	Time   *big.Int `json:"time"         gencodec:"required"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	NumTxs uint64   `json:"num_txs"      gencodec:"required"`
	// TODO(namdoh@): Create a separate block type for Dual's blockchain.
	NumDualEvents uint64 `json:"num_dual_events" gencodec:"required"`

	GasLimit uint64 `json:"gasLimit"         gencodec:"required"`
	GasUsed  uint64 `json:"gasUsed"          gencodec:"required"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`
	//@huny TotalTxs    uint64   `json:"total_txs"`

	// hashes of block data
	LastCommitHash common.Hash `json:"last_commit_hash"    gencodec:"required"` // commit from validators from the last block
	TxHash         common.Hash `json:"data_hash"           gencodec:"required"` // transactions

	// TODO(namdoh@): Create a separate block type for Dual's blockchain.
	DualEventsHash common.Hash `json:"dual_events_hash"    gencodec:"required"` // dual's events

	Validator common.Address `json:"validator"`
	// hashes from the app output from the prev block
	ValidatorsHash common.Hash `json:"validators_hash"` // validators for the current block
	ConsensusHash  common.Hash `json:"consensus_hash"`  // consensus params for current block
	AppHash        common.Hash `json:"app_hash"`        // state after txs from the previous block
	//@huny LastResultsHash common.Hash `json:"last_results_hash"` // root hash of all results from the txs from the previous block

	// consensus info
	//@huny EvidenceHash common.Hash `json:"evidence_hash"` // evidence included in the block
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	return rlpHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h))
}

// StringLong returns a long string representing full info about Header
func (h *Header) StringLong() string {
	if h == nil {
		return "nil-Header"
	}
	// TODO(thientn): check why String() of common.Hash is not called when logging, and have to call Hex() instead.
	return fmt.Sprintf("Header{Height:%v  Time:%v  NumTxs:%v  LastBlockID:%v  LastCommitHash:%v TxHash:%v  Root:%v  ValidatorsHash:%v  ConsensusHash:%v}#%v",
		h.Height, time.Unix(h.Time.Int64(), 0), h.NumTxs, h.LastBlockID, h.LastCommitHash.Hex(), h.TxHash.Hex(), h.AppHash.Hex(), h.ValidatorsHash.Hex(), h.ConsensusHash.Hex(), h.Hash().Hex())

}

// String returns a short string representing Header by simplifying byte array to hex, and truncate the first 12 character of hex string
func (h *Header) String() string {
	if h == nil {
		return "nil-Header"
	}
	headerHash := h.Hash()
	return fmt.Sprintf("Header{Height:%v  Time:%v  NumTxs:%v  LastBlockID:%v  LastCommitHash:%v TxHash:%v  Root:%v  ValidatorsHash:%v  ConsensusHash:%v}#%v",
		h.Height, time.Unix(h.Time.Int64(), 0), h.NumTxs, h.LastBlockID, h.LastCommitHash.Fingerprint(), h.TxHash.Hex(), h.AppHash.Fingerprint(), h.ValidatorsHash.Fingerprint(), h.ConsensusHash.Fingerprint(), headerHash.Fingerprint())
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents together.
type Body struct {
	Transactions []*Transaction
	DualEvents   []*DualEvent
	LastCommit   *Commit
}

// Body returns the non-header content of the block.
func (b *Block) Body() *Body {
	return &Body{Transactions: b.transactions, DualEvents: b.dualEvents, LastCommit: b.lastCommit}
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// BlockAccount stores basic data of an account in block.
type BlockAccount struct {
	// Cannot use map because of RLP.
	Addr    *common.Address
	Balance *big.Int
}

// Block represents an entire block in the Kardia blockchain.
type Block struct {
	mtx          sync.Mutex
	header       *Header
	transactions Transactions
	dualEvents   DualEvents
	lastCommit   *Commit

	// caches
	hash atomic.Value
	size atomic.Value
}

// "external" block encoding. used for Kardia protocol, etc.
type extblock struct {
	Header     *Header
	Txs        []*Transaction
	DualEvents []*DualEvent
	LastCommit *Commit
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash and NumTxs in header are ignored and set to values
// derived from the given txs.
func NewBlock(header *Header, txs []*Transaction, lastCommit *Commit) *Block {
	b := &Block{
		header:     header,
		lastCommit: lastCommit,
	}

	b.header.NumTxs = uint64(len(txs))
	if b.header.NumTxs > 0 {
		b.transactions = Transactions(txs)
	}

	b.fillHeader()

	// TODO(namdoh): Store evidence hash.

	return b
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *Block) fillHeader() {
	if b.header.LastCommitHash.IsZero() {
		b.header.LastCommitHash = b.LastCommit().Hash()
	}

	if b.header.TxHash.IsZero() {
		b.header.TxHash = b.transactions.Hash()
	}
}

// NewDualBlock creates a new block for dual chain. The input data is copied,
// changes to header and to the field values will not affect the
// block.
func NewDualBlock(header *Header, events DualEvents, commit *Commit) *Block {
	b := &Block{
		header:     CopyHeader(header),
		lastCommit: CopyCommit(commit),
	}

	b.header.DualEventsHash = EmptyRootHash

	if len(events) == 0 {
		b.header.DualEventsHash = EmptyRootHash
	} else {
		b.header.DualEventsHash = DeriveSha(events)
		b.header.NumDualEvents = uint64(len(events))
		b.dualEvents = make(DualEvents, len(events))
		copy(b.dualEvents, events)
	}

	b.fillHeader()

	// TODO(namdoh): Store evidence hash.

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	return &cpy
}

// CopyHeader creates a deep copy of a block commit to prevent side effects from
// modifying a commit variable.
func CopyCommit(c *Commit) *Commit {
	if c == nil {
		return c
	}
	cpy := *c
	return &cpy
}

//  DecodeRLP implements rlp.Decoder, decodes RLP stream to Block struct.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.transactions, b.dualEvents, b.lastCommit = eb.Header, eb.Txs, eb.DualEvents, eb.LastCommit
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes Block into the RLP stream.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:     b.header,
		Txs:        b.transactions,
		DualEvents: b.dualEvents,
		LastCommit: b.LastCommit(),
	})
}

//  DecodeRLP implements rlp.Decoder, decodes RLP stream to Body struct.
// Custom Encode/Decode for Body because of LastCommit RLP issue#73, otherwise Body can use RLP default decoder.
func (b *Body) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.Transactions, b.DualEvents, b.LastCommit = eb.Txs, eb.DualEvents, eb.LastCommit
	return nil
}

func (b *Body) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:     &Header{},
		Txs:        b.Transactions,
		DualEvents: b.DualEvents,
		LastCommit: b.LastCommit,
	})
}

func (b *Block) Transactions() Transactions { return b.transactions }

func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *Block) DualEvents() DualEvents { return b.dualEvents }

// WithBody returns a new block with the given transaction.
func (b *Block) WithBody(body *Body) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(body.Transactions)),
		dualEvents:   make([]*DualEvent, len(body.DualEvents)),
		lastCommit:   body.LastCommit,
	}
	copy(block.transactions, body.Transactions)
	copy(block.dualEvents, body.DualEvents)
	return block
}

func (b *Block) Height() uint64   { return b.header.Height }
func (b *Block) GasLimit() uint64 { return b.header.GasLimit }
func (b *Block) Time() *big.Int   { return b.header.Time }
func (b *Block) NumTxs() uint64   { return b.header.NumTxs }

func (b *Block) LastCommitHash() common.Hash { return b.header.LastCommitHash }
func (b *Block) TxHash() common.Hash         { return b.header.TxHash }
func (b *Block) LastCommit() *Commit         { return b.lastCommit }
func (b *Block) AppHash() common.Hash        { return b.header.AppHash }

// TODO(namdoh): This is a hack due to rlp nature of decode both nil or empty
// struct pointer as nil. After encoding an empty struct and send it over to
// another node, decoding it would become nil.
func (b *Block) SetLastCommit(c *Commit) {
	b.lastCommit = c
}

func (b *Block) Header() *Header { return CopyHeader(b.header) }
func (b *Block) HashesTo(hash common.Hash) bool {
	if hash.IsZero() {
		return false
	}
	if b == nil {
		return false
	}
	return b.Hash().Equal(hash)
}

// MakePartSet returns a PartSet containing parts of a serialized block.
// This is the form in which the block is gossipped to peers.
// CONTRACT: partSize is greater than zero.
func (b *Block) MakePartSet(partSize int) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	bz, err := rlp.EncodeToBytes(b)
	if err != nil {
		panic(err)
	}

	return NewPartSetFromData(bz, partSize)
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previously cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	newTxs := uint64(len(b.transactions))
	if b.header.NumTxs != newTxs {
		return fmt.Errorf("wrong Block.Header.NumTxs. Expected %v, got %v", newTxs, b.header.NumTxs)
	}

	if err := b.header.LastBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong Header.LastBlockID: %v", err)
	}

	// Validate the last commit and its hash.
	if b.header.Height > 1 {
		if b.lastCommit == nil {
			return errors.New("nil LastCommit")
		}
		if err := b.lastCommit.ValidateBasic(); err != nil {
			return err
		}
	}

	if b.lastCommit == nil && !b.header.LastCommitHash.IsZero() {
		return fmt.Errorf("Wrong Block.Header.LastCommitHash.  lastCommit is nil, but expect zero hash, but got: %v", b.header.LastCommitHash)
	} else if b.lastCommit != nil && !b.header.LastCommitHash.Equal(b.lastCommit.Hash()) {
		return fmt.Errorf("Wrong Block.Header.LastCommitHash.  Expected %v, got %v.  Last commit %v", b.header.LastCommitHash, b.lastCommit.Hash(), b.lastCommit)
	}
	// TODO(namdoh): Re-enable check for Data hash.
	//b.logger.Info("Block.ValidateBasic() - not yet implement validating data hash.")
	//if !bytes.Equal(b.DataHash, b.Data.Hash()) {
	//	return fmt.Errorf("Wrong Block.Header.DataHash.  Expected %v, got %v", b.DataHash, b.Data.Hash())
	//}
	//if !bytes.Equal(b.EvidenceHash, b.Evidence.Hash()) {
	//	return errors.New(cmn.Fmt("Wrong Block.Header.EvidenceHash.  Expected %v, got %v", b.EvidenceHash, b.Evidence.Hash()))
	//}

	//b.logger.Info("Block.ValidateBasic() - implement validate DualEvents.")

	return nil
}

// StringLong returns a long string representing full info about Block
func (b *Block) StringLong() string {
	if b == nil {
		return "nil-Block"
	}

	return fmt.Sprintf("Block{%v  %v  %v  %v}#%v",
		b.header, b.transactions, b.dualEvents, b.lastCommit, b.Hash().Hex())
}

// String returns a short string representing block by simplifying block header and lastcommit
func (b *Block) String() string {
	if b == nil {
		return "nil-Block"
	}
	blockHash := b.Hash()
	return fmt.Sprintf("Block{h:%v  tx:%v  de:%v  c:%v}#%v",
		b.header, b.transactions, b.dualEvents, b.lastCommit, blockHash.Fingerprint())
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if b == nil {
		log.Warn("Hashing nil block")
		return common.Hash{}
	}

	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

type BlockID struct {
	Hash        common.Hash   `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

func NewZeroBlockID() BlockID {
	return BlockID{}
}

func (b *BlockID) IsZero() bool {
	return b.Hash.IsZero() && b.PartsHeader.IsZero()
}

func (b *BlockID) Equal(other BlockID) bool {
	return b.Hash.Equal(other.Hash) && b.PartsHeader.Equals(other.PartsHeader)
}

// Key returns a machine-readable string representation of the BlockID
func (blockID *BlockID) Key() string {
	return string(blockID.Hash.String() + blockID.PartsHeader.Hash.String())
}

// String returns the first 12 characters of hex string representation of the BlockID
func (blockID BlockID) String() string {
	return fmt.Sprintf("%s:%s", blockID.Hash.Fingerprint(), blockID.PartsHeader.Hash.Fingerprint())
}

func (blockID BlockID) StringLong() string {
	return fmt.Sprintf("%s:%s", blockID.Hash.String(), blockID.PartsHeader.Hash.String())
}

// IsComplete returns true if this is a valid BlockID of a non-nil block.
func (blockID BlockID) IsComplete() bool {
	return !blockID.Hash.IsZero() && !blockID.PartsHeader.IsZero()
}

// ValidateBasic performs basic validation.
func (blockID BlockID) ValidateBasic() error {

	if err := blockID.PartsHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong PartsHeader: %v", err)
	}
	return nil
}

type Blocks []*Block

type BlockBy func(b1, b2 *Block) bool

func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Height(b1, b2 *Block) bool { return b1.header.Height < b2.header.Height }

// Helper function
type DerivableList interface {
	Len() int
	GetRlp(i int) []byte
}

func DeriveSha(list DerivableList) common.Hash {
	keybuf := new(bytes.Buffer)
	t := new(trie.Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		t.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return t.Hash()

	//return common.BytesToHash([]byte(""))
}
