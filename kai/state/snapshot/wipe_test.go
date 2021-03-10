/*
 *  Copyright 2021 KardiaChain
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

package snapshot

import (
	"math/rand"
	"testing"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/common"
)

// Tests that given a database with random data content, all parts of a snapshot
// can be crrectly wiped without touching anything else.
func TestWipe(t *testing.T) {
	// Create a database with some random snapshot data
	db := memorydb.New()

	for i := 0; i < 128; i++ {
		account := randomHash()
		kvstore.WriteAccountSnapshot(db, account, randomHash().Bytes())
		for j := 0; j < 1024; j++ {
			kvstore.WriteStorageSnapshot(db, account, randomHash(), randomHash().Bytes())
		}
	}
	kvstore.WriteSnapshotRoot(db, randomHash())

	// Add some random non-snapshot data too to make wiping harder
	for i := 0; i < 65536; i++ {
		// Generate a key that's the wrong length for a state snapshot item
		var keysize int
		for keysize == 0 || keysize == 32 || keysize == 64 {
			keysize = 8 + rand.Intn(64) // +8 to ensure we will "never" randomize duplicates
		}
		// Randomize the suffix, dedup and inject it under the snapshot namespace
		keysuffix := make([]byte, keysize)
		rand.Read(keysuffix)

		if rand.Int31n(2) == 0 {
			db.Put(append(kvstore.SnapshotAccountPrefix, keysuffix...), randomHash().Bytes())
		} else {
			db.Put(append(kvstore.SnapshotStoragePrefix, keysuffix...), randomHash().Bytes())
		}
	}
	// Sanity check that all the keys are present
	var items int

	it := db.NewIterator(kvstore.SnapshotAccountPrefix, nil)
	defer it.Release()

	for it.Next() {
		key := it.Key()
		if len(key) == len(kvstore.SnapshotAccountPrefix)+common.HashLength {
			items++
		}
	}
	it = db.NewIterator(kvstore.SnapshotStoragePrefix, nil)
	defer it.Release()

	for it.Next() {
		key := it.Key()
		if len(key) == len(kvstore.SnapshotStoragePrefix)+2*common.HashLength {
			items++
		}
	}
	if items != 128+128*1024 {
		t.Fatalf("snapshot size mismatch: have %d, want %d", items, 128+128*1024)
	}
	if hash := kvstore.ReadSnapshotRoot(db); hash == (common.Hash{}) {
		t.Errorf("snapshot block marker mismatch: have %#x, want <not-nil>", hash)
	}
	// Wipe all snapshot entries from the database
	<-wipeSnapshot(db, true)

	// Iterate over the database end ensure no snapshot information remains
	it = db.NewIterator(kvstore.SnapshotAccountPrefix, nil)
	defer it.Release()

	for it.Next() {
		key := it.Key()
		if len(key) == len(kvstore.SnapshotAccountPrefix)+common.HashLength {
			t.Errorf("snapshot entry remained after wipe: %x", key)
		}
	}
	it = db.NewIterator(kvstore.SnapshotStoragePrefix, nil)
	defer it.Release()

	for it.Next() {
		key := it.Key()
		if len(key) == len(kvstore.SnapshotStoragePrefix)+2*common.HashLength {
			t.Errorf("snapshot entry remained after wipe: %x", key)
		}
	}
	if hash := kvstore.ReadSnapshotRoot(db); hash != (common.Hash{}) {
		t.Errorf("snapshot block marker remained after wipe: %#x", hash)
	}
	// Iterate over the database and ensure miscellaneous items are present
	items = 0

	it = db.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		items++
	}
	if items != 65536 {
		t.Fatalf("misc item count mismatch: have %d, want %d", items, 65536)
	}
}
