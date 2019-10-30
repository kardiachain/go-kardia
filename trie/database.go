package trie

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

// Database wraps all database operations. All methods are safe for concurrent use.
type Database interface {
	Putter
	Deleter
	Reader

	NewBatch() Batch
}

// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Reader
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Putter
}

// Batch is a write-only database that commits changes to its host database
// when Write is called. Batch cannot be used concurrently.
type Batch interface {
	Putter
	Deleter
	ValueSize() int // amount of data in the batch
	Write() error
	// Reset resets the batch for reuse
	Reset()
}
