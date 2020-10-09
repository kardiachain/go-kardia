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

package storage

import (
	"github.com/kardiachain/go-kardiamain/kai/kaidb/leveldb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/types"
)

// DbInfo is used to start new database
type DbInfo interface {
	Name() string
	Start() (types.StoreDB, error)
}

// LevelDbInfo implements DbInfo to start chain using levelDB
type LevelDbInfo struct {
	ChainData string
	DbCaches  int
	DbHandles int
}

func NewLevelDbInfo(chainData string, dbCaches, dbHandles int) *LevelDbInfo {
	return &LevelDbInfo{
		ChainData: chainData,
		DbCaches:  dbCaches,
		DbHandles: dbHandles,
	}
}

func (info *LevelDbInfo) Name() string {
	return "levelDB"
}

func (info *LevelDbInfo) Start() (types.StoreDB, error) {
	db, err := leveldb.New(info.ChainData, info.DbCaches, info.DbHandles, "kai")
	if err != nil {
		return nil, err
	}

	return kvstore.NewStoreDB(db), nil
}

func NewMemoryDatabase() types.StoreDB {
	return kvstore.NewStoreDB(memorydb.New())
}

// NewLevelDBDatabase creates a persistent key-value database without a freezer
// moving immutable chain segments into cold storage.
func NewLevelDBDatabase(file string, cache int, handles int, namespace string) (types.StoreDB, error) {
	db, err := leveldb.New(file, cache, handles, namespace)
	if err != nil {
		return nil, err
	}
	return kvstore.NewStoreDB(db), nil
}
