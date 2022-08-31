/*
   Copyright 2022 Erigon contributors
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package memdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
)

func initializeDbNonDupSort(rwTx kvstore.RwTx) {
	rwTx.Put(kvstore.HashedAccounts, []byte("AAAA"), []byte("value"))
	rwTx.Put(kvstore.HashedAccounts, []byte("CAAA"), []byte("value1"))
	rwTx.Put(kvstore.HashedAccounts, []byte("CBAA"), []byte("value2"))
	rwTx.Put(kvstore.HashedAccounts, []byte("CCAA"), []byte("value3"))
}

func TestPutAppendHas(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.Nil(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	require.NoError(t, batch.Append(kvstore.HashedAccounts, []byte("AAAA"), []byte("value1.5")))
	require.Error(t, batch.Append(kvstore.HashedAccounts, []byte("AAAA"), []byte("value1.3")))
	require.NoError(t, batch.Put(kvstore.HashedAccounts, []byte("AAAA"), []byte("value1.3")))
	require.NoError(t, batch.Append(kvstore.HashedAccounts, []byte("CBAA"), []byte("value3.5")))
	require.Error(t, batch.Append(kvstore.HashedAccounts, []byte("CBAA"), []byte("value3.1")))
	require.NoError(t, batch.AppendDup(kvstore.HashedAccounts, []byte("CBAA"), []byte("value3.1")))

	require.Nil(t, batch.Flush(rwTx))

	exist, err := batch.Has(kvstore.HashedAccounts, []byte("AAAA"))
	require.Nil(t, err)
	require.Equal(t, exist, true)

	val, err := batch.GetOne(kvstore.HashedAccounts, []byte("AAAA"))
	require.Nil(t, err)
	require.Equal(t, val, []byte("value1.3"))

	exist, err = batch.Has(kvstore.HashedAccounts, []byte("KKKK"))
	require.Nil(t, err)
	require.Equal(t, exist, false)
}

func TestLastMiningDB(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	batch.Put(kvstore.HashedAccounts, []byte("BAAA"), []byte("value4"))
	batch.Put(kvstore.HashedAccounts, []byte("BCAA"), []byte("value5"))

	cursor, err := batch.Cursor(kvstore.HashedAccounts)
	require.NoError(t, err)

	key, value, err := cursor.Last()
	require.NoError(t, err)

	require.Equal(t, key, []byte("CCAA"))
	require.Equal(t, value, []byte("value3"))

	key, value, err = cursor.Next()
	require.NoError(t, err)
	require.Equal(t, key, []byte(nil))
	require.Equal(t, value, []byte(nil))
}

func TestLastMiningMem(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	batch.Put(kvstore.HashedAccounts, []byte("BAAA"), []byte("value4"))
	batch.Put(kvstore.HashedAccounts, []byte("DCAA"), []byte("value5"))

	cursor, err := batch.Cursor(kvstore.HashedAccounts)
	require.NoError(t, err)

	key, value, err := cursor.Last()
	require.NoError(t, err)

	require.Equal(t, key, []byte("DCAA"))
	require.Equal(t, value, []byte("value5"))

	key, value, err = cursor.Next()
	require.NoError(t, err)
	require.Equal(t, key, []byte(nil))
	require.Equal(t, value, []byte(nil))
}

func TestDeleteMining(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)
	batch := NewMemoryBatch(rwTx)
	batch.Put(kvstore.HashedAccounts, []byte("BAAA"), []byte("value4"))
	batch.Put(kvstore.HashedAccounts, []byte("DCAA"), []byte("value5"))
	batch.Put(kvstore.HashedAccounts, []byte("FCAA"), []byte("value5"))

	batch.Delete(kvstore.HashedAccounts, []byte("BAAA"))
	batch.Delete(kvstore.HashedAccounts, []byte("CBAA"))

	cursor, err := batch.Cursor(kvstore.HashedAccounts)
	require.NoError(t, err)

	key, value, err := cursor.SeekExact([]byte("BAAA"))
	require.NoError(t, err)
	require.Equal(t, key, []byte(nil))
	require.Equal(t, value, []byte(nil))

	key, value, err = cursor.SeekExact([]byte("CBAA"))
	require.NoError(t, err)
	require.Equal(t, key, []byte(nil))
	require.Equal(t, value, []byte(nil))
}

func TestFlush(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)
	batch := NewMemoryBatch(rwTx)
	batch.Put(kvstore.HashedAccounts, []byte("BAAA"), []byte("value4"))
	batch.Put(kvstore.HashedAccounts, []byte("AAAA"), []byte("value5"))
	batch.Put(kvstore.HashedAccounts, []byte("FCAA"), []byte("value5"))

	require.NoError(t, batch.Flush(rwTx))

	value, err := rwTx.GetOne(kvstore.HashedAccounts, []byte("BAAA"))
	require.NoError(t, err)
	require.Equal(t, value, []byte("value4"))

	value, err = rwTx.GetOne(kvstore.HashedAccounts, []byte("AAAA"))
	require.NoError(t, err)
	require.Equal(t, value, []byte("value5"))
}

func TestForEach(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	batch.Put(kvstore.HashedAccounts, []byte("FCAA"), []byte("value5"))
	require.NoError(t, batch.Flush(rwTx))

	var keys []string
	var values []string
	err = batch.ForEach(kvstore.HashedAccounts, []byte("XYAZ"), func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Nil(t, keys)
	require.Nil(t, values)

	err = batch.ForEach(kvstore.HashedAccounts, []byte("CC"), func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Equal(t, []string{"CCAA", "FCAA"}, keys)
	require.Equal(t, []string{"value3", "value5"}, values)

	var keys1 []string
	var values1 []string

	err = batch.ForEach(kvstore.HashedAccounts, []byte("A"), func(k, v []byte) error {
		keys1 = append(keys1, string(k))
		values1 = append(values1, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Equal(t, []string{"AAAA", "CAAA", "CBAA", "CCAA", "FCAA"}, keys1)
	require.Equal(t, []string{"value", "value1", "value2", "value3", "value5"}, values1)
}

func TestForPrefix(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	var keys1 []string
	var values1 []string

	err = batch.ForPrefix(kvstore.HashedAccounts, []byte("AB"), func(k, v []byte) error {
		keys1 = append(keys1, string(k))
		values1 = append(values1, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Nil(t, keys1)
	require.Nil(t, values1)

	err = batch.ForPrefix(kvstore.HashedAccounts, []byte("AAAA"), func(k, v []byte) error {
		keys1 = append(keys1, string(k))
		values1 = append(values1, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Equal(t, []string{"AAAA"}, keys1)
	require.Equal(t, []string{"value"}, values1)

	var keys []string
	var values []string
	err = batch.ForPrefix(kvstore.HashedAccounts, []byte("C"), func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})
	require.Nil(t, err)
	require.Equal(t, []string{"CAAA", "CBAA", "CCAA"}, keys)
	require.Equal(t, []string{"value1", "value2", "value3"}, values)
}

func TestForAmount(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	var keys []string
	var values []string
	err = batch.ForAmount(kvstore.HashedAccounts, []byte("C"), uint32(3), func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})

	require.Nil(t, err)
	require.Equal(t, []string{"CAAA", "CBAA", "CCAA"}, keys)
	require.Equal(t, []string{"value1", "value2", "value3"}, values)

	var keys1 []string
	var values1 []string
	err = batch.ForAmount(kvstore.HashedAccounts, []byte("C"), uint32(10), func(k, v []byte) error {
		keys1 = append(keys1, string(k))
		values1 = append(values1, string(v))
		return nil
	})

	require.Nil(t, err)
	require.Equal(t, []string{"CAAA", "CBAA", "CCAA"}, keys1)
	require.Equal(t, []string{"value1", "value2", "value3"}, values1)
}

func TestGetOneAfterClearBucket(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	err = batch.ClearBucket(kvstore.HashedAccounts)
	require.Nil(t, err)

	cond := batch.isTableCleared(kvstore.HashedAccounts)
	require.True(t, cond)

	val, err := batch.GetOne(kvstore.HashedAccounts, []byte("A"))
	require.Nil(t, err)
	require.Nil(t, val)

	val, err = batch.GetOne(kvstore.HashedAccounts, []byte("AAAA"))
	require.Nil(t, err)
	require.Nil(t, val)
}

func TestSeekExactAfterClearBucket(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	err = batch.ClearBucket(kvstore.HashedAccounts)
	require.Nil(t, err)

	cond := batch.isTableCleared(kvstore.HashedAccounts)
	require.True(t, cond)

	cursor, err := batch.RwCursor(kvstore.HashedAccounts)
	require.NoError(t, err)

	key, val, err := cursor.SeekExact([]byte("AAAA"))
	require.Nil(t, err)
	assert.Nil(t, key)
	assert.Nil(t, val)

	err = cursor.Put([]byte("AAAA"), []byte("valueX"))
	require.Nil(t, err)

	key, val, err = cursor.SeekExact([]byte("AAAA"))
	require.Nil(t, err)
	assert.Equal(t, []byte("AAAA"), key)
	assert.Equal(t, []byte("valueX"), val)

	key, val, err = cursor.SeekExact([]byte("BBBB"))
	require.Nil(t, err)
	assert.Nil(t, key)
	assert.Nil(t, val)
}

func TestFirstAfterClearBucket(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	err = batch.ClearBucket(kvstore.HashedAccounts)
	require.Nil(t, err)

	err = batch.Put(kvstore.HashedAccounts, []byte("BBBB"), []byte("value5"))
	require.Nil(t, err)

	cursor, err := batch.Cursor(kvstore.HashedAccounts)
	require.NoError(t, err)

	key, val, err := cursor.First()
	require.Nil(t, err)
	assert.Equal(t, []byte("BBBB"), key)
	assert.Equal(t, []byte("value5"), val)

	key, val, err = cursor.Next()
	require.Nil(t, err)
	assert.Nil(t, key)
	assert.Nil(t, val)
}

func TestIncReadSequence(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbNonDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	_, err = batch.IncrementSequence(kvstore.HashedAccounts, uint64(12))
	require.Nil(t, err)

	val, err := batch.ReadSequence(kvstore.HashedAccounts)
	require.Nil(t, err)
	require.Equal(t, val, uint64(12))
}

func initializeDbDupSort(rwTx kvstore.RwTx) {
	rwTx.Put(kvstore.AccountChangeSet, []byte("key1"), []byte("value1.1"))
	rwTx.Put(kvstore.AccountChangeSet, []byte("key3"), []byte("value3.1"))
	rwTx.Put(kvstore.AccountChangeSet, []byte("key1"), []byte("value1.3"))
	rwTx.Put(kvstore.AccountChangeSet, []byte("key3"), []byte("value3.3"))
}

func TestNext(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	batch.Put(kvstore.AccountChangeSet, []byte("key1"), []byte("value1.2"))

	cursor, err := batch.CursorDupSort(kvstore.AccountChangeSet)
	require.NoError(t, err)

	k, v, err := cursor.First()
	require.Nil(t, err)
	assert.Equal(t, []byte("key1"), k)
	assert.Equal(t, []byte("value1.1"), v)

	k, v, err = cursor.Next()
	require.Nil(t, err)
	assert.Equal(t, []byte("key1"), k)
	assert.Equal(t, []byte("value1.2"), v)

	k, v, err = cursor.Next()
	require.Nil(t, err)
	assert.Equal(t, []byte("key1"), k)
	assert.Equal(t, []byte("value1.3"), v)

	k, v, err = cursor.Next()
	require.Nil(t, err)
	assert.Equal(t, []byte("key3"), k)
	assert.Equal(t, []byte("value3.1"), v)

	k, v, err = cursor.Next()
	require.Nil(t, err)
	assert.Equal(t, []byte("key3"), k)
	assert.Equal(t, []byte("value3.3"), v)

	k, v, err = cursor.Next()
	require.Nil(t, err)
	assert.Nil(t, k)
	assert.Nil(t, v)
}

func TestNextNoDup(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	batch.Put(kvstore.AccountChangeSet, []byte("key2"), []byte("value2.1"))
	batch.Put(kvstore.AccountChangeSet, []byte("key2"), []byte("value2.2"))

	cursor, err := batch.CursorDupSort(kvstore.AccountChangeSet)
	require.NoError(t, err)

	k, _, err := cursor.First()
	require.Nil(t, err)
	assert.Equal(t, []byte("key1"), k)

	k, _, err = cursor.NextNoDup()
	require.Nil(t, err)
	assert.Equal(t, []byte("key2"), k)

	k, _, err = cursor.NextNoDup()
	require.Nil(t, err)
	assert.Equal(t, []byte("key3"), k)
}

func TestDeleteCurrentDuplicates(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbDupSort(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	cursor, err := batch.RwCursorDupSort(kvstore.AccountChangeSet)
	require.NoError(t, err)

	require.NoError(t, cursor.Put([]byte("key3"), []byte("value3.2")))

	key, _, err := cursor.SeekExact([]byte("key3"))
	require.NoError(t, err)
	require.Equal(t, []byte("key3"), key)

	require.NoError(t, cursor.DeleteCurrentDuplicates())

	require.NoError(t, batch.Flush(rwTx))

	var keys []string
	var values []string
	err = rwTx.ForEach(kvstore.AccountChangeSet, nil, func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, []string{"key1", "key1"}, keys)
	require.Equal(t, []string{"value1.1", "value1.3"}, values)
}

func TestSeekBothRange(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	rwTx.Put(kvstore.AccountChangeSet, []byte("key1"), []byte("value1.1"))
	rwTx.Put(kvstore.AccountChangeSet, []byte("key3"), []byte("value3.3"))

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	cursor, err := batch.RwCursorDupSort(kvstore.AccountChangeSet)
	require.NoError(t, err)

	require.NoError(t, cursor.Put([]byte("key3"), []byte("value3.1")))
	require.NoError(t, cursor.Put([]byte("key1"), []byte("value1.3")))

	v, err := cursor.SeekBothRange([]byte("key2"), []byte("value1.2"))
	require.NoError(t, err)
	// SeekBothRange does exact match of the key, but range match of the value, so we get nil here
	require.Nil(t, v)

	v, err = cursor.SeekBothRange([]byte("key3"), []byte("value3.2"))
	require.NoError(t, err)
	require.Equal(t, "value3.3", string(v))
}

func initializeDbAutoConversion(rwTx kvstore.RwTx) {
	rwTx.Put(kvstore.PlainState, []byte("A"), []byte("0"))
	rwTx.Put(kvstore.PlainState, []byte("A..........................._______________________________A"), []byte("1"))
	rwTx.Put(kvstore.PlainState, []byte("A..........................._______________________________C"), []byte("2"))
	rwTx.Put(kvstore.PlainState, []byte("B"), []byte("8"))
	rwTx.Put(kvstore.PlainState, []byte("C"), []byte("9"))
	rwTx.Put(kvstore.PlainState, []byte("D..........................._______________________________A"), []byte("3"))
	rwTx.Put(kvstore.PlainState, []byte("D..........................._______________________________C"), []byte("4"))
}

func TestAutoConversion(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbAutoConversion(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	c, err := batch.RwCursor(kvstore.PlainState)
	require.NoError(t, err)

	// key length conflict
	require.Error(t, c.Put([]byte("A..........................."), []byte("?")))

	require.NoError(t, c.Delete([]byte("A..........................._______________________________A")))
	require.NoError(t, c.Put([]byte("B"), []byte("7")))
	require.NoError(t, c.Delete([]byte("C")))
	require.NoError(t, c.Put([]byte("D..........................._______________________________C"), []byte("6")))
	require.NoError(t, c.Put([]byte("D..........................._______________________________E"), []byte("5")))

	k, v, err := c.First()
	require.NoError(t, err)
	assert.Equal(t, []byte("A"), k)
	assert.Equal(t, []byte("0"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("A..........................._______________________________C"), k)
	assert.Equal(t, []byte("2"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("B"), k)
	assert.Equal(t, []byte("7"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("D..........................._______________________________A"), k)
	assert.Equal(t, []byte("3"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("D..........................._______________________________C"), k)
	assert.Equal(t, []byte("6"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("D..........................._______________________________E"), k)
	assert.Equal(t, []byte("5"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Nil(t, k)
	assert.Nil(t, v)
}

func TestAutoConversionDelete(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbAutoConversion(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	c, err := batch.RwCursor(kvstore.PlainState)
	require.NoError(t, err)

	require.NoError(t, c.Delete([]byte("A..........................._______________________________A")))
	require.NoError(t, c.Delete([]byte("A..........................._______________________________C")))
	require.NoError(t, c.Delete([]byte("B")))
	require.NoError(t, c.Delete([]byte("C")))

	k, v, err := c.First()
	require.NoError(t, err)
	assert.Equal(t, []byte("A"), k)
	assert.Equal(t, []byte("0"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("D..........................._______________________________A"), k)
	assert.Equal(t, []byte("3"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Equal(t, []byte("D..........................._______________________________C"), k)
	assert.Equal(t, []byte("4"), v)

	k, v, err = c.Next()
	require.NoError(t, err)
	assert.Nil(t, k)
	assert.Nil(t, v)
}

func TestAutoConversionSeekBothRange(t *testing.T) {
	rwTx, err := New().BeginRw(context.Background())
	require.NoError(t, err)

	initializeDbAutoConversion(rwTx)

	batch := NewMemoryBatch(rwTx)
	defer batch.Close()

	c, err := batch.RwCursorDupSort(kvstore.PlainState)
	require.NoError(t, err)

	require.NoError(t, c.Delete([]byte("A..........................._______________________________A")))
	require.NoError(t, c.Put([]byte("D..........................._______________________________C"), []byte("6")))
	require.NoError(t, c.Put([]byte("D..........................._______________________________E"), []byte("5")))

	v, err := c.SeekBothRange([]byte("A..........................."), []byte("_______________________________A"))
	require.NoError(t, err)
	assert.Equal(t, []byte("_______________________________C2"), v)

	_, v, err = c.NextDup()
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = c.SeekBothRange([]byte("A..........................."), []byte("_______________________________X"))
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = c.SeekBothRange([]byte("B..........................."), []byte(""))
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = c.SeekBothRange([]byte("C..........................."), []byte(""))
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = c.SeekBothRange([]byte("D..........................."), []byte(""))
	require.NoError(t, err)
	assert.Equal(t, []byte("_______________________________A3"), v)

	_, v, err = c.NextDup()
	require.NoError(t, err)
	assert.Equal(t, []byte("_______________________________C6"), v)

	_, v, err = c.NextDup()
	require.NoError(t, err)
	assert.Equal(t, []byte("_______________________________E5"), v)

	_, v, err = c.NextDup()
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = c.SeekBothRange([]byte("X..........................."), []byte("_______________________________Y"))
	require.NoError(t, err)
	assert.Nil(t, v)
}
