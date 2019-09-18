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

package mongodb

import (
	"context"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"time"
)

var client *mongo.Client

type Store struct {
	uri string
	dbName string
}

func NewClient(uri string) (*mongo.Client, *context.Context, error) {
	// add timeout for context
	// TODO: move timeout to config
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if client == nil {
		var err error
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return nil, nil, err
		}
	}
	return client, &ctx, nil
}

// TODO: add more config for db connection
func NewDB(uri, dbName string, drop bool) (*Store, error) {
	client, _, err := NewClient(uri)
	if err != nil {
		return nil, err
	}
	db := client.Database(dbName)

	if drop {
		if err := db.Drop(context.Background()); err != nil {
			return nil, err
		}
	}

	// create index for block
	if err := createBlockIndex(db); err != nil {
		return nil, err
	}

	// create index for transaction
	if err := createTransactionIndex(db); err != nil {
		return nil, err
	}

	// create index for dual event
	if err := createDualEventIndex(db); err != nil {
		return nil, err
	}

	// create index for receipt
	if err := createReceiptIndex(db); err != nil {
		return nil, err
	}

	// create index for commit
	if err := createCommitIndex(db); err != nil {
		return nil, err
	}

	// create index for trie
	if err := createTrieIndex(db); err != nil {
		return nil, err
	}

	// create index txLookupEntryTable
	if err := createTxLookupEntryIndex(db); err != nil {
		return nil, err
	}

	// disconnect client to close connection to mongodb
	//if err := client.Disconnect(*ctx); err != nil {
	//	return nil, err
	//}
	return &Store{uri: uri, dbName: dbName}, nil
}

// execute wraps executed code to a mongodb connection.
func (db *Store) execute(f func(mongoDb *mongo.Database, ctx *context.Context) error) error {
	if mongoDb, ctx, err := db.DB(); err != nil {
		return err
	} else {
		return f(mongoDb, ctx)
	}
}

// Put puts the given key / value to the queue
func (db *Store) Put(key, value interface{}) error {
	if result, _ := db.Has(key); !result {
		cache := Caching{
			Key: common.Bytes2Hex(key.([]byte)),
			Value: common.Bytes2Hex(value.([]byte)),
		}

		output, err := bson.Marshal(cache)
		if err != nil {
			return err
		}
		document, err := bsonx.ReadDoc(output)
		if err != nil {
			return err
		}
		return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
			_, e := mongoDb.Collection(trieTable).InsertOne(context.Background(), document)
			return e
		})
	}
	return nil
}

// WriteBody stores a block body into the database.
func (db *Store)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	log.Warn("WriteBody has not implemented yet")
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *Store)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	log.Warn("WriteBodyRLP has not implemented yet")
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *Store)WriteHeader(header *types.Header) {
	log.Warn("WriteHeader has not implemented yet")
}

// WriteChainConfig writes the chain config settings to the database.
func (db *Store)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	if err := db.insertChainConfig(cfg, hash); err != nil {
		log.Error("error while inserting chain config", "err", err, "hash", hash.Hex(), "cfg", cfg.String())
	}
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *Store)WriteBlock(block *types.Block) {
	newBlock := NewBlock(block)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		if e := db.insertBlock(mongoDb, ctx, newBlock); e != nil {
			return e
		}
		if block.NumTxs() > 0 {
			txs := make([]*Transaction, 0)
			for i, tx := range block.Transactions() {
				newTx, err := NewTransaction(tx, newBlock.Height, newBlock.Hash, i)
				if err != nil {
					log.Error("error while convert transaction", "err", err)
					continue
				}
				txs = append(txs, newTx)
			}
			if len(txs) > 0 {
				// insert many transactions
				return db.insertTransactions(txs)
			}
		}
		return nil
	}); err != nil {
		log.Error("error while insert new block", "err", err)
	}
}

func (db *Store)getReceiptByTxHash(mongoDb *mongo.Database, hash string) (*Receipt, error) {
	cur := mongoDb.Collection(receiptTable).FindOne(
		context.Background(),
		bson.M{txHash: bsonx.String(hash)},
	)
	var r Receipt
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store) getReceiptsByBlockHash(mongoDb *mongo.Database, hash common.Hash) ([]*Receipt, error) {
	cur, err := mongoDb.Collection(receiptTable).Find(context.Background(), bson.M{"blockHash": bsonx.String(hash.Hex())})
	if err != nil {
		return nil, err
	}
	receipts := make([]*Receipt, 0)
	for cur.Next(context.Background()) {
		var r Receipt
		if err := cur.Decode(&r); err != nil {
			return nil, err
		}
		receipts = append(receipts, &r)
	}
	return receipts, nil
}

func (db *Store)insertReceipts(mongoDb *mongo.Database, hash string, height uint64, receipts types.Receipts) error {
	newReceipts := make([]interface{}, 0)
	for _, receipt := range receipts {
		if _, err := db.getReceiptByTxHash(mongoDb, receipt.TxHash.Hex()); err != nil {
			r := NewReceipt(receipt, height, hash)
			newReceipts = append(newReceipts, r)
		}
	}

	if len(newReceipts) > 0 {
		if _, err := mongoDb.Collection(receiptTable).InsertMany(context.Background(), newReceipts); err != nil {
			return err
		}
	}
	return nil
}

func (db *Store)getHeadHeaderHash(mongoDb *mongo.Database) (*HeadHeaderHash, error) {
	cur := mongoDb.Collection(headHeaderTable).FindOne(
		context.Background(),
		bson.M{"ID": bsonx.Int32(1)},
	)
	var r HeadHeaderHash
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store)setHeadBlockHash(hash string) error {
	return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		collection := mongoDb.Collection(headBlockTable)
		if _, err := db.getHeadBlockHash(mongoDb); err != nil {
			// do insert
			head := HeadBlockHash{
				ID: 1,
				Hash: hash,
			}
			output, err := bson.Marshal(head)
			if err != nil {
				return err
			}
			document, err := bsonx.ReadDoc(output)
			if err != nil {
				return err
			}
			_, e := collection.InsertOne(*ctx, document)
			return e
		}
		// otherwise do update
		_, e := collection.UpdateOne(*ctx,  bson.M{"ID": 1}, bson.D{
			{"$set", bson.D{{"hash", hash}}},
		})
		return e
	})
}

func (db *Store)getHeadBlockHash(mongoDb *mongo.Database) (*HeadBlockHash, error) {
	cur := mongoDb.Collection(headBlockTable).FindOne(
		context.Background(),
		bson.M{"ID": bsonx.Int32(1)},
	)
	var r HeadBlockHash
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store)setHeadHeaderHash(hash string) error {
	return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		collection := mongoDb.Collection(headHeaderTable)
		if _, err := db.getHeadHeaderHash(mongoDb); err != nil {
			// do insert
			head := HeadHeaderHash{
				ID: 1,
				Hash: hash,
			}
			output, err := bson.Marshal(head)
			if err != nil {
				return err
			}
			document, err := bsonx.ReadDoc(output)
			if err != nil {
				return err
			}
			_, e := collection.InsertOne(*ctx, document)
			return e
		}
		// otherwise do update
		_, e := collection.UpdateOne(*ctx, bson.M{"ID": 1}, bson.D{
			{"$set", bson.D{{"hash", hash}}},
		})
		return e
	})
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *Store)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		return db.insertReceipts(mongoDb, hash.Hex(), height, receipts)
	}); err != nil {
		log.Error("error while writing receipts", "err", err, "height", height)
	}
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *Store)WriteCanonicalHash(hash common.Hash, height uint64) {
	log.Warn("WriteCanonicalHash has not implemented yet")
}

// WriteHeadBlockHash stores the head block's hash.
func (db *Store)WriteHeadBlockHash(hash common.Hash) {
	if err := db.setHeadBlockHash(hash.Hex()); err != nil {
		log.Error("error while set head block hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *Store)WriteHeadHeaderHash(hash common.Hash) {
	if err := db.setHeadHeaderHash(hash.Hex()); err != nil {
		log.Error("error while set head header hash", "err", err)
	}
}

// WriteCommit stores a commit into the database.
func (db *Store)WriteCommit(height uint64, commit *types.Commit) {
	newCommit := NewCommit(commit, height)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		return db.insertCommit(mongoDb, ctx, newCommit, height)
	}); err != nil {
		log.Error("error while insert commit", "err", err, "height", height, "commit", commit.String())
	}
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *Store)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	panic("WriteCommitRLP has not implemented yet")
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *Store)WriteTxLookupEntries(block *types.Block) {
	for idx, tx := range block.Transactions() {
		if blockHash, _, _ := db.ReadTxLookupEntry(tx.Hash()); blockHash.IsZero() {
			entry := TxLookupEntry{
				TxHash: tx.Hash().Hex(),
				BlockIndex: block.Height(),
				Index: uint64(idx),
				BlockHash: block.Hash().Hex(),
			}
			output, err := bson.Marshal(entry)
			if err != nil {
				log.Error("error while marshal entry", "err", err)
				return
			}
			document, err := bsonx.ReadDoc(output)
			if err != nil {
				log.Error("error while reading output to Doc", "err", err)
				return
			}
			if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
				_, e := mongoDb.Collection(txLookupEntryTable).InsertOne(*ctx, document)
				return e
			}); err != nil {
				log.Error("error while adding new txLookupEntry", "err", err, "txHash", tx.Hash(), "blockHeight", block.Height())
			}
		}
	}
}

// Stores a hash into the database.
func (db *Store)StoreHash(hash *common.Hash) {
	log.Warn("StoreHash has not implemented yet")
}

// Stores a tx hash into the database.
func (db *Store)StoreTxHash(hash *common.Hash) {
	log.Warn("StoreHash has not implemented yet")
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *Store)ReadCanonicalHash(height uint64) common.Hash {
	var hash common.Hash
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		block, e := db.getBlockById(mongoDb, ctx, height)
		if block != nil {
			hash = common.HexToHash(block.Hash)
		}
		return e
	}); err != nil {
		return common.NewZeroHash()
	}
	return hash
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *Store)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	config, err := db.getChainConfig(hash.Hex())
	if err != nil {
		log.Error("error while getting chain config", "err", err, "hash", hash.Hex())
		return nil
	}
	return config.ToChainConfig()
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *Store)ReadBody(hash common.Hash, height uint64) *types.Body {
	var body *types.Body
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		transactions, e := db.getTransactionsByBlockId(mongoDb, ctx, height)
		if e != nil {
			return e
		}
		txs := make([]*types.Transaction, 0)
		for _, transaction := range transactions {
			txs = append(txs, transaction.ToTransaction())
		}

		// get commit from block
		commit, e := db.getCommitById(mongoDb, ctx, height)
		if e != nil {
			return e
		}
		// TODO: get dualevents. currently make it empty
		dualEvents := make([]*types.DualEvent, 0)
		body = &types.Body{
			Transactions: txs,
			DualEvents: dualEvents,
			LastCommit: commit.ToCommit(),
		}
		return nil
	}); err != nil {
		log.Debug("error while getting body", "err", err, "height", height)
		return nil
	}
	return body
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *Store)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *Store)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	var newBlock *types.Block
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		var block *Block

		if block, e = db.getBlockById(mongoDb, ctx, height); e != nil {
			return e
		}
		// get transactions from db from block height
		transactions, e := db.getTransactionsByBlockId(mongoDb, ctx, height)
		if e != nil {
			return e
		}
		txs := make([]*types.Transaction, 0)
		for _, transaction := range transactions {
			txs = append(txs, transaction.ToTransaction())
		}

		// TODO: get dualevents. currently make it empty
		dualEvents := make([]*types.DualEvent, 0)
		newBlock = block.ToBlock(logger)
		body := types.Body{
			Transactions: txs,
			DualEvents: dualEvents,
		}
		if block.Height != 0 {
			commit, err := db.getCommitById(mongoDb, ctx, block.Height)
			if err != nil {
				return nil
			}
			body.LastCommit = commit.ToCommit()
		} else {
			commit := new(types.Commit)
			commit.MakeNilEmpty()
			body.LastCommit = commit
		}
		newBlock = newBlock.WithBody(&body)
		return nil
	}); err != nil {
		logger.Error("error while reading block", "err", err, "height", height, "hash", hash.Hex())
		return nil
	}
	return newBlock
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *Store)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *Store)ReadHeadBlockHash() common.Hash {
	hash := common.NewZeroHash()
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		head, e := db.getHeadBlockHash(mongoDb)
		if head != nil {
			hash = common.HexToHash(head.Hash)
		}
		return e
	}); err != nil {
		log.Error("error while reading head block hash", "err", err)
	}
	return hash
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *Store)ReadHeadHeaderHash() common.Hash {
	hash := common.NewZeroHash()
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		head, e := db.getHeadHeaderHash(mongoDb)
		if head != nil {
			hash = common.HexToHash(head.Hash)
		}
		return e
	}); err != nil {
		log.Error("error while reading head header hash", "err", err)
	}
	return hash
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *Store)ReadCommitRLP(height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadBody retrieves the commit at a given height.
func (db *Store)ReadCommit(height uint64) *types.Commit {
	var commit *Commit
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		commit, e = db.getCommitById(mongoDb, ctx, height)
		return e
	}); err != nil {
		log.Error("error while getting commit from height", "err", err, "height", height)
		return nil
	}
	return commit.ToCommit()
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *Store)ReadHeaderHeight(hash common.Hash) *uint64 {
	block, err := db.getBlockByHash(hash.Hex())
	if err != nil {
		log.Error("error while getting block", "err", err, "hash", hash.Hex())
		return nil
	}
	return &block.Height
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *Store)ReadHeader(hash common.Hash, height uint64) *types.Header {
	var block *Block
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		block, e = db.getBlockById(mongoDb, ctx, height)
		return e
	}); err != nil {
		log.Error("error while getting block by height", "err", err, "height", height)
		return nil
	}
	return block.ToHeader()
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *Store)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	var tx *Transaction
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		tx, e = db.getTransactionByHash(mongoDb, ctx, hash.Hex())
		return e
	}); err != nil || tx == nil {
		log.Error("error while getting tx from hash", "err", err, "hash", hash.Hex())
		return nil, common.NewZeroHash(), 0, 0
	}
	return tx.ToTransaction(), common.HexToHash(tx.BlockHash), tx.Height, uint64(tx.Index)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *Store)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	panic("Not implemented yet")
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *Store)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	panic("Not implemented yet")
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *Store)ReadHeaderNumber(hash common.Hash) *uint64 {
	height := uint64(0)
	return &height
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *Store)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	newReceipts := make(types.Receipts, 0)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		if receipts, e := db.getReceiptsByBlockHash(mongoDb, hash); e != nil {
			return e
		} else {
			for _, receipt := range receipts {
				newReceipts = append(newReceipts, receipt.ToReceipt())
			}
			return nil
		}
	}); err != nil {
		log.Error("error while getting receipts from block", "err", err, "height", number, "hash", hash.Hex())
		return nil
	}
	return newReceipts
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *Store)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	var txLookupEntry TxLookupEntry
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(txLookupEntryTable).FindOne(*ctx, bson.M{txHash: bsonx.String(hash.Hex())})
		return cur.Decode(&txLookupEntry)
	}); err != nil {
		return common.NewZeroHash(), 0, 0
	}
	return common.HexToHash(txLookupEntry.BlockHash), txLookupEntry.BlockIndex, txLookupEntry.Index
}

// Returns true if a hash already exists in the database.
func (db *Store)CheckHash(hash *common.Hash) bool {
	block, err := db.getBlockByHash(hash.Hex())
	if err != nil || block == nil {
		return false
	}
	return true
}

// Returns true if a tx hash already exists in the database.
func (db *Store)CheckTxHash(hash *common.Hash) bool {
	var tx *Transaction
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		tx, e = db.getTransactionByHash(mongoDb, ctx, hash.Hex())
		return e
	}); err != nil || tx == nil {
		return false
	}
	return true
}

// DeleteBody removes all block body data associated with a hash.
func (db *Store)DeleteBody(hash common.Hash, height uint64) {
	panic("DeleteBody has not implemented yet")
}

// DeleteHeader removes all block header data associated with a hash.
func (db *Store)DeleteHeader(hash common.Hash, height uint64) {
	panic("DeleteHeader has not implemented yet")
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *Store)DeleteCanonicalHash(number uint64) {
	panic("DeleteCanonicalHash has not implemented yet")
}

func (db *Store) Has(key interface{}) (bool, error) {
	if value, err := db.Get(key); value != nil {
		return true, err
	} else {
		return false, err
	}
}

// Get returns the given key if it's present.
func (db *Store) Get(key interface{}) (interface{}, error) {
	var c Caching
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(trieTable).FindOne(*ctx, bson.M{"key": bsonx.String(common.Bytes2Hex(key.([]byte)))})
		return cur.Decode(&c)
	}); err != nil {
		return nil, err
	}
	return common.Hex2Bytes(c.Value), nil
}

// Delete deletes the key from the queue and database
func (db *Store) Delete(key interface{}) error {
	panic("Not implemented yet")
}

func (db *Store) NewIterator() iterator.Iterator {
	panic("Not implemented yet")
}

func (db *Store) Close() {
	// Stop the metrics collection to avoid internal database races
	panic("Not implemented yet")
}

func (db *Store) DB() (*mongo.Database, *context.Context, error) {
	if client, ctx, err := NewClient(db.uri); err != nil {
		return nil, nil, err
	} else {
		return client.Database(db.dbName), ctx, nil
	}
}

func (db *Store) NewBatch() types.Batch {
	return newMongoDbBatch(db)
}

func (db *Store) getBlockById(mongoDb *mongo.Database, ctx *context.Context, blockId uint64) (*Block, error) {
	var b Block
	cur := mongoDb.Collection(blockTable).FindOne(*ctx, bson.M{height: bsonx.Int64(int64(blockId))})
	if err := cur.Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (db *Store) getBlockByHash(hash string) (*Block, error) {
	var b Block
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(blockTable).FindOne(*ctx, bson.M{"hash": bsonx.String(hash)})
		return cur.Decode(&b)
	}); err != nil {
		return nil, err
	}
	return &b, nil
}

func (db *Store)insertBlock(mongoDb *mongo.Database, ctx *context.Context, block *Block) error {
	if b, _ := db.getBlockById(mongoDb, ctx, block.Height); b == nil {
		output, e := bson.Marshal(block)
		if e != nil {
			return e
		}
		document, e := bsonx.ReadDoc(output)
		if e != nil {
			return e
		}
		_, e = mongoDb.Collection(blockTable).InsertOne(*ctx, document)
		return e
	}
	return nil
}

func (db *Store)getTransactionsByBlockId(mongoDb *mongo.Database, ctx *context.Context, height uint64) ([]*Transaction, error) {
	txs := make([]*Transaction, 0)
	if cur, err := mongoDb.Collection(txTable).Find(context.Background(), bson.M{"height": bsonx.Int64(int64(height))}); err != nil {
		return nil, err
	} else {
		for cur.Next(*ctx) {
			var tx Transaction
			if err := cur.Decode(&tx); err != nil {
				log.Error("error while decode tx data from database", "err", err)
				continue
			}
			txs = append(txs, &tx)
		}
	}
	return txs, nil
}

func (db *Store)getTransactionByHash(mongoDb *mongo.Database, ctx *context.Context, hash string) (*Transaction, error) {
	var tx Transaction
	cur := mongoDb.Collection(txTable).FindOne(
		*ctx,
		bson.M{"hash": bsonx.String(hash)},
	)
	if err := cur.Decode(&tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (db *Store)insertTransactions(transactions []*Transaction) error {
	return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		txs := make([]interface{}, 0)
		for _, t := range transactions {
			if _, err := db.getTransactionByHash(mongoDb, ctx, t.Hash); err != nil {
				txs = append(txs, t)
			}
		}

		if len(txs) > 0 {
			_, e := mongoDb.Collection(txTable).InsertMany(*ctx, txs)
			return e
		}
		return nil
	})
}

func (db *Store) getCommitById(mongoDb *mongo.Database, ctx *context.Context, blockId uint64) (*Commit, error) {
	cur := mongoDb.Collection(commitTable).FindOne(*ctx, bson.M{height: bsonx.Int64(int64(blockId))})
	var c Commit
	if err := cur.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *Store)insertCommit(mongoDb *mongo.Database, ctx *context.Context, commit *Commit, height uint64) error {
	if b, _ := db.getCommitById(mongoDb, ctx, height); b == nil {
		output, err := bson.Marshal(commit)
		if err != nil {
			return err
		}
		document, err := bsonx.ReadDoc(output)
		if err != nil {
			return err
		}
		_, e := mongoDb.Collection(commitTable).InsertOne(*ctx, document)
		return e
	}
	return nil
}

func (db *Store) getChainConfig(hash string) (*ChainConfig, error) {
	var c ChainConfig
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(chainConfigTable).FindOne(*ctx, bson.M{"hash": bsonx.String(hash)})
		return cur.Decode(&c)
	}); err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *Store) insertChainConfig(config *types.ChainConfig, hash common.Hash) error {
	if c, _ := db.getChainConfig(hash.Hex()); c == nil {
		chainConfig := NewChainConfig(config, hash)
		output, err := bson.Marshal(chainConfig)
		if err != nil {
			return err
		}
		document, err := bsonx.ReadDoc(output)
		if err != nil {
			return err
		}
		return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
			_, e := mongoDb.Collection(chainConfigTable).InsertOne(*ctx, document)
			return e
		})
	}
	return nil
}

type mongoDbBatch struct {
	db *Store
	size int
}

func newMongoDbBatch(db *Store) *mongoDbBatch {
	return &mongoDbBatch{db: db}
}

// Put puts the given key / value to the queue
func (db *mongoDbBatch) Put(key, value interface{}) error {
	return db.db.Put(key, value)
}

// WriteBody stores a block body into the database.
func (db *mongoDbBatch)WriteBody(hash common.Hash, height uint64, body *types.Body) {
	db.db.WriteBody(hash, height, body)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *mongoDbBatch)WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	db.db.WriteBodyRLP(hash, height, rlp)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *mongoDbBatch)WriteHeader(header *types.Header) {
	db.db.WriteHeader(header)
}

// WriteChainConfig writes the chain config settings to the database.
func (db *mongoDbBatch)WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	db.db.WriteChainConfig(hash, cfg)
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *mongoDbBatch)WriteBlock(block *types.Block) {
	db.db.WriteBlock(block)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (db *mongoDbBatch)WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	db.db.WriteReceipts(hash, height, receipts)
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *mongoDbBatch)WriteCanonicalHash(hash common.Hash, height uint64) {
	db.db.WriteCanonicalHash(hash, height)
}

// WriteHeadBlockHash stores the head block's hash.
func (db *mongoDbBatch)WriteHeadBlockHash(hash common.Hash) {
	db.db.WriteHeadBlockHash(hash)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *mongoDbBatch)WriteHeadHeaderHash(hash common.Hash) {
	db.db.WriteHeadHeaderHash(hash)
}

// WriteCommit stores a commit into the database.
func (db *mongoDbBatch)WriteCommit(height uint64, commit *types.Commit) {
	db.db.WriteCommit(height, commit)
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *mongoDbBatch)WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	db.db.WriteCommitRLP(height, rlp)
}

func (db *mongoDbBatch)WriteTxLookupEntries(block *types.Block) {
	db.db.WriteTxLookupEntries(block)
}

// Stores a hash into the database.
func (db *mongoDbBatch)StoreHash(hash *common.Hash) {
	db.db.StoreHash(hash)
}

// Stores a tx hash into the database.
func (db *mongoDbBatch)StoreTxHash(hash *common.Hash) {
	db.db.StoreTxHash(hash)
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *mongoDbBatch)ReadCanonicalHash(height uint64) common.Hash {
	return db.db.ReadCanonicalHash(height)
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func (db *mongoDbBatch)ReadChainConfig(hash common.Hash) *types.ChainConfig {
	return db.db.ReadChainConfig(hash)
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *mongoDbBatch)ReadBody(hash common.Hash, height uint64) *types.Body {
	return db.db.ReadBody(hash, height)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *mongoDbBatch)ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	return db.db.ReadBodyRLP(hash, height)
}

func (db *mongoDbBatch)ReadBlock(logger log.Logger, hash common.Hash, height uint64) *types.Block {
	return db.db.ReadBlock(logger, hash, height)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *mongoDbBatch)ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	return db.ReadHeaderRLP(hash, height)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *mongoDbBatch)ReadHeadBlockHash() common.Hash {
	return db.db.ReadHeadBlockHash()
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func (db *mongoDbBatch)ReadHeadHeaderHash() common.Hash {
	return db.db.ReadHeadHeaderHash()
}

// ReadCommitRLP retrieves the commit in RLP encoding.
func (db *mongoDbBatch)ReadCommitRLP(height uint64) rlp.RawValue {
	return db.db.ReadCommitRLP(height)
}

// ReadBody retrieves the commit at a given height.
func (db *mongoDbBatch)ReadCommit(height uint64) *types.Commit {
	return db.db.ReadCommit(height)
}

// ReadHeaderheight returns the header height assigned to a hash.
func (db *mongoDbBatch)ReadHeaderHeight(hash common.Hash) *uint64 {
	return db.db.ReadHeaderHeight(hash)
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *mongoDbBatch)ReadHeader(hash common.Hash, height uint64) *types.Header {
	return db.db.ReadHeader(hash, height)
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *mongoDbBatch)ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return db.db.ReadTransaction(hash)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *mongoDbBatch)ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return db.db.ReadDualEventLookupEntry(hash)
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *mongoDbBatch)ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	return db.db.ReadDualEvent(hash)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *mongoDbBatch)ReadHeaderNumber(hash common.Hash) *uint64 {
	return db.db.ReadHeaderNumber(hash)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *mongoDbBatch)ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	return db.db.ReadReceipts(hash, number)
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *mongoDbBatch)ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	return db.db.ReadTxLookupEntry(hash)
}

// Returns true if a hash already exists in the database.
func (db *mongoDbBatch)CheckHash(hash *common.Hash) bool {
	return db.db.CheckHash(hash)
}

// Returns true if a tx hash already exists in the database.
func (db *mongoDbBatch)CheckTxHash(hash *common.Hash) bool {
	return db.db.CheckTxHash(hash)
}

// DeleteBody removes all block body data associated with a hash.
func (db *mongoDbBatch)DeleteBody(hash common.Hash, height uint64) {
	db.db.DeleteBody(hash, height)
}

// DeleteHeader removes all block header data associated with a hash.
func (db *mongoDbBatch)DeleteHeader(hash common.Hash, height uint64) {
	db.db.DeleteHeader(hash, height)
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *mongoDbBatch)DeleteCanonicalHash(number uint64) {
	panic("DeleteCanonicalHash has not implemented yet")
}

func (db *mongoDbBatch) Has(key interface{}) (bool, error) {
	return db.db.Has(key)
}

// Get returns the given key if it's present.
func (db *mongoDbBatch) Get(key interface{}) (interface{}, error) {
	return db.db.Get(key)
}

// Delete deletes the key from the queue and database
func (db *mongoDbBatch) Delete(key interface{}) error {
	panic("Not implemented yet")
}

func (db *mongoDbBatch) NewIterator() iterator.Iterator {
	panic("Not implemented yet")
}

func (db *mongoDbBatch) Close() {
	// Stop the metrics collection to avoid internal database races
	panic("Not implemented yet")
}

func (db *mongoDbBatch) Write() error {
	return nil
}

func (db *mongoDbBatch) ValueSize() int {
	return db.size
}

func (db *mongoDbBatch) Reset() {
	db.size = 0
}

