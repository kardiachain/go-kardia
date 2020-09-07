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

package evidence

import (
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/rlp"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"

	"github.com/kardiachain/go-kardiamain/types"
)

const (
	baseKeyLookup   = "evidence-lookup"   // all evidence
	baseKeyOutqueue = "evidence-outqueue" // not-yet broadcast
	baseKeyPending  = "evidence-pending"  // broadcast but not committed
)

// Info ...
type Info struct {
	Committed bool
	Priority  uint64
	Evidence  []byte
}

func keyLookup(evidence types.Evidence) []byte {
	return keyLookupFromHeightAndHash(int64(evidence.Height()), evidence.Hash().Bytes())
}

// big endian padded hex
func bE(h int64) string {
	return fmt.Sprintf("%0.16X", h)
}

func keyLookupFromHeightAndHash(height int64, hash []byte) []byte {
	return _key("%s/%s/%X", baseKeyLookup, bE(height), hash)
}

func keyOutqueue(evidence types.Evidence, priority int64) []byte {
	return _key("%s/%s/%s/%X", baseKeyOutqueue, bE(priority), bE(int64(evidence.Height())), evidence.Hash())
}

func keyPending(evidence types.Evidence) []byte {
	return _key("%s/%s/%X", baseKeyPending, bE(int64(evidence.Height())), evidence.Hash())
}

func _key(format string, o ...interface{}) []byte {
	return []byte(fmt.Sprintf(format, o...))
}

// Store is a store of all the evidence we've seen, including
// evidence that has been committed, evidence that has been verified but not broadcast,
// and evidence that has been broadcast but not yet committed.
type Store struct {
	db kaidb.Database
}

// NewStore ...
func NewStore(db kaidb.Database) *Store {
	return &Store{
		db: db,
	}
}

// PriorityEvidence returns the evidence from the outqueue, sorted by highest priority.
func (store *Store) PriorityEvidence() (evidence []types.Evidence) {
	// reverse the order so highest priority is first
	l := store.listEvidence(baseKeyOutqueue, -1)
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}

	return l
}

// PendingEvidence returns up to maxNum known, uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (store *Store) PendingEvidence(maxNum int64) (evidence []types.Evidence) {
	return store.listEvidence(baseKeyPending, maxNum)
}

// Has checks if the evidence is already stored
func (store *Store) Has(evidence types.Evidence) bool {
	key := keyLookup(evidence)
	ok, _ := store.db.Has(key)
	return ok
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// It is wrapped by PriorityEvidence and PendingEvidence for convenience.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (store *Store) listEvidence(prefixKey string, maxNum int64) (evidence []types.Evidence) {
	var count int64
	iter := store.db.NewIteratorWithPrefix([]byte(prefixKey))
	for iter.Next() {
		val := iter.Value()

		if count == maxNum {
			return evidence
		}
		count++

		var ei Info
		err := rlp.DecodeBytes(val, &ei)
		if err != nil {
			panic(err)
		}

		ev, err := types.EvidenceFromBytes(ei.Evidence)
		if err != nil {
			panic(err)
		}
		evidence = append(evidence, ev)
	}
	return evidence
}

// AddNewEvidence adds the given evidence to the database.
// It returns false if the evidence is already stored.
func (store *Store) AddNewEvidence(evidence types.Evidence, priority int64) (bool, error) {
	// check if we already have seen it
	if store.Has(evidence) {
		return false, nil
	}

	evb, err := types.EvidenceToBytes(evidence)
	if err != nil {
		return false, err
	}

	ei := Info{
		Committed: false,
		Priority:  uint64(priority),
		Evidence:  evb,
	}
	eiBytes, err := rlp.EncodeToBytes(ei)
	if err != nil {
		return false, err
	}

	// add it to the store
	key := keyOutqueue(evidence, priority)
	if err = store.db.Put(key, eiBytes); err != nil {
		return false, err
	}

	key = keyPending(evidence)
	if err = store.db.Put(key, eiBytes); err != nil {
		return false, err
	}

	key = keyLookup(evidence)
	if err = store.db.Put(key, eiBytes); err != nil {
		return false, err
	}

	return true, nil
}

// GetInfo fetches the Info with the given height and hash.
// If not found, ei.Evidence is nil.
func (store *Store) GetInfo(height int64, hash []byte) Info {
	key := keyLookupFromHeightAndHash(height, hash)
	val, _ := store.db.Get(key)
	if len(val) == 0 {
		return Info{}
	}
	var ei Info
	err := rlp.DecodeBytes(val, &ei)
	if err != nil {
		panic(err)
	}
	return ei
}

// MarkEvidenceAsBroadcasted removes evidence from Outqueue.
func (store *Store) MarkEvidenceAsBroadcasted(evidence types.Evidence) {
	ei := store.getInfo(evidence)
	if ei.Evidence == nil {
		// nothing to do; we did not store the evidence yet (AddNewEvidence):
		return
	}
	// remove from the outqueue
	key := keyOutqueue(evidence, int64(ei.Priority))
	store.db.Delete(key)
}

// MarkEvidenceAsCommitted removes evidence from pending and outqueue and sets the state to committed.
func (store *Store) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	pendingKey := keyPending(evidence)
	store.db.Delete(pendingKey)

	evb, err := types.EvidenceToBytes(evidence)
	if err != nil {
		panic(err)
	}

	// committed Info doens't need priority
	ei := Info{
		Committed: true,
		Evidence:  evb,
		Priority:  0,
	}

	lookupKey := keyLookup(evidence)
	b, err := rlp.EncodeToBytes(ei)

	store.db.Put(lookupKey, b)
}

//---------------------------------------------------
// utils

// getInfo is convenience for calling GetInfo if we have the full evidence.
func (store *Store) getInfo(evidence types.Evidence) Info {
	return store.GetInfo(int64(evidence.Height()), evidence.Hash().Bytes())
}
