/*
   Copyright 2021 Erigon contributors

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

	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore/mdbx"
	"github.com/kardiachain/go-kardia/lib/log"
)

func New() kvstore.RwDB {
	return mdbx.NewMDBX(log.New()).InMem().MustOpen()
}

func NewPoolDB() kvstore.RwDB {
	return mdbx.NewMDBX(log.New()).InMem().Label(kvstore.TxPoolDB).WithTableCfg(func(_ kvstore.TableCfg) kvstore.TableCfg { return kvstore.TxpoolTablesCfg }).MustOpen()
}
func NewDownloaderDB() kvstore.RwDB {
	return mdbx.NewMDBX(log.New()).InMem().Label(kvstore.DownloaderDB).WithTableCfg(func(_ kvstore.TableCfg) kvstore.TableCfg { return kvstore.DownloaderTablesCfg }).MustOpen()
}
func NewSentryDB() kvstore.RwDB {
	return mdbx.NewMDBX(log.New()).InMem().Label(kvstore.SentryDB).WithTableCfg(func(_ kvstore.TableCfg) kvstore.TableCfg { return kvstore.SentryTablesCfg }).MustOpen()
}

func NewTestDB(tb testing.TB) kvstore.RwDB {
	tb.Helper()
	db := New()
	tb.Cleanup(db.Close)
	return db
}

func NewTestPoolDB(tb testing.TB) kvstore.RwDB {
	tb.Helper()
	db := NewPoolDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestDownloaderDB(tb testing.TB) kvstore.RwDB {
	tb.Helper()
	db := NewDownloaderDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestSentrylDB(tb testing.TB) kvstore.RwDB {
	tb.Helper()
	db := NewPoolDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestTx(tb testing.TB) (kvstore.RwDB, kvstore.RwTx) {
	tb.Helper()
	db := New()
	tb.Cleanup(db.Close)
	tx, err := db.BeginRw(context.Background())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(tx.Rollback)
	return db, tx
}

func NewTestPoolTx(tb testing.TB) (kvstore.RwDB, kvstore.RwTx) {
	tb.Helper()
	db := NewTestPoolDB(tb)
	tx, err := db.BeginRw(context.Background()) //nolint
	if err != nil {
		tb.Fatal(err)
	}
	if tb != nil {
		tb.Cleanup(tx.Rollback)
	}
	return db, tx
}

func NewTestSentryTx(tb testing.TB) (kvstore.RwDB, kvstore.RwTx) {
	tb.Helper()
	db := NewTestSentrylDB(tb)
	tx, err := db.BeginRw(context.Background()) //nolint
	if err != nil {
		tb.Fatal(err)
	}
	if tb != nil {
		tb.Cleanup(tx.Rollback)
	}
	return db, tx
}
