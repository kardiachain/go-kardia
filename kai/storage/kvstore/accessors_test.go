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

package kvstore

import (
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
	"testing"
)

// Tests that head headers and head blocks can be assigned, individually.
func TestHeadStorage(t *testing.T) {
	db := memorydb.New()

	block := types.NewBlockWithHeader(&types.Header{Height: uint64(1337)})

	// Check that no head entries are in a pristine database
	if entry := ReadHeadBlockHash(db); entry != (common.Hash{}) {
		t.Fatalf("Non head block entry returned: %v", entry)
	}
	// Assign separate entries for the head header and block
	WriteHeadBlockHash(db, block.Hash())

	// Check that both heads are present, and different (i.e. two heads maintained)
	if entry := ReadHeadBlockHash(db); entry != block.Hash() {
		t.Fatalf("Head header hash mismatch: have %v, want %v", entry, block.Hash())
	}
}

// Tests that canonical numbers can be mapped to hashes and retrieved.
func TestCanonicalMappingStorage(t *testing.T) {
	db := memorydb.New()

	// Create a test canonical number and assinged hash to move around
	hash, number := common.Hash{0: 0xff}, uint64(314)
	if entry := ReadCanonicalHash(db, number); entry != (common.Hash{}) {
		t.Fatalf("Non existent canonical mapping returned: %v", entry)
	}
	// Write and verify the TD in the database
	WriteCanonicalHash(db, hash, number)
	if entry := ReadCanonicalHash(db, number); entry == (common.Hash{}) {
		t.Fatalf("Stored canonical mapping not found")
	} else if entry != hash {
		t.Fatalf("Retrieved canonical mapping mismatch: have %v, want %v", entry, hash)
	}
	// Delete the TD and verify the execution
	DeleteCanonicalHash(db, number)
	if entry := ReadCanonicalHash(db, number); entry != (common.Hash{}) {
		t.Fatalf("Deleted canonical mapping returned: %v", entry)
	}
}
