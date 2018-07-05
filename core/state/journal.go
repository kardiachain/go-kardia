package state

import (
	//"math/big"

	"conceptchain/common"
)

// journalEntry is a modification entry in the state change journal that can be
// reverted on demand.
type journalEntry interface {
	// revert undoes the changes introduced by this journal entry.
	revert(*StateDB)

	// dirtied returns the Ethereum address modified by this journal entry.
	dirtied() *common.Address
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in case of an execution
// exception or revertal request.
type journal struct {
	entries []journalEntry         // Current changes tracked by the journal
	dirties map[common.Address]int // Dirty accounts and the number of changes
}

// newJournal create a new initialized journal.
func newJournal() *journal {
	return &journal{
		dirties: make(map[common.Address]int),
	}
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry journalEntry) {
	j.entries = append(j.entries, entry)
	if addr := entry.dirtied(); addr != nil {
		j.dirties[*addr]++
	}
}

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *common.Address
	}
	resetObjectChange struct {
		prev *stateObject
	}

	// Changes to individual account.
	nonceChange struct {
		account *common.Address
		prev    uint64
	}
)

func (ch createObjectChange) revert(s *StateDB) {
	delete(s.stateObjects, *ch.account)
	delete(s.stateObjectsDirty, *ch.account)
}

func (ch createObjectChange) dirtied() *common.Address {
	return ch.account
}

func (ch resetObjectChange) revert(s *StateDB) {
	s.setStateObject(ch.prev)
}

func (ch resetObjectChange) dirtied() *common.Address {
	return nil
}

func (ch nonceChange) revert(s *StateDB) {
	s.getStateObject(*ch.account).setNonce(ch.prev)
}

func (ch nonceChange) dirtied() *common.Address {
	return ch.account
}
