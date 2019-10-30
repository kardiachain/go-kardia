package trie

import (
	"errors"
	"sync"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

type MemStore struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func NewMemStore() *MemStore {
	return &MemStore{
		db: make(map[string][]byte),
	}
}

func NewMemStoreWithCap(size int) *MemStore {
	return &MemStore{
		db: make(map[string][]byte, size),
	}
}

func (db *MemStore) Put(key, value interface{}) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	switch value.(type) {
	case rlp.RawValue:
		db.db[string(key.([]byte))] = common.CopyBytes(value.(rlp.RawValue))
	default:
		db.db[string(key.([]byte))] = common.CopyBytes(value.([]byte))
	}
	return nil
}

func (db *MemStore) Has(key interface{}) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.db[string(key.([]byte))]
	return ok, nil
}

func (db *MemStore) Get(key interface{}) (interface{}, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if entry, ok := db.db[string(key.([]byte))]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, errors.New("not found")
}

func (db *MemStore) Keys() [][]byte {
	db.lock.RLock()
	defer db.lock.RUnlock()

	keys := [][]byte{}
	for key := range db.db {
		keys = append(keys, []byte(key))
	}
	return keys
}

func (db *MemStore) Delete(key interface{}) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	delete(db.db, string(key.([]byte)))
	return nil
}

func (db *MemStore) Close() {}

func (db *MemStore) NewBatch() Batch {
	return &memBatch{db: db}
}

func (db *MemStore) Len() int { return len(db.db) }

type keyValue struct{ k, v []byte }

type memBatch struct {
	db     *MemStore
	writes []keyValue
	size   int
}

func (b *memBatch) Put(key, value interface{}) error {
	b.writes = append(b.writes, keyValue{common.CopyBytes(key.([]byte)), common.CopyBytes(value.([]byte))})
	b.size += len(value.([]byte))
	return nil
}

func (b *memBatch) Delete(key interface{}) error {
	b.writes = append(b.writes, keyValue{common.CopyBytes(key.([]byte)), nil})
	return nil
}

func (b *memBatch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, kv := range b.writes {
		if kv.v == nil {
			delete(b.db.db, string(kv.k))
			continue
		}
		b.db.db[string(kv.k)] = kv.v
	}
	return nil
}

func (b *memBatch) ValueSize() int {
	return b.size
}

func (b *memBatch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

func (db *memBatch) Has(key interface{}) (bool, error) {
	db.db.lock.RLock()
	defer db.db.lock.RUnlock()

	return db.db.Has(key.([]byte))
}

func (db *memBatch) Get(key interface{}) (interface{}, error) {
	db.db.lock.RLock()
	defer db.db.lock.RUnlock()

	return db.db.Get(key.([]byte))
}
