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
	"github.com/kardiachain/go-kardiamain/lib/merkle"
	krand "github.com/kardiachain/go-kardiamain/lib/rand"
	"github.com/kardiachain/go-kardiamain/types"
	ktime "github.com/kardiachain/go-kardiamain/types/time"
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

// Tests block storage and retrieval operations.
func TestBlockStorage(t *testing.T) {
	db := memorydb.New()

	// Create a test block to move around the database and make sure it's really new
	block := types.NewBlockWithHeader(&types.Header{
		Height:             krand.Uint64(),
		Time:               ktime.Now(),
		NumTxs:             krand.Uint64(),
		GasLimit:           krand.Uint64(),
		LastBlockID:        types.BlockID{},
		ProposerAddress:    common.HexToAddress(krand.Hash(merkle.AddressSize).String()),
		LastCommitHash:     krand.Hash(merkle.Size),
		TxHash:             krand.Hash(merkle.Size),
		ValidatorsHash:     krand.Hash(merkle.Size),
		NextValidatorsHash: krand.Hash(merkle.Size),
		ConsensusHash:      krand.Hash(merkle.Size),
		AppHash:            krand.Hash(merkle.Size),
		EvidenceHash:       krand.Hash(merkle.Size),
	})
	//partsSet := block.MakePartSet(types.BlockPartSizeBytes)

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
	//WriteBlock(db, block, partsSet, &types.Commit{})
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
