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

// Package state provides a caching layer atop the Kardia state trie.
package state

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/trie"
	"github.com/kardiachain/go-kardiamain/types"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

// StateDBs within the Kardia protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	logger log.Logger

	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memorized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int

	lock sync.Mutex
}

// Create a new state from a given trie.
func New(logger log.Logger, root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		logger:            logger,
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}, nil
}

// Retrieve a state object or create a new state object if nil.
func (sdb *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = sdb.createObject(addr)
	}
	return stateObject
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the KVM emits new state logs.
func (sdb *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	sdb.thash = thash
	sdb.bhash = bhash
	sdb.txIndex = ti
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the KVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Kai doesn't disappear.
func (sdb *StateDB) CreateAccount(addr common.Address) {
	newState, prev := sdb.createObject(addr)
	if prev != nil {
		newState.setBalance(prev.data.Balance)
	}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (sdb *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = sdb.getStateObject(addr)
	newobj = newObject(sdb, addr, Account{})
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		sdb.journal.append(createObjectChange{account: &addr})
	} else {
		sdb.journal.append(resetObjectChange{prev: prev})
	}
	sdb.setStateObject(newobj)
	return newobj, prev
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (sdb *StateDB) Copy() *StateDB {
	sdb.lock.Lock()
	defer sdb.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                sdb.db,
		trie:              sdb.db.CopyTrie(sdb.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(sdb.journal.dirties)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(sdb.journal.dirties)),
		refund:            sdb.refund,
		logs:              make(map[common.Hash][]*types.Log, len(sdb.logs)),
		logSize:           sdb.logSize,
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}

	// Copy the dirty states, logs, and preimages
	for addr := range sdb.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := sdb.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}

	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range sdb.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = sdb.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}

	for hash, logs := range sdb.logs {
		state.logs[hash] = make([]*types.Log, len(logs))
		copy(state.logs[hash], logs)
	}
	for hash, preimage := range sdb.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (sdb *StateDB) Empty(addr common.Address) bool {
	so := sdb.getStateObject(addr)
	return so == nil || so.empty()
}

// Database retrieves the low level database supporting the lower level trie ops.
func (sdb *StateDB) Database() Database {
	return sdb.db
}

func (sdb *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return sdb.logs[hash]
}

func (sdb *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce() + 1
	}

	return 0
}

// AddBalance adds amount to the account associated with addr.
func (sdb *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// Retrieve the balance from the given address or 0 if object not found
func (sdb *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	sdb.logger.Error("StateDB addr not found", "addr", addr)
	return common.Big0
}

// SubBalance subtracts amount from the account associated with addr.
func (sdb *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (sdb *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (sdb *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (sdb *StateDB) SetState(addr common.Address, key, value common.Hash) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(sdb.db, key, value)
	}
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (sdb *StateDB) Reset(root common.Hash) error {
	tr, err := sdb.db.OpenTrie(root)
	if err != nil {
		return err
	}
	sdb.trie = tr
	sdb.stateObjects = make(map[common.Address]*stateObject)
	sdb.stateObjectsDirty = make(map[common.Address]struct{})
	sdb.thash = common.Hash{}
	sdb.bhash = common.Hash{}
	sdb.txIndex = 0
	sdb.logs = make(map[common.Hash][]*types.Log)
	sdb.logSize = 0
	sdb.preimages = make(map[common.Hash][]byte)
	sdb.clearJournalAndRefund()
	return nil
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (sdb *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	sdb.Finalise(deleteEmptyObjects)
	return sdb.trie.Hash()
}

// Finalise finalises the state by removing the sdb destructed objects
// and clears the journal as well as the refunds.
func (sdb *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range sdb.journal.dirties {
		stateObject, exist := sdb.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			sdb.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(sdb.db)
			sdb.updateStateObject(stateObject)
		}
		sdb.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	sdb.clearJournalAndRefund()
}

//
// Setting, updating & deleting state object methods.
//

// setError remembers the first non-nil error it is called with.
func (sdb *StateDB) setError(err error) {
	if sdb.dbErr == nil {
		sdb.dbErr = err
	}
}

func (sdb *StateDB) Error() error {
	return sdb.dbErr
}

// Retrieve a state object given by the address. Returns nil if not found.
func (sdb *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := sdb.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := sdb.trie.TryGet(addr[:])
	if len(enc) == 0 {
		sdb.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		sdb.logger.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(sdb, addr, data)
	sdb.setStateObject(obj)
	return obj
}

func (sdb *StateDB) setStateObject(object *stateObject) {
	sdb.lock.Lock()
	sdb.stateObjects[object.Address()] = object
	sdb.lock.Unlock()
}

// deleteStateObject removes the given object from the state trie.
func (sdb *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	sdb.setError(sdb.trie.TryDelete(addr[:]))
}

// updateStateObject writes the given object to the trie.
func (sdb *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	sdb.setError(sdb.trie.TryUpdate(addr[:], data))
}

// GetRefund returns the current value of the refund counter.
func (sdb *StateDB) GetRefund() uint64 {
	return sdb.refund
}

func (sdb *StateDB) clearJournalAndRefund() {
	sdb.journal = newJournal()
	sdb.validRevisions = sdb.validRevisions[:0]
	sdb.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (sdb *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer sdb.clearJournalAndRefund()

	for addr := range sdb.journal.dirties {
		sdb.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range sdb.stateObjects {
		_, isDirty := sdb.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			sdb.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				sdb.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(sdb.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			sdb.updateStateObject(stateObject)
		}
		delete(sdb.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = sdb.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			sdb.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			sdb.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	sdb.logger.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}

func (sdb *StateDB) AddLog(log *types.Log) {
	sdb.journal.append(addLogChange{txhash: sdb.thash})

	log.TxHash = sdb.thash
	log.BlockHash = sdb.bhash
	log.TxIndex = uint(sdb.txIndex)
	log.Index = sdb.logSize
	sdb.logs[sdb.thash] = append(sdb.logs[sdb.thash], log)
	sdb.logSize++
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (self *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := self.preimages[hash]; !ok {
		self.journal.append(addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		self.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (self *StateDB) Preimages() map[common.Hash][]byte {
	return self.preimages
}

func (sdb *StateDB) AddRefund(gas uint64) {
	sdb.journal.append(refundChange{prev: sdb.refund})
	sdb.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (self *StateDB) SubRefund(gas uint64) {
	self.journal.append(refundChange{prev: self.refund})
	if gas > self.refund {
		panic("Refund counter below zero")
	}
	self.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (sdb *StateDB) Exist(addr common.Address) bool {
	return sdb.getStateObject(addr) != nil
}

func (sdb *StateDB) GetCode(addr common.Address) []byte {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(sdb.db)
	}
	return nil
}

func (sdb *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

func (sdb *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := sdb.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		sdb.setError(err)
	}
	return size
}

func (sdb *StateDB) GetState(addr common.Address, bhash common.Hash) common.Hash {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(sdb.db, bhash)
	}
	return common.Hash{}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (sdb *StateDB) Suicide(addr common.Address) bool {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	sdb.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

func (sdb *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (sdb *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(sdb.validRevisions), func(i int) bool {
		return sdb.validRevisions[i].id >= revid
	})
	if idx == len(sdb.validRevisions) || sdb.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := sdb.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	sdb.journal.revert(sdb, snapshot)
}

// Snapshot returns an identifier for the current revision of the state.
func (sdb *StateDB) Snapshot() int {
	id := sdb.nextRevisionId
	sdb.nextRevisionId++
	sdb.validRevisions = append(sdb.validRevisions, revision{id, sdb.journal.length()})
	return id
}
