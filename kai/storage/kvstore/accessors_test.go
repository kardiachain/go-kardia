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
	"testing"
	"time"

	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
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

// Tests block storage and retrieval operations.
func TestBlockStorage(t *testing.T) {
	db := memorydb.New()

	vote := &types.Vote{
		ValidatorIndex: 1,
		Height:         1336,
		Round:          1,
		Timestamp:      time.Now(),
		Type:           kproto.PrecommitType,
		BlockID:        types.BlockID{},
	}
	lastCommit := &types.Commit{
		Height:     1336,
		Round:      1,
		Signatures: []types.CommitSig{vote.CommitSig(), types.NewCommitSigAbsent()},
	}
	header := &types.Header{
		Height:         1337,
		Time:           time.Now(),
		TxHash:         types.EmptyRootHash,
		LastCommitHash: lastCommit.Hash(),
	}
	block := types.NewBlock(header, nil, lastCommit, nil)
	partsSet := block.MakePartSet(types.BlockPartSizeBytes)

	// Check that no entries are in a pristine database
	if entry := ReadBlock(db, block.Height()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	if entry := ReadHeader(db, block.Height()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	if entry := ReadBody(db, block.Height()); entry != nil {
		t.Fatalf("Non existent body returned: %v", entry)
	}

	// Write and verify the block in the database
	WriteBlock(db, block, partsSet, lastCommit)

	// Check that header height are present
	if entry := ReadHeaderHeight(db, block.Hash()); *entry != block.Height() {
		t.Fatalf("Block height mismatch: have %v, want %v", *entry, block.Height())
	}

	// Check that header are present
	if entry := ReadHeader(db, block.Height()); entry == nil {
		t.Fatalf("Header not found")
	}

	// Check block meta are present
	if entry := ReadBlockMeta(db, block.Height()); entry == nil {
		t.Fatalf("Block Meta not found")
	}

	// Check block parts are present
	for i := uint32(0); i < partsSet.Total(); i++ {
		if entry := ReadBlockPart(db, block.Height(), int(i)); entry == nil {
			t.Fatalf("Block part not found index: %v", i)
		}
	}

	// Check block commit are present
	if entry := ReadCommit(db, block.Height()-1); entry == nil {
		t.Fatalf("Block commit not found")
	}

	// Check last commit are present
	//if entry := ReadSeenCommit(db, lastCommit.Height); entry != block.LastCommit() {
	//	t.Fatalf("Read commit mismatch: have %v, want %v", entry, block.LastCommit())
	//}
}

func TestAppHashStorage(t *testing.T) {
	db := memorydb.New()
	height := uint64(1337)
	block := types.NewBlockWithHeader(&types.Header{Height: height})

	// Check that no entries are in a pristine database
	if entry := ReadAppHash(db, height); entry != (common.Hash{}) {
		t.Fatalf("Non app hash entry returned: %v", entry)
	}
	// Assign separate entries for the head header and block
	WriteAppHash(db, height, block.Hash())

	// Check that entries are present
	if entry := ReadAppHash(db, height); entry.Equal(common.Hash{}) {
		t.Fatalf("App hash mismatch: have %v, want %v", entry, block.Hash())
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
	// Write and verify the entries in the database
	WriteCanonicalHash(db, hash, number)
	if entry := ReadCanonicalHash(db, number); entry == (common.Hash{}) {
		t.Fatalf("Stored canonical mapping not found")
	} else if entry != hash {
		t.Fatalf("Retrieved canonical mapping mismatch: have %v, want %v", entry, hash)
	}
	// Delete the entries and verify the execution
	DeleteCanonicalHash(db, number)
	if entry := ReadCanonicalHash(db, number); entry != (common.Hash{}) {
		t.Fatalf("Deleted canonical mapping returned: %v", entry)
	}
}
