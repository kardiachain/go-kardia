package state

import (
	"sync"

	"conceptchain/common"
	kaidb "conceptchain/database"
	"conceptchain/trie"
	lru "github.com/hashicorp/golang-lru"
)

// Trie cache generation limit after which to evict trie nodes from memory.
var MaxTrieCacheGen = uint16(120)

const (
	// Number of past tries to keep. This value is chosen such that
	// reasonable chain reorg depths will hit an existing trie.
	maxPastTries = 12

	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root common.Hash) (Trie, error)
}

// Trie is a Kardia Merkle Trie.
type Trie interface {
	TryGet(key []byte) ([]byte, error)
	/*@huny
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit(onleaf trie.LeafCallback) (common.Hash, error)
	Hash() common.Hash
	NodeIterator(startKey []byte) trie.NodeIterator
	GetKey([]byte) []byte // TODO(fjl): remove this when SecureTrie is removed
	Prove(key []byte, fromLevel uint, proofDb kaidb.Putter) error
	*/
}

// NewDatabase creates a backing store for state. The returned database is safe for
// concurrent use and retains cached trie nodes in memory. The pool is an optional
// intermediate trie-node memory pool between the low level storage layer and the
// high level trie abstraction.
func NewDatabase(db kaidb.Database) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:            trie.NewDatabase(db),
		codeSizeCache: csc,
	}
}

type cachingDB struct {
	db            *trie.Database
	mu            sync.Mutex
	pastTries     []*trie.SecureTrie
	codeSizeCache *lru.Cache
}

// OpenTrie opens the main account trie.
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for i := len(db.pastTries) - 1; i >= 0; i-- {
		if db.pastTries[i].Hash() == root {
			return cachedTrie{db.pastTries[i].Copy(), db}, nil
		}
	}
	tr, err := trie.NewSecure(root, db.db, MaxTrieCacheGen)
	if err != nil {
		return nil, err
	}
	return cachedTrie{tr, db}, nil
}

func (db *cachingDB) pushTrie(t *trie.SecureTrie) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(db.pastTries) >= maxPastTries {
		copy(db.pastTries, db.pastTries[1:])
		db.pastTries[len(db.pastTries)-1] = t
	} else {
		db.pastTries = append(db.pastTries, t)
	}
}

// cachedTrie inserts its trie into a cachingDB on commit.
type cachedTrie struct {
	*trie.SecureTrie
	db *cachingDB
}

func (m cachedTrie) Commit(onleaf trie.LeafCallback) (common.Hash, error) {
	root, err := m.SecureTrie.Commit(onleaf)
	if err == nil {
		m.db.pushTrie(m.SecureTrie)
	}
	return root, err
}
