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

// DBInfo is used to start new database
type DBInfo interface {
	Name() string
	Start() (types.Database, error)
}

// MongoDBInfo implements DBInfo to start chain using MongoDB
type MongoDBInfo struct {
	URI string
	DatabaseName string
	Drop bool
}

// LDBInfo implements DBInfo to start chain using levelDB
type LDBInfo struct {
	ChainData string
	DbCaches int
	DbHandles int
}

func NewMongoDBInfo(uri, databaseName string, drop bool) *MongoDBInfo {
	return &MongoDBInfo{
		URI: uri,
		DatabaseName: databaseName,
		Drop: drop,
	}
}

func (db *MongoDBInfo) Name() string {
	return "MongoDB"
}

func (db *MongoDBInfo) Start() (types.Database, error) {
	return mongodb.NewDB(db.URI, db.DatabaseName, db.Drop)
}

func NewLDBInfo(chainData string, dbCaches, dbHandles int) *LDBInfo {
	return &LDBInfo{
		ChainData: chainData,
		DbCaches: dbCaches,
		DbHandles: dbHandles,
	}
}

func (db *LDBInfo) Name() string {
	return "levelDB"
}

func (db *LDBInfo) Start() (types.Database, error) {
	return NewLDBStore(db.ChainData, db.DbCaches, db.DbHandles)
}
