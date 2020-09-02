package evidence

import (
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreAddDuplicate(t *testing.T) {
	assert := assert.New(t)

	db := memorydb.New()
	store := NewStore(db)

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, common.BytesToAddress([]byte("addr1")))

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.True(added)

	// cant add twice
	added, err = store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.False(added)
}

func TestStoreCommitDuplicate(t *testing.T) {
	assert := assert.New(t)

	db := memorydb.New()
	store := NewStore(db)

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, common.BytesToAddress([]byte("addr1")))

	store.MarkEvidenceAsCommitted(ev)

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.False(added)
}

func TestStoreMark(t *testing.T) {
	assert := assert.New(t)

	db := memorydb.New()
	store := NewStore(db)

	// before we do anything, priority/pending are empty
	priorityEv := store.PriorityEvidence()
	pendingEv := store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, common.BytesToAddress([]byte("val1")))
	evb, _ := types.EvidenceToBytes(ev)

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.True(added)

	// get the evidence. verify. should be uncommitted
	ei := store.GetInfo(int64(ev.Height()), ev.Hash().Bytes())
	assert.Equal(evb, ei.Evidence)
	assert.Equal(uint64(priority), ei.Priority)
	assert.False(ei.Committed)

	// new evidence should be returns in priority/pending
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(1, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority is now empty
	store.MarkEvidenceAsBroadcasted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority and pending are now empty
	store.MarkEvidenceAsCommitted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	// evidence should show committed
	newPriority := int64(0)
	ei = store.GetInfo(int64(ev.Height()), ev.Hash().Bytes())
	evb, _ = types.EvidenceToBytes(ev)
	assert.Equal(evb, ei.Evidence)
	assert.Equal(uint64(newPriority), ei.Priority)
	assert.True(ei.Committed)
}

func TestStorePriority(t *testing.T) {
	assert := assert.New(t)

	db := memorydb.New()
	store := NewStore(db)

	// sorted by priority and then height
	cases := []struct {
		ev       types.MockEvidence
		priority int64
	}{
		{types.NewMockEvidence(2, time.Now().UTC(), 1, common.BytesToAddress([]byte("val1"))), 17},
		{types.NewMockEvidence(5, time.Now().UTC(), 2, common.BytesToAddress([]byte("val2"))), 15},
		{types.NewMockEvidence(10, time.Now().UTC(), 2, common.BytesToAddress([]byte("val2"))), 13},
		{types.NewMockEvidence(100, time.Now().UTC(), 2, common.BytesToAddress([]byte("val2"))), 11},
		{types.NewMockEvidence(90, time.Now().UTC(), 2, common.BytesToAddress([]byte("val2"))), 11},
		{types.NewMockEvidence(80, time.Now().UTC(), 2, common.BytesToAddress([]byte("val2"))), 11},
	}

	for _, c := range cases {
		added, err := store.AddNewEvidence(c.ev, c.priority)
		require.NoError(t, err)
		assert.True(added)
	}

	evList := store.PriorityEvidence()
	for i, ev := range evList {
		assert.Equal(ev, cases[i].ev)
	}
}
