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

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto/sha3"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/trie"
	"math/big"
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
	NumTxs uint64   `json:"num_txs"      gencodec:"required`

	GasLimit uint64 `json:"gasLimit"         gencodec:"required"`
	GasUsed  uint64 `json:"gasUsed"          gencodec:"required"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`
	//@huny TotalTxs    uint64   `json:"total_txs"`

	Coinbase common.Address `json:"miner"            gencodec:"required"`

	// hashes of block data
	LastCommitHash common.Hash `json:"last_commit_hash"    gencodec:"required"` // commit from validators from the last block
	TxHash         common.Hash `json:"data_hash"           gencodec:"required"` // transactions
	Root           common.Hash `json:"stateRoot"           gencodec:"required"` // state root
	ReceiptHash    common.Hash `json:"receiptsRoot"        gencodec:"required"` // receipt root
	Bloom          Bloom       `json:"logsBloom"           gencodec:"required"`

	// hashes from the app output from the prev block
	ValidatorsHash common.Hash `json:"validators_hash"` // validators for the current block
	ConsensusHash  common.Hash `json:"consensus_hash"`  // consensus params for current block
	//@huny AppHash         common.Hash `json:"app_hash"`          // state after txs from the previous block
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

func (h *Header) String() string {
	if h == nil {
		return "nil-Header"
	}
	// TODO(thientn): check why String() of common.Hash is not called when logging, and have to call Hex() instead.
	return fmt.Sprintf("Header{Height:%v  Time:%v  NumTxs:%v  LastBlockID:%v  LastCommitHash:%v  TxHash:%v  ValidatorsHash:%v  ConsensusHash:%v}#%v",
		h.Height, time.Unix(h.Time.Int64(), 0), h.NumTxs, h.LastBlockID, h.LastCommitHash.Hex(), h.TxHash.Hex(), h.ValidatorsHash.Hex(), h.ConsensusHash.Hex(), h.Hash().Hex())

}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents together.
type Body struct {
	Transactions []*Transaction
	LastCommit   *Commit
	Accounts     AccountStates
}

// Body returns the non-header content of the block.
func (b *Block) Body() *Body {
	return &Body{Transactions: b.transactions, LastCommit: b.lastCommit, Accounts: b.accounts}
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// AccountStates keeps the world state of accounts.
type AccountStates []*BlockAccount

func (a AccountStates) String() string {
	var accountsS string
	if a != nil || len(a) > 0 {
		var buffer bytes.Buffer
		for _, account := range a {
			hexS := account.Addr.Hex()
			buffer.WriteString(fmt.Sprintf("[%v]:%d,", hexS[len(hexS)-5:len(hexS)], account.Balance.Int64()))
		}
		accountsS = fmt.Sprintf("AccountStates:(%v)", buffer.String())
	} else {
		if a == nil {
			accountsS = "AccountStates:nil"
		} else {
			accountsS = "AccountStates:[]"
		}
	}

	return accountsS
}

// BlockAccount stores basic data of an account in block.
type BlockAccount struct {
	// Cannot use map because of RLP.
	Addr    *common.Address
	Balance *big.Int
}

func (s AccountStates) GetAccount(address *common.Address) *BlockAccount {
	for _, account := range s {
		if account.Addr.String() == address.String() {
			return account
		}
	}
	return nil
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	mtx          sync.Mutex
	header       *Header
	transactions Transactions
	lastCommit   *Commit
	accounts     AccountStates

	// caches
	hash atomic.Value
	size atomic.Value
}

// "external" block encoding. used for Kardia protocol, etc.
type extblock struct {
	Header     *Header
	Txs        []*Transaction
	LastCommit *Commit
	Accounts   AccountStates
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash and NumTxs in header are ignored and set to values
// derived from the given txs.
func NewBlock(header *Header, txs []*Transaction, receipts []*Receipt, commit *Commit, accounts AccountStates) *Block {
	b := &Block{header: CopyHeader(header), lastCommit: CopyCommit(commit)}

	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.header.NumTxs = uint64(len(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	if b.header.LastCommitHash.IsZero() {
		if commit == nil {
			log.Error("NewBlock - commit should never be nil.")
			b.header.LastCommitHash = common.NewZeroHash()
		} else {
			log.Error("Compute last commit hash", "commit", commit)
			b.header.LastCommitHash = commit.Hash()
		}
	}

	// TODO(thientn): Creates and save a copy.
	b.accounts = accounts

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

// DecodeRLP decodes the Kardia
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.transactions, b.lastCommit, b.accounts = eb.Header, eb.Txs, eb.LastCommit, eb.Accounts
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:     b.header,
		Txs:        b.transactions,
		LastCommit: b.lastCommit,
		Accounts:   b.accounts,
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

// WithBody returns a new block with the given transaction.
func (b *Block) WithBody(body *Body) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(body.Transactions)),
		lastCommit:   body.LastCommit,
		accounts:     body.Accounts,
	}
	copy(block.transactions, body.Transactions)
	return block
}

func (b *Block) Height() uint64   { return b.header.Height }
func (b *Block) GasLimit() uint64 { return b.header.GasLimit }
func (b *Block) GasUsed() uint64  { return b.header.GasUsed }
func (b *Block) Time() *big.Int   { return b.header.Time }
func (b *Block) NumTxs() uint64   { return b.header.NumTxs }

func (b *Block) LastCommitHash() common.Hash { return b.header.LastCommitHash }
func (b *Block) TxHash() common.Hash         { return b.header.TxHash }
func (b *Block) Root() common.Hash           { return b.header.Root }
func (b *Block) ReceiptHash() common.Hash    { return b.header.ReceiptHash }
func (b *Block) Bloom() Bloom                { return b.header.Bloom }
func (b *Block) LastCommit() *Commit         { return b.lastCommit }
func (b *Block) Accounts() AccountStates     { return b.accounts }

// TODO(namdoh): This is a hack due to rlp nature of decode both nil or empty
// struct pointer as nil. After encoding an empty struct and send it over to
// another node, decoding it would become nil.
func (b *Block) SetLastCommit(c *Commit) {
	log.Error("SetLastCommit is a hack. Remove asap!!")
	b.lastCommit = c
}

func (b *Block) Header() *Header { return CopyHeader(b.header) }
func (b *Block) HashesTo(id BlockID) bool {
	return b.Hash().Equal(common.Hash(id))
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
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
		return errors.New("Nil blocks are invalid")
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	newTxs := uint64(len(b.transactions))
	if b.header.NumTxs != newTxs {
		return fmt.Errorf("Wrong Block.Header.NumTxs. Expected %v, got %v", newTxs, b.header.NumTxs)
	}

	if b.lastCommit == nil && !b.header.LastCommitHash.IsZero() {
		return fmt.Errorf("Wrong Block.Header.LastCommitHash.  lastCommit is nil, but expect zero hash, but got: %v", b.header.LastCommitHash)
	} else if b.lastCommit != nil && !b.header.LastCommitHash.Equal(b.lastCommit.Hash()) {
		return fmt.Errorf("Wrong Block.Header.LastCommitHash.  Expected %v, got %v", b.header.LastCommitHash, b.lastCommit.Hash())
	}
	if b.header.Height != 1 {
		if err := b.lastCommit.ValidateBasic(); err != nil {
			return err
		}
	}
	// TODO(namdoh): Re-enable check for Data hash.
	log.Error("Block.ValidateBasic() - not yet implement validating data hash.")
	//if !bytes.Equal(b.DataHash, b.Data.Hash()) {
	//	return fmt.Errorf("Wrong Block.Header.DataHash.  Expected %v, got %v", b.DataHash, b.Data.Hash())
	//}
	//if !bytes.Equal(b.EvidenceHash, b.Evidence.Hash()) {
	//	return errors.New(cmn.Fmt("Wrong Block.Header.EvidenceHash.  Expected %v, got %v", b.EvidenceHash, b.Evidence.Hash()))
	//}

	return nil
}

func (b *Block) String() string {
	if b == nil {
		return "nil-Block"
	}

	return fmt.Sprintf("Block{%v  %v  %v %v}#%v",
		b.header, b.transactions, b.lastCommit, b.accounts, b.Hash().Hex())
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func (b *Block) BlockID() BlockID {
	return BlockID(b.Hash())
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

type BlockID common.Hash

func (id BlockID) String() string {
	return common.Hash(id).Hex()
	return common.Hash(id).Hex()
}

func NewZeroBlockID() BlockID {
	return BlockID{}
}

func (b *BlockID) IsZero() bool {
	zero := BlockID{}
	return bytes.Equal(b[:], zero[:])
}

func (b *BlockID) Equal(id BlockID) bool {
	return common.Hash(*b).Equal(common.Hash(id))
}

// Key returns a machine-readable string representation of the BlockID
func (blockID *BlockID) Key() string {
	return string(blockID[:])
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
	trie := new(trie.Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()

	//return common.BytesToHash([]byte(""))
}
