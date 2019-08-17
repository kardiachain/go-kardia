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
	"github.com/kardiachain/go-kardia/kai/storage/mongodb"
	"github.com/kardiachain/go-kardia/types"
)

// DbInfo is used to start new database
type DbInfo interface {
	Name() string
	Start() (types.Database, error)
}

// MongoDbInfo implements DbInfo to start chain using MongoDB
type MongoDbInfo struct {
	URI string
	DatabaseName string
	Drop bool // if drop is true, drop database
}

// LevelDbInfo implements DbInfo to start chain using levelDB
type LevelDbInfo struct {
	ChainData string
	DbCaches int
	DbHandles int
}

func NewMongoDbInfo(uri, databaseName string, drop bool) *MongoDbInfo {
	return &MongoDbInfo{
		URI: uri,
		DatabaseName: databaseName,
		Drop: drop,
	}
}

func (db *MongoDbInfo) Name() string {
	return "MongoDB"
}

func (db *MongoDbInfo) Start() (types.Database, error) {
	return mongodb.NewDB(db.URI, db.DatabaseName, db.Drop)
}

func NewLevelDbInfo(chainData string, dbCaches, dbHandles int) *LevelDbInfo {
	return &LevelDbInfo{
		ChainData: chainData,
		DbCaches: dbCaches,
		DbHandles: dbHandles,
	}
}

func (db *LevelDbInfo) Name() string {
	return "levelDB"
}

func (db *LevelDbInfo) Start() (types.Database, error) {
	return NewLDBStore(db.ChainData, db.DbCaches, db.DbHandles)
}
