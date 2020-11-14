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
	"math/big"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gogo/protobuf/proto"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto/sha3"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/math"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/trie"
)

var (
	// EmptyRootHash ...
	EmptyRootHash = DeriveSha(Transactions{})
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Kardia blockchain.
type Header struct {
	// basic block info
	Height uint64    `json:"height"       gencodec:"required"`
	Time   time.Time `json:"time"         gencodec:"required"`
	NumTxs uint64    `json:"num_txs"      gencodec:"required"`
	// TODO(namdoh@): Create a separate block type for Dual's blockchain.
	NumDualEvents uint64 `json:"num_dual_events" gencodec:"required"`

	GasLimit uint64 `json:"gasLimit"         gencodec:"required"`
	GasUsed  uint64 `json:"gasUsed"          gencodec:"required"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`

	ProposerAddress common.Address `json:"proposer"            gencodec:"required"`

	// hashes of block data
	LastCommitHash common.Hash `json:"last_commit_hash"    gencodec:"required"` // commit from validators from the last block
	TxHash         common.Hash `json:"data_hash"           gencodec:"required"` // transactions
	// TODO(namdoh@): Create a separate block type for Dual's blockchain.
	DualEventsHash common.Hash `json:"dual_events_hash"    gencodec:"required"` // dual's events

	// hashes from the app output from the prev block
	ValidatorsHash     common.Hash `json:"validators_hash"`      // validators hash for the current block
	NextValidatorsHash common.Hash `json:"next_validators_hash"` // next validators hask for next block
	ConsensusHash      common.Hash `json:"consensus_hash"`       // consensus params for current block
	AppHash            common.Hash `json:"app_hash"`             // state after txs from the previous block
	//@huny LastResultsHash common.Hash `json:"last_results_hash"` // root hash of all results from the txs from the previous block

	// consensus info
	EvidenceHash common.Hash `json:"evidence_hash"` // evidence included in the block
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
	return fmt.Sprintf("Header{Height:%v  Time:%v  NumTxs:%v  LastBlockID:%v  LastCommitHash:%v  TxHash:%v  AppHash:%v  ValidatorsHash:%v  ConsensusHash:%v}#%v",
		h.Height, h.Time, h.NumTxs, h.LastBlockID, h.LastCommitHash.Hex(), h.TxHash.Hex(), h.AppHash.Hex(), h.ValidatorsHash.Hex(), h.ConsensusHash.Hex(), h.Hash().Hex())

}

// String returns a short string representing Header by simplifying byte array to hex, and truncate the first 12 character of hex string
func (h *Header) String() string {
	if h == nil {
		return "nil-Header"
	}
	headerHash := h.Hash()
	return fmt.Sprintf("Header{Height:%v  Time:%v  NumTxs:%v  LastBlockID:%v  LastCommitHash:%v  TxHash:%v  AppHash:%v  ValidatorsHash:%v  ConsensusHash:%v}#%v",
		h.Height, h.Time, h.NumTxs, h.LastBlockID, h.LastCommitHash.Fingerprint(),
		h.TxHash.Fingerprint(), h.AppHash.Fingerprint(), h.ValidatorsHash.Fingerprint(), h.ConsensusHash.Fingerprint(), headerHash.Fingerprint())
}

// ToProto converts Header to protobuf
func (h *Header) ToProto() *kproto.Header {
	if h == nil {
		return nil
	}

	return &kproto.Header{
		Height:             h.Height,
		Time:               h.Time,
		LastBlockId:        h.LastBlockID.ToProto(),
		ValidatorsHash:     h.ValidatorsHash.Bytes(),
		NextValidatorsHash: h.NextValidatorsHash.Bytes(),
		ConsensusHash:      h.ConsensusHash.Bytes(),
		AppHash:            h.AppHash.Bytes(),
		DataHash:           h.TxHash.Bytes(),
		GasLimit:           h.GasLimit,
		EvidenceHash:       h.EvidenceHash.Bytes(),
		LastCommitHash:     h.LastCommitHash.Bytes(),
		ProposerAddress:    h.ProposerAddress.Bytes(),
		NumTxs:             h.NumTxs,
	}
}

// FromProto sets a protobuf Header to the given pointer.
// It returns an error if the header is invalid.
func HeaderFromProto(ph *kproto.Header) (Header, error) {
	if ph == nil {
		return Header{}, errors.New("nil Header")
	}

	h := new(Header)

	bi, err := BlockIDFromProto(&ph.LastBlockId)
	if err != nil {
		return Header{}, err
	}

	//h.Version = ph.Version
	//h.ChainID = ph.ChainID
	h.Height = ph.Height
	h.Time = ph.Time
	h.Height = ph.Height
	h.LastBlockID = *bi
	h.ValidatorsHash = common.BytesToHash(ph.ValidatorsHash)
	h.NextValidatorsHash = common.BytesToHash(ph.NextValidatorsHash)
	h.ConsensusHash = common.BytesToHash(ph.ConsensusHash)
	h.AppHash = common.BytesToHash(ph.AppHash)
	h.TxHash = common.BytesToHash(ph.DataHash)
	h.EvidenceHash = common.BytesToHash(ph.EvidenceHash)
	h.LastCommitHash = common.BytesToHash(ph.LastCommitHash)
	h.ValidatorsHash = common.BytesToHash(ph.ValidatorsHash)
	h.GasLimit = ph.GasLimit
	h.NumTxs = ph.NumTxs
	h.ProposerAddress = common.BytesToAddress(ph.ProposerAddress)

	return *h, nil
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents together.
type Body struct {
	Transactions []*Transaction
	DualEvents   []*DualEvent
	LastCommit   *Commit
}

func (b *Body) Copy() *Body {
	var bodyCopy Body
	bodyCopy.LastCommit = b.LastCommit.Copy()
	bodyCopy.Transactions = make([]*Transaction, len(b.Transactions))
	copy(bodyCopy.Transactions, b.Transactions)
	bodyCopy.DualEvents = make([]*DualEvent, len(b.DualEvents))
	copy(bodyCopy.DualEvents, b.DualEvents)
	return &bodyCopy
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
	evidence     *EvidenceData

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
	Evidence   *EvidenceData
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash and NumTxs in header are ignored and set to values
// derived from the given txs.
func NewBlock(header *Header, txs []*Transaction, lastCommit *Commit, evidence []Evidence) *Block {
	b := &Block{
		header:     CopyHeader(header),
		lastCommit: CopyCommit(lastCommit),
		evidence:   &EvidenceData{Evidence: evidence},
	}

	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.header.NumTxs = uint64(len(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if b.header.LastCommitHash.IsZero() {
		if lastCommit == nil {
			b.header.LastCommitHash = common.NewZeroHash()
		} else {
			b.header.LastCommitHash = lastCommit.Hash()
		}
	}

	if len(evidence) > 0 {
		b.header.EvidenceHash = b.evidence.Hash()
	} else {
		b.header.EvidenceHash = common.NewZeroHash()
	}

	return b
}

// NewDualBlock creates a new block for dual chain. The input data is copied,
// changes to header and to the field values will not affect the
// block.
func NewDualBlock(header *Header, events DualEvents, commit *Commit, evidence []Evidence) *Block {
	b := &Block{
		header:     CopyHeader(header),
		lastCommit: CopyCommit(commit),
		evidence:   &EvidenceData{Evidence: evidence},
	}

	b.header.DualEventsHash = EmptyRootHash

	if b.header.LastCommitHash.IsZero() {
		if commit == nil {
			b.header.LastCommitHash = common.NewZeroHash()
		} else {
			b.header.LastCommitHash = commit.Hash()
		}
	}

	if len(events) == 0 {
		b.header.DualEventsHash = EmptyRootHash
	} else {
		b.header.DualEventsHash = DeriveSha(events)
		b.header.NumDualEvents = uint64(len(events))
		b.dualEvents = make(DualEvents, len(events))
		copy(b.dualEvents, events)
	}

	if len(evidence) > 0 {
		b.header.EvidenceHash = b.evidence.Hash()
	} else {
		b.header.EvidenceHash = common.NewZeroHash()
	}
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
func (b *Block) GasUsed() uint64  { return b.header.GasUsed }
func (b *Block) Time() time.Time  { return b.header.Time }
func (b *Block) NumTxs() uint64   { return b.header.NumTxs }

func (b *Block) ProposerAddress() common.Address { return b.header.ProposerAddress }
func (b *Block) LastCommitHash() common.Hash     { return b.header.LastCommitHash }
func (b *Block) TxHash() common.Hash             { return b.header.TxHash }
func (b *Block) ValidatorHash() common.Hash      { return b.header.ValidatorsHash }
func (b *Block) NextValidatorHash() common.Hash  { return b.header.NextValidatorsHash }
func (b *Block) AppHash() common.Hash            { return b.header.AppHash }
func (b *Block) LastCommit() *Commit             { return b.lastCommit }
func (b *Block) Evidence() *EvidenceData         { return b.evidence }

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
func (b *Block) MakePartSet(partSize uint32) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	pbb, err := b.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(pbb)
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

// ToProto converts Block to protobuf
func (b *Block) ToProto() (*kproto.Block, error) {
	if b == nil {
		return nil, errors.New("nil Block")
	}

	pb := new(kproto.Block)

	pb.Header = *b.header.ToProto()
	pb.LastCommit = b.lastCommit.ToProto()
	pb.Data = b.transactions.ToProto()

	protoEvidence, err := b.evidence.ToProto()
	if err != nil {
		return nil, err
	}
	pb.Evidence = *protoEvidence

	return pb, nil
}

// FromProto sets a protobuf Block to the given pointer.
// It returns an error if the block is invalid.
func BlockFromProto(bp *kproto.Block) (*Block, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(Block)
	h, err := HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.header = &h
	data, err := DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.transactions = data
	b.evidence = &EvidenceData{}
	if err := b.evidence.FromProto(&bp.Evidence); err != nil {
		return nil, err
	}

	if bp.LastCommit != nil {
		lc, err := CommitFromProto(bp.LastCommit)
		if err != nil {
			return nil, err
		}
		b.lastCommit = lc
	}

	return b, b.ValidateBasic()
}

type BlockID struct {
	Hash        common.Hash   `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

func NewZeroBlockID() BlockID {
	return BlockID{}
}

func (blockID *BlockID) IsZero() bool {
	return blockID.Hash.IsZero() && blockID.PartsHeader.IsZero()
}

func (blockID *BlockID) Equal(other BlockID) bool {
	return blockID.Hash.Equal(other.Hash) && blockID.PartsHeader.Equals(other.PartsHeader)
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

// ValidateBasic performs basic validation.
func (blockID BlockID) ValidateBasic() error {

	if err := blockID.PartsHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong PartsHeader: %v", err)
	}
	return nil
}

// ToProto converts BlockID to protobuf
func (blockID *BlockID) ToProto() kproto.BlockID {
	if blockID == nil {
		return kproto.BlockID{}
	}

	return kproto.BlockID{
		Hash:          blockID.Hash.Bytes(),
		PartSetHeader: blockID.PartsHeader.ToProto(),
	}
}

// IsComplete returns true if this is a valid BlockID of a non-nil block.
func (blockID BlockID) IsComplete() bool {
	return !blockID.Hash.IsZero() && !blockID.PartsHeader.IsZero()
}

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func BlockIDFromProto(bID *kproto.BlockID) (*BlockID, error) {
	if bID == nil {
		return nil, errors.New("nil BlockID")
	}

	blockID := new(BlockID)
	ph, err := PartSetHeaderFromProto(&bID.PartSetHeader)
	if err != nil {
		return nil, err
	}

	blockID.PartsHeader = *ph
	blockID.Hash = common.BytesToHash(bID.Hash)
	return blockID, blockID.ValidateBasic()
}

type Blocks []*Block

// BlockMeta contains meta information about a block - namely, it's ID and Header.
type BlockMeta struct {
	BlockID BlockID `json:"block_id"` // the block hash and partsethash
	Header  *Header `json:"header"`   // The block's Header
}

// NewBlockMeta returns a new BlockMeta from the block and its blockParts.
func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		BlockID: BlockID{block.Hash(), blockParts.Header()},
		Header:  block.Header(),
	}
}

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

//-----------------------------------------------------------------------------

// EvidenceData contains any evidence of malicious wrong-doing by validators
type EvidenceData struct {
	Evidence EvidenceList `json:"evidence"`

	// Volatile
	hash     common.Hash
	byteSize int64
}

// Hash returns the hash of the data.
func (data *EvidenceData) Hash() common.Hash {
	if data.hash.IsZero() {
		data.hash = data.Evidence.Hash()
	}
	return data.hash
}

// StringIndented returns a string representation of the evidence.
func (data *EvidenceData) StringIndented(indent string) string {
	if data == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, math.MinInt(len(data.Evidence), 21))
	for i, ev := range data.Evidence {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(data.Evidence))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EvidenceData{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts EvidenceData to protobuf
func (data *EvidenceData) ToProto() (*kproto.EvidenceData, error) {
	if data == nil {
		return nil, errors.New("nil evidence data")
	}

	evi := new(kproto.EvidenceData)
	eviBzs := make([]kproto.Evidence, len(data.Evidence))
	for i := range data.Evidence {
		protoEvi, err := EvidenceToProto(data.Evidence[i])
		if err != nil {
			return nil, err
		}
		eviBzs[i] = *protoEvi
	}
	evi.Evidence = eviBzs

	return evi, nil
}

// FromProto sets a protobuf EvidenceData to the given pointer.
func (data *EvidenceData) FromProto(eviData *kproto.EvidenceData) error {
	if eviData == nil {
		return errors.New("nil evidenceData")
	}

	eviBzs := make(EvidenceList, len(eviData.Evidence))
	for i := range eviData.Evidence {
		evi, err := EvidenceFromProto(&eviData.Evidence[i])
		if err != nil {
			return err
		}
		eviBzs[i] = evi
	}
	data.Evidence = eviBzs
	data.byteSize = int64(eviData.Size())

	return nil
}
