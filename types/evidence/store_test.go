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
