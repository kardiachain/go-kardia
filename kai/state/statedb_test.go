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

package state

import (
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
)

// Tests that updating a state trie does not leak any database writes prior to
// actually committing the state.
func TestUpdateLeaks(t *testing.T) {
	// Create an empty state database
	db := memorydb.New()
	state, _ := New(log.New(), common.Hash{}, NewDatabase(db))

	// Update it with some accounts
	for i := byte(0); i < 255; i++ {
		addr := common.BytesToAddress([]byte{i})
		state.AddBalance(addr, big.NewInt(int64(11*i)))
		state.SetNonce(addr, uint64(42*i))
		if i%2 == 0 {
			state.SetState(addr, common.BytesToHash([]byte{i, i, i}), common.BytesToHash([]byte{i, i, i, i}))
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i})
		}
		state.IntermediateRoot(false)
	}
	// Ensure that no data was leaked into the database

	iteractor := db.NewIterator()
	for iteractor.Next() {
		t.Errorf("State leaked into database: %x -> %x", iteractor.Key(), iteractor.Value())
	}
}

// Tests that no intermediate state of an object is stored into the database,
// only the one right before the commit.
func TestIntermediateLeaks(t *testing.T) {
	// Create two state databases, one transitioning to the final state, the other final from the beginning
	transDb := memorydb.New()
	finalDb := memorydb.New()
	transState, _ := New(log.New(), common.Hash{}, NewDatabase(transDb))
	finalState, _ := New(log.New(), common.Hash{}, NewDatabase(finalDb))

	modify := func(state *StateDB, addr common.Address, i, tweak byte) {
		state.AddBalance(addr, big.NewInt(int64(11*i)+int64(tweak)))
		state.SetNonce(addr, uint64(42*i+tweak))
		if i%2 == 0 {
			state.SetState(addr, common.Hash{i, i, i, 0}, common.Hash{})
			state.SetState(addr, common.Hash{i, i, i, tweak}, common.Hash{i, i, i, i, tweak})
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i, tweak})
		}
	}

	// Modify the transient state.
	for i := byte(0); i < 255; i++ {
		modify(transState, common.Address{byte(i)}, i, 0)
	}
	// Write modifications to trie.
	transState.IntermediateRoot(false)

	// Overwrite all the data with new values in the transient database.
	for i := byte(0); i < 255; i++ {
		modify(transState, common.Address{byte(i)}, i, 99)
		modify(finalState, common.Address{byte(i)}, i, 99)
	}

	// Commit and cross check the databases.
	if _, err := transState.Commit(false); err != nil {
		t.Fatalf("failed to commit transition state: %v", err)
	}
	if _, err := finalState.Commit(false); err != nil {
		t.Fatalf("failed to commit final state: %v", err)
	}

	iterator := finalDb.NewIterator()
	for iterator.Next() {
		if _, err := transDb.Get(iterator.Key()); err != nil {
			t.Errorf("entry missing from the transition database: %x -> %x", iterator.Key(), iterator.Value())
		}
	}

	iterator = transDb.NewIterator()
	for iterator.Next() {
		if _, err := finalDb.Get(iterator.Key()); err != nil {
			t.Errorf("extra entry in the transition database: %x -> %x", iterator.Key(), iterator.Value())
		}
	}
}

// TestCopy tests that copying a statedb object indeed makes the original and
// the copy independent of each other. This test is a regression test against
// https://github.com/ethereum/go-ethereum/pull/15549.
func TestCopy(t *testing.T) {
	// Create a random state test to copy and modify "independently"
	orig, _ := New(log.New(), common.Hash{}, NewDatabase(memorydb.New()))

	for i := byte(0); i < 255; i++ {
		obj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		obj.AddBalance(big.NewInt(int64(i)))
		orig.updateStateObject(obj)
	}
	orig.Finalise(false)

	// Copy the state, modify both in-memory
	copy := orig.Copy()

	for i := byte(0); i < 255; i++ {
		origObj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		copyObj := copy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))

		origObj.AddBalance(big.NewInt(2 * int64(i)))
		copyObj.AddBalance(big.NewInt(3 * int64(i)))

		orig.updateStateObject(origObj)
		copy.updateStateObject(copyObj)
	}
	// Finalise the changes on both concurrently
	done := make(chan struct{})
	go func() {
		orig.Finalise(true)
		close(done)
	}()
	copy.Finalise(true)
	<-done

	// Verify that the two states have been updated independently
	for i := byte(0); i < 255; i++ {
		origObj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		copyObj := copy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))

		if want := big.NewInt(3 * int64(i)); origObj.Balance().Cmp(want) != 0 {
			t.Errorf("orig obj %d: balance mismatch: have %v, want %v", i, origObj.Balance(), want)
		}
		if want := big.NewInt(4 * int64(i)); copyObj.Balance().Cmp(want) != 0 {
			t.Errorf("copy obj %d: balance mismatch: have %v, want %v", i, copyObj.Balance(), want)
		}
	}
}

// TestCopyOfCopy tests that modified objects are carried over to the copy, and the copy of the copy.
// See https://github.com/kardia/go-kardia/pull/15225#issuecomment-380191512
func TestCopyOfCopy(t *testing.T) {
	sdb, _ := New(log.New(), common.Hash{}, NewDatabase(memorydb.New()))
	addr := common.HexToAddress("aaaa")
	sdb.AddBalance(addr, big.NewInt(42))

	if got := sdb.Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("1st copy fail, expected 42, got %v", got)
	}
	if got := sdb.Copy().Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("2nd copy fail, expected 42, got %v", got)
	}
}
