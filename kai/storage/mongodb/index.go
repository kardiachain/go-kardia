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

/*
 *	index.go is used to define all indices before starting mongodb.
 */
package mongodb

import (
	"context"
	"github.com/kardiachain/go-kardia/lib/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"time"
)

const (
	height = "height"
	hash = "hash"
	hashed = "hashed"
	blockHash = "blockHash"
	txHash = "txHash"
	from = "from"
	to = "to"
	contractAddress = "contractAddress"
	key = "key"
	blockIndex = "blockIndex"
	address = "address"
)

func indexModel(key string, unique bool, indexType string) mongo.IndexModel {
	index := mongo.IndexModel{}
	if indexType == hashed {
		index.Keys = bsonx.Doc{{Key: key, Value: bsonx.String(indexType)}}
	} else {
		index.Keys = bsonx.Doc{{Key: key, Value: bsonx.Int32(1)}}
	}
	//index.Keys = keysBson
	if unique {
		opts := options.IndexOptions{Unique: &unique}
		index.Options = &opts
	}
	log.Info("index", "data", index)
	return index
}

func createIndex(key string, unique bool, indexType string, collection *mongo.Collection) error {
	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)
	indexModel := indexModel(key, unique, indexType)
	_, err := collection.Indexes().CreateOne(context.Background() , indexModel, opts)
	return err
}

// createBlockIndex creates indices for block table
func createBlockIndex(db *mongo.Database) error {
	blockCollection := db.Collection(blockTable)
	if err := createIndex(height, true, "", blockCollection); err != nil {
		return err
	}
	if err := createIndex(hash, false, hashed, blockCollection); err != nil {
		return err
	}
	return nil
}

// createTransactionIndex creates indices for transaction table
func createTransactionIndex(db *mongo.Database) error {
	txCollection := db.Collection(txTable)
	if err := createIndex(height, false, "", txCollection); err != nil {
		return err
	}
	if err := createIndex(hash, false, hashed, txCollection); err != nil {
		return err
	}
	if err := createIndex(blockHash, false, hashed, txCollection); err != nil {
		return err
	}
	if err := createIndex(from, false, hashed, txCollection); err != nil {
		return err
	}
	if err := createIndex(to, false, hashed, txCollection); err != nil {
		return err
	}
	return nil
}

// createReceiptIndex creates indices for receipt table
func createReceiptIndex(db *mongo.Database) error {
	receiptCollection := db.Collection(receiptTable)
	if err := createIndex(blockHash, false, hashed, receiptCollection); err != nil {
		return err
	}
	if err := createIndex(txHash, false, hashed, receiptCollection); err != nil {
		return err
	}
	if err := createIndex(contractAddress, false, hashed, receiptCollection); err != nil {
		return err
	}
	if err := createIndex(height, false, "", receiptCollection); err != nil {
		return err
	}
	return nil
}

// createDualEventIndex creates indices for dualEvent Table
func createDualEventIndex(db *mongo.Database) error {
	dualEvtCollection := db.Collection(dualEvtTable)
	if err := createIndex(height, false, "", dualEvtCollection); err != nil {
		return err
	}
	if err := createIndex(hash, false, hashed, dualEvtCollection); err != nil {
		return err
	}
	if err := createIndex(blockHash, false, hashed, dualEvtCollection); err != nil {
		return err
	}
	return nil
}

// createCommitIndex creates indices for commit table
func createCommitIndex(db *mongo.Database) error {
	commitCollection := db.Collection(commitTable)
	if err := createIndex(height, true, "", commitCollection); err != nil {
		return err
	}
	return nil
}

// createTrieIndex creates indices for trie table
func createTrieIndex(db *mongo.Database) error {
	trieCollection := db.Collection(trieTable)
	if err := createIndex(key, false, hashed, trieCollection); err != nil {
		return err
	}
	return nil
}

// createTxLookupEntryIndex creates indices for txLookupEntry table
func createTxLookupEntryIndex(db *mongo.Database) error {
	txLookupEntryCollection := db.Collection(txLookupEntryTable)
	if err := createIndex(blockHash, false, hashed, txLookupEntryCollection); err != nil {
		return err
	}
	if err := createIndex(txHash, false, hashed, txLookupEntryCollection); err != nil {
		return err
	}
	if err := createIndex(blockIndex, false, "", txLookupEntryCollection); err != nil {
		return err
	}
	return nil
}
