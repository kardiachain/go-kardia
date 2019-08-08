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
