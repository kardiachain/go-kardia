package main

import (
	"flag"
	"fmt"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"

	"gopkg.in/urfave/cli.v1"
)

var (
	recoverTxLookupIndexCmd = cli.Command{
		Name:  "recover",
		Usage: "Recover receipts of corrupted transactions in each block",
		Action: func(ctx *cli.Context) error {
			flag.Parse()
			cfg, err := LoadConfig(args)
			if err != nil {
				return err
			}

			err = recoverTxLookupEntry(cfg)
			if err != nil {
				return fmt.Errorf("failed to recover: %w", err)
			}

			return nil
		},
	}
)

func recoverTxLookupEntry(cfg *Config) error {
	// use the parsed config to load the storeDB
	store, err := loadStoreDB(cfg)
	if err != nil {
		return err
	}
	var (
		block *types.Block
		bi    *types.BlockInfo
	)
	for i := cfg.Debug.FromBlock; i <= cfg.Debug.EndBlock; i++ {
		if i%100000 == 0 {
			fmt.Printf("Processing block %v...\n", i)
		}
		block = store.ReadBlock(i)
		bi = store.ReadBlockInfo(block.Hash(), block.Height())
		if err = processBlockInfo(store, block, bi); err != nil {
			fmt.Printf("Error while processing block %v...\n", i)
			return err
		}
	}
	return nil
}

func loadStoreDB(cfg *Config) (types.StoreDB, error) {
	nodeCfg, err := cfg.getNodeConfig()
	if err != nil {
		return nil, err
	}
	db, err := storage.NewLevelDBDatabase(nodeCfg.ResolvePath("chaindata"), 16, 32, "chaindata")
	if err != nil {
		return nil, err
	}
	return db, nil
}

func processBlockInfo(store types.StoreDB, block *types.Block, bi *types.BlockInfo) error {
	if block.NumTxs() == uint64(len(bi.Receipts)) || block.NumTxs() == 0 {
		return nil
	}

	txs := types.Transactions{}
	receipts := bi.Receipts
	for i := uint64(0); i < block.NumTxs(); i++ {
		if i < uint64(len(receipts)) && bi.Receipts[i].TxHash.Equal(block.Transactions()[i].Hash()) {
			txs = append(txs, block.Transactions()[i])
			continue
		}
		tx, blockHash, blockHeight, _ := store.ReadTransaction(block.Transactions()[i].Hash())
		if tx == nil {
			fmt.Printf("Inserting fake receipt of a bad tx, hash: %v, block height %v\n", block.Transactions()[i].Hash().Hex(), block.Height())
			receipts = insertReceipts(receipts, i, reconstructBadReceipt(block.Transactions()[i]))
		} else {
			correctBi := store.ReadBlockInfo(blockHash, blockHeight)
			for j := 0; j < len(correctBi.Receipts); j++ {
				if correctBi.Receipts[j].TxHash.Equal(block.Transactions()[i].Hash()) {
					fmt.Printf("Correcting receipt of a bad tx, hash: %v, wrong block height %v, correct block height %v\n", block.Transactions()[i].Hash().Hex(), block.Height(), blockHeight)
					receipts = insertReceipts(receipts, i, correctBi.Receipts[j])
				}
			}
		}
	}
	store.WriteBlockInfo(block.Hash(), block.Height(), bi)
	if err := rewriteTxLookupIndex(store.DB(), block.Hash(), block.Height(), txs); err != nil {
		return err
	}
	fmt.Printf("Bad block %v, NumTxs %v, NumReceipts %v and %v\n", block.Height(), block.NumTxs(), len(bi.Receipts), len(receipts))
	return nil
}

func reconstructBadReceipt(tx *types.Transaction) *types.Receipt {
	return &types.Receipt{
		PostState:         nil,
		Status:            0,
		CumulativeGasUsed: 0,
		Bloom:             types.Bloom{},
		Logs:              []*types.Log{},
		TxHash:            tx.Hash(),
		ContractAddress:   common.Address{},
		GasUsed:           0,
	}
}

func rewriteTxLookupIndex(db kaidb.Database, blockHash common.Hash, blockHeight uint64, blockTxs types.Transactions) error {
	for i, tx := range blockTxs {
		entry := kvstore.TxLookupEntry{
			BlockHash:  blockHash,
			BlockIndex: blockHeight,
			Index:      uint64(i),
		}
		data, err := rlp.EncodeToBytes(entry)
		if err != nil {
			fmt.Printf("Failed to encode transaction lookup entry: %v\n", err)
		}
		if err := db.Put(kvstore.TxLookupKey(tx.Hash()), data); err != nil {
			fmt.Printf("Failed to store transaction lookup entry %v\n", err)
		}
	}
	return nil
}

func insertReceipts(a types.Receipts, index uint64, value *types.Receipt) types.Receipts {
	if uint64(len(a)) == index { // nil or empty slice or after last element
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}
