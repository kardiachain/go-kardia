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
	"errors"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

var client *mongo.Client

type Store struct {
	uri    string
	dbName string
}

func NewClient(uri string) (*mongo.Client, *context.Context, context.CancelFunc, error) {
	// add timeout for context
	// TODO: move timeout to config
	ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Minute)
	if client == nil {
		var err error
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return client, &ctx, cancelCtx, nil
}

// TODO: add more config for db connection
func NewDB(uri, dbName string, drop bool) (*Store, error) {
	client, ctx, cancelCtxFunc, err := NewClient(uri)
	if err != nil {
		return nil, err
	}
	defer cancelCtxFunc()
	db := client.Database(dbName)

	if drop {
		if err := db.Drop(*ctx); err != nil {
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

	// create index for watcherAction
	if err := createWatcherActionIndex(db); err != nil {
		return nil, err
	}

	// create index for dualAction
	if err := createDualActionIndex(db); err != nil {
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
	if mongoDb, ctx, cancelCtxFunc, err := db.db(); err != nil {
		return err
	} else {
		defer cancelCtxFunc()
		return f(mongoDb, ctx)
	}
}

// Put puts the given key / value to the queue
func (db *Store) Put(key, value []byte) error {
	if result, _ := db.Has(key); !result {
		cache := Caching{
			Key:   common.Bytes2Hex(key),
			Value: common.Bytes2Hex(value),
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
			_, e := mongoDb.Collection(trieTable).InsertOne(*ctx, document)
			return e
		})
	}
	return nil
}

// WriteBody stores a block body into the database.
func (db *Store) WriteBody(hash common.Hash, height uint64, body *types.Body) {
	log.Warn("WriteBody has not implemented yet")
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func (db *Store) WriteBodyRLP(hash common.Hash, height uint64, rlp rlp.RawValue) {
	log.Warn("WriteBodyRLP has not implemented yet")
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-height mapping.
func (db *Store) WriteHeader(header *types.Header) {
	log.Warn("WriteHeader has not implemented yet")
}

// WriteChainConfig writes the chain config settings to the database.
func (db *Store) WriteChainConfig(hash common.Hash, cfg *types.ChainConfig) {
	if err := db.insertChainConfig(cfg, hash); err != nil {
		log.Error("error while inserting chain config", "err", err, "hash", hash.Hex(), "cfg", cfg.String())
	}
}

// WriteBlock serializes a block into the database, header and body separately.
func (db *Store) WriteBlock(block *types.Block, parts *types.PartSet, seenCommit *types.Commit) {
	newBlock := NewBlock(block)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		if e := db.insertBlock(mongoDb, ctx, newBlock); e != nil {
			return e
		}
		if block.NumTxs() > 0 {
			go func() {
				if err := db.insertTransactions(block.Transactions(), newBlock.Height, newBlock.Hash); err != nil {
					log.Error("error while insert new transactions", "err", err, "block", block.Height())
				}
			}()
		}
		return nil
	}); err != nil {
		log.Error("error while insert new block", "err", err)
	}
}

func (db *Store) getReceiptByTxHash(mongoDb *mongo.Database, ctx *context.Context, hash string) (*Receipt, error) {
	cur := mongoDb.Collection(receiptTable).FindOne(
		*ctx,
		bson.M{txHash: bsonx.String(hash)},
	)
	var r Receipt
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store) getReceiptsByBlockHash(mongoDb *mongo.Database, ctx *context.Context, hash common.Hash) ([]*Receipt, error) {
	cur, err := mongoDb.Collection(receiptTable).Find(*ctx, bson.M{"blockHash": bsonx.String(hash.Hex())})
	if err != nil {
		return nil, err
	}
	receipts := make([]*Receipt, 0)
	for cur.Next(*ctx) {
		var r Receipt
		if err := cur.Decode(&r); err != nil {
			return nil, err
		}
		receipts = append(receipts, &r)
	}
	return receipts, nil
}

func (db *Store) insertReceipts(hash string, height uint64, receipts types.Receipts) error {
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		newReceipts := make([]interface{}, 0)
		for _, receipt := range receipts {
			if _, err := db.getReceiptByTxHash(mongoDb, ctx, receipt.TxHash.Hex()); err != nil {
				r := NewReceipt(receipt, height, hash)
				newReceipts = append(newReceipts, r)
			}
		}

		if len(newReceipts) > 0 {
			if _, err := mongoDb.Collection(receiptTable).InsertMany(*ctx, newReceipts); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (db *Store) getHeadHeaderHash(mongoDb *mongo.Database, ctx *context.Context) (*HeadHeaderHash, error) {
	cur := mongoDb.Collection(headHeaderTable).FindOne(
		*ctx,
		bson.M{"ID": bsonx.Int32(1)},
	)
	var r HeadHeaderHash
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store) setHeadBlockHash(hash string) error {
	return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		collection := mongoDb.Collection(headBlockTable)
		if _, err := db.getHeadBlockHash(mongoDb, ctx); err != nil {
			// do insert
			head := HeadBlockHash{
				ID:   1,
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

func (db *Store) getHeadBlockHash(mongoDb *mongo.Database, ctx *context.Context) (*HeadBlockHash, error) {
	cur := mongoDb.Collection(headBlockTable).FindOne(
		*ctx,
		bson.M{"ID": bsonx.Int32(1)},
	)
	var r HeadBlockHash
	err := cur.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *Store) setHeadHeaderHash(hash string) error {
	return db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		collection := mongoDb.Collection(headHeaderTable)
		if _, err := db.getHeadHeaderHash(mongoDb, ctx); err != nil {
			// do insert
			head := HeadHeaderHash{
				ID:   1,
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
func (db *Store) WriteReceipts(hash common.Hash, height uint64, receipts types.Receipts) {
	// add this process into goroutine in order to prevent slow sync processes
	go func() {
		if err := db.insertReceipts(hash.Hex(), height, receipts); err != nil {
			log.Error("error while writing receipts", "err", err, "height", height)
		}
	}()
}

// WriteCanonicalHash stores the hash assigned to a canonical block height.
func (db *Store) WriteCanonicalHash(hash common.Hash, height uint64) {
	log.Warn("WriteCanonicalHash has not implemented yet")
}

// WriteHeadBlockHash stores the head block's hash.
func (db *Store) WriteHeadBlockHash(hash common.Hash) {
	if err := db.setHeadBlockHash(hash.Hex()); err != nil {
		log.Error("error while set head block hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func (db *Store) WriteHeadHeaderHash(hash common.Hash) {
	if err := db.setHeadHeaderHash(hash.Hex()); err != nil {
		log.Error("error while set head header hash", "err", err)
	}
}

// WriteCommit stores a commit into the database.
func (db *Store) WriteCommit(height uint64, commit *types.Commit) {
	newCommit := NewCommit(commit, height)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		return db.insertCommit(mongoDb, ctx, newCommit, height)
	}); err != nil {
		log.Error("error while insert commit", "err", err, "height", height, "commit", commit.String())
	}
}

// WriteCommitRLP stores an RLP encoded commit into the database.
func (db *Store) WriteCommitRLP(height uint64, rlp rlp.RawValue) {
	panic("WriteCommitRLP has not implemented yet")
}

func (db *Store) WriteEvent(smc *types.KardiaSmartcontract) {
	if smc == nil {
		log.Warn("smc is nil")
		return
	}
	if len(smc.WatcherActions) > 0 {
		for _, action := range smc.WatcherActions {
			evt := WatcherAction{
				ContractAddress: smc.SmcAddress,
				ABI:             smc.SmcAbi,
				Method:          action.Method,
				DualAction:      action.DualAction,
			}
			output, err := bson.Marshal(evt)
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
				_, e := mongoDb.Collection(watcherActionTable).InsertOne(*ctx, document)
				return e
			}); err != nil {
				log.Error("error while adding new event", "err", err, "address", smc.SmcAddress, "method", action.Method)
				return
			}
		}
	}

	if len(smc.DualActions) > 0 {
		for _, action := range smc.DualActions {
			evt := DualAction{
				Name:            action.Name,
				ContractAddress: smc.SmcAddress,
				ABI:             smc.SmcAbi,
			}
			output, err := bson.Marshal(evt)
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
				_, e := mongoDb.Collection(dualActionTable).InsertOne(*ctx, document)
				return e
			}); err != nil {
				log.Error("error while adding new event", "err", err, "address", smc.SmcAddress, "method", action.Name)
				return
			}
		}
	}
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func (db *Store) WriteTxLookupEntries(block *types.Block) {
	for idx, tx := range block.Transactions() {
		if blockHash, _, _ := db.ReadTxLookupEntry(tx.Hash()); blockHash.IsZero() {
			entry := TxLookupEntry{
				TxHash:     tx.Hash().Hex(),
				BlockIndex: block.Height(),
				Index:      uint64(idx),
				BlockHash:  block.Hash().Hex(),
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
func (db *Store) StoreHash(hash *common.Hash) {
	log.Warn("StoreHash has not implemented yet")
}

// Stores a tx hash into the database.
func (db *Store) StoreTxHash(hash *common.Hash) {
	log.Warn("StoreHash has not implemented yet")
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block height.
func (db *Store) ReadCanonicalHash(height uint64) common.Hash {
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
func (db *Store) ReadChainConfig(hash common.Hash) *types.ChainConfig {
	config, err := db.getChainConfig(hash.Hex())
	if err != nil {
		log.Error("error while getting chain config", "err", err, "hash", hash.Hex())
		return nil
	}
	return config.ToChainConfig()
}

// ReadBody retrieves the block body corresponding to the hash.
func (db *Store) ReadBody(hash common.Hash, height uint64) *types.Body {
	signer := types.HomesteadSigner{}
	var body *types.Body
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		transactions, e := db.getTransactionsByBlockId(mongoDb, ctx, height)
		if e != nil {
			return e
		}
		txs := make([]*types.Transaction, 0)
		for _, transaction := range transactions {
			txs = append(txs, transaction.ToTransaction(signer))
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
			DualEvents:   dualEvents,
			LastCommit:   commit.ToCommit(),
		}
		return nil
	}); err != nil {
		log.Debug("error while getting body", "err", err, "height", height)
		return nil
	}
	return body
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func (db *Store) ReadBodyRLP(hash common.Hash, height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func (db *Store) ReadBlock(hash common.Hash, height uint64) *types.Block {
	var newBlock *types.Block

	signer := types.HomesteadSigner{}
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
			txs = append(txs, transaction.ToTransaction(signer))
		}

		// TODO: get dualevents. currently make it empty
		dualEvents := make([]*types.DualEvent, 0)
		newBlock = block.ToBlock()
		body := types.Body{
			Transactions: txs,
			DualEvents:   dualEvents,
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
		panic(err)
		return nil
	}
	return newBlock
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *Store) ReadHeaderRLP(hash common.Hash, height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func (db *Store) ReadSeenCommit(height uint64) *types.Commit {
	panic("Not implemented yet")
}

func (db *Store) ReadAppHash(height uint64) common.Hash {
	panic("Not implemented yet")
}

func (db *Store) WriteAppHash(height uint64, hash common.Hash) {
	panic("Not implemented yet")
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func (db *Store) ReadHeadBlockHash() common.Hash {
	hash := common.NewZeroHash()
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		head, e := db.getHeadBlockHash(mongoDb, ctx)
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
func (db *Store) ReadHeadHeaderHash() common.Hash {
	hash := common.NewZeroHash()
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		head, e := db.getHeadHeaderHash(mongoDb, ctx)
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
func (db *Store) ReadCommitRLP(height uint64) rlp.RawValue {
	panic("Not implemented yet")
}

// ReadBody retrieves the commit at a given height.
func (db *Store) ReadCommit(height uint64) *types.Commit {
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
func (db *Store) ReadHeaderHeight(hash common.Hash) *uint64 {
	block, err := db.getBlockByHash(hash.Hex())
	if err != nil {
		log.Error("error while getting block", "err", err, "hash", hash.Hex())
		return nil
	}
	return &block.Height
}

// ReadHeader retrieves the block header corresponding to the hash.
func (db *Store) ReadHeader(hash common.Hash, height uint64) *types.Header {
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

func (db *Store) DeleteBlockMeta(hash common.Hash, height uint64) {

}

func (db *Store) DeleteBlockPart(hash common.Hash, height uint64) {

}

func (db *Store) ReadBlockPart(hash common.Hash, height uint64, index int) *types.Part {
	panic("read block part error")
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func (db *Store) ReadTransaction(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	signer := types.HomesteadSigner{}
	var tx *Transaction
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		var e error
		tx, e = db.getTransactionByHash(mongoDb, ctx, hash.Hex())
		return e
	}); err != nil || tx == nil {
		log.Error("error while getting tx from hash", "err", err, "hash", hash.Hex())
		return nil, common.NewZeroHash(), 0, 0
	}
	return tx.ToTransaction(signer), common.HexToHash(tx.BlockHash), tx.Height, uint64(tx.Index)
}

// Retrieves the positional metadata associated with a dual's event
// hash to allow retrieving the event by hash.
func (db *Store) ReadDualEventLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
	panic("Not implemented yet")
}

// Retrieves a specific dual's event from the database, along with
// its added positional metadata.
func (db *Store) ReadDualEvent(hash common.Hash) (*types.DualEvent, common.Hash, uint64, uint64) {
	panic("Not implemented yet")
}

// ReadHeaderNumber returns the header number assigned to a hash.
func (db *Store) ReadHeaderNumber(hash common.Hash) *uint64 {
	height := uint64(0)
	return &height
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func (db *Store) ReadReceipts(hash common.Hash, number uint64) types.Receipts {
	newReceipts := make(types.Receipts, 0)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		if receipts, e := db.getReceiptsByBlockHash(mongoDb, ctx, hash); e != nil {
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

func (db *Store) getEvents(address string) ([]*WatcherAction, error) {
	events := make([]*WatcherAction, 0)
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur, e := mongoDb.Collection(watcherActionTable).Find(*ctx, bson.M{"contractAddress": bsonx.String(address)})
		if e != nil {
			return e
		}
		for cur.Next(*ctx) {
			var r WatcherAction
			if err := cur.Decode(&r); err != nil {
				return err
			}
			events = append(events, &r)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return events, nil
}

func (db *Store) getEvent(address, method string) (*WatcherAction, error) {
	var event WatcherAction
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		filter := bson.A{
			bson.D{{"contractAddress", bsonx.String(address)}},
			bson.D{{"method", bsonx.String(method)}},
		}
		cur := mongoDb.Collection(watcherActionTable).FindOne(
			*ctx,
			filter,
		)
		err := cur.Decode(&event)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &event, nil
}

func (db *Store) getEventByDualAction(action string) (*DualAction, error) {
	var event DualAction
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(dualActionTable).FindOne(
			*ctx,
			bson.M{"name": bsonx.String(action)},
		)
		err := cur.Decode(&event)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &event, nil
}

func (db *Store) ReadSmartContractAbi(address string) *abi.ABI {
	events, err := db.getEvents(address)
	if err != nil || events == nil || len(events) == 0 {
		return nil
	}
	if events[0].ABI != "" {
		abiStr := strings.Replace(events[0].ABI, "'", "\"", -1)
		a, err := abi.JSON(strings.NewReader(abiStr))
		if err != nil {
			return nil
		}
		return &a
	}
	return nil
}

func (db *Store) ReadEvent(address string, method string) *types.WatcherAction {
	event, err := db.getEvent(address, method)
	if err != nil {
		return nil
	}
	return &types.WatcherAction{
		Method:     event.Method,
		DualAction: event.DualAction,
	}
}

func (db *Store) ReadEvents(address string) []*types.WatcherAction {
	events, err := db.getEvents(address)
	if err != nil {
		return nil
	}
	watcherActions := make([]*types.WatcherAction, 0)
	for _, evt := range events {
		watcherAction := &types.WatcherAction{
			Method:     evt.Method,
			DualAction: evt.DualAction,
		}
		watcherActions = append(watcherActions, watcherAction)
	}
	return watcherActions
}

func (db *Store) ReadSmartContractFromDualAction(action string) (string, *abi.ABI) {
	event, err := db.getEventByDualAction(action)
	if err != nil {
		return "", nil
	}

	a, err := abi.JSON(strings.NewReader(event.ABI))
	if err != nil {
		return "", nil
	}
	return event.ContractAddress, &a
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func (db *Store) ReadTxLookupEntry(hash common.Hash) (common.Hash, uint64, uint64) {
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
func (db *Store) CheckHash(hash *common.Hash) bool {
	block, err := db.getBlockByHash(hash.Hex())
	if err != nil || block == nil {
		return false
	}
	return true
}

// Returns true if a tx hash already exists in the database.
func (db *Store) CheckTxHash(hash *common.Hash) bool {
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
func (db *Store) DeleteBody(hash common.Hash, height uint64) {
	panic("DeleteBody has not implemented yet")
}

// DeleteHeader removes all block header data associated with a hash.
func (db *Store) DeleteHeader(hash common.Hash, height uint64) {
	panic("DeleteHeader has not implemented yet")
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *Store) DeleteCanonicalHash(number uint64) {
	panic("DeleteCanonicalHash has not implemented yet")
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func (db *Store) ReadBlockMeta(hash common.Hash, number uint64) *types.BlockMeta {
	panic("ReadBlockMeta has not implemented yet")
}

func (db *Store) Has(key []byte) (bool, error) {
	if value, err := db.Get(key); value != nil {
		return true, err
	} else {
		return false, err
	}
}

// Get returns the given key if it's present.
func (db *Store) Get(key []byte) ([]byte, error) {
	var c Caching
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		cur := mongoDb.Collection(trieTable).FindOne(*ctx, bson.M{"key": bsonx.String(common.Bytes2Hex(key))})
		return cur.Decode(&c)
	}); err != nil {
		return nil, err
	}
	return common.Hex2Bytes(c.Value), nil
}

// Delete deletes the key from the queue and database
func (db *Store) Delete(key []byte) error {
	panic("Not implemented yet")
}

func (db *Store) NewIterator() kaidb.Iterator {
	panic("Not implemented yet")
}

func (db *Store) NewIteratorWithPrefix(prefix []byte) kaidb.Iterator {
	panic("Not implemented yet")
}

func (db *Store) NewIteratorWithStart(prefix []byte) kaidb.Iterator {
	panic("Not implemented yet")
}

func (db *Store) Close() error {
	// Stop the metrics collection to avoid internal database races
	return errors.New("Not implemented yet")
}

func (db *Store) Stat(a string) (string, error) {
	return "", nil
}

// Compact is not supported on a memory database, but there's no need either as
// a memory database doesn't waste space anyway.
func (db *Store) Compact(start []byte, limit []byte) error {
	return nil
}

// Compact is not supported on a memory database, but there's no need either as
// a memory database doesn't waste space anyway.
func (db *Store) NewBatch() kaidb.Batch {
	return nil
}

func (db *Store) DB() kaidb.Database {
	return db
}

func (db *Store) db() (*mongo.Database, *context.Context, context.CancelFunc, error) {
	if client, ctx, cancelCtxFunc, err := NewClient(db.uri); err != nil {
		return nil, nil, nil, err
	} else {
		return client.Database(db.dbName), ctx, cancelCtxFunc, nil
	}
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

func (db *Store) insertBlock(mongoDb *mongo.Database, ctx *context.Context, block *Block) error {
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

func (db *Store) getTransactionsByBlockId(mongoDb *mongo.Database, ctx *context.Context, height uint64) ([]*Transaction, error) {
	txs := make([]*Transaction, 0)
	if cur, err := mongoDb.Collection(txTable).Find(*ctx, bson.M{"height": bsonx.Int64(int64(height))}); err != nil {
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

func (db *Store) getTransactionByHash(mongoDb *mongo.Database, ctx *context.Context, hash string) (*Transaction, error) {
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

func (db *Store) insertTransactions(transactions types.Transactions, blockHeight uint64, blockHash string) error {
	if err := db.execute(func(mongoDb *mongo.Database, ctx *context.Context) error {
		txs := make([]interface{}, 0)
		for i, tx := range transactions {
			if _, err := db.getTransactionByHash(mongoDb, ctx, tx.Hash().Hex()); err != nil {
				newTx, err := NewTransaction(tx, blockHeight, blockHash, i)
				if err != nil {
					log.Error("error while convert transaction", "err", err)
					continue
				}
				txs = append(txs, newTx)
			}
		}

		if len(txs) > 0 {
			_, e := mongoDb.Collection(txTable).InsertMany(*ctx, txs)
			return e
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (db *Store) getCommitById(mongoDb *mongo.Database, ctx *context.Context, blockId uint64) (*Commit, error) {
	cur := mongoDb.Collection(commitTable).FindOne(*ctx, bson.M{height: bsonx.Int64(int64(blockId))})
	var c Commit
	if err := cur.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *Store) insertCommit(mongoDb *mongo.Database, ctx *context.Context, commit *Commit, height uint64) error {
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
