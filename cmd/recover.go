package main

import (
	"flag"
	"fmt"

	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
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
	removeTxLookupIndexCmd = cli.Command{
		Name:  "remove-lookup-index",
		Usage: "Recover all TxLookupIndex in database",
		Action: func(ctx *cli.Context) error {
			flag.Parse()
			cfg, err := LoadConfig(args)
			if err != nil {
				return err
			}

			err = removeTxLookupEntry(cfg)
			if err != nil {
				return fmt.Errorf("failed to remove TxLookupIndex: %w", err)
			}

			return nil
		},
	}
)

func removeTxLookupEntry(cfg *Config) error {
	// use the parsed config to load the storeDB
	store, err := loadStoreDB(cfg)
	if err != nil {
		return err
	}
	var block *types.Block
	fmt.Printf("Removing TxLookupIndex...\n")
	for i := cfg.Debug.FromBlock; i <= cfg.Debug.EndBlock; i++ {
		if i%100000 == 0 {
			fmt.Printf("Removing TxLookupIndex in block %v...\n", i)
		}
		block = store.ReadBlock(i)
		txs := block.Transactions()
		for i := range txs {
			if err = store.DeleteTxLookupEntry(txs[i].Hash()); err != nil {
				fmt.Printf("Error while removing tx %v, block %d, err: %v\n", txs[i].Hash(), i, err)
				return err
			}
		}
	}
	return nil
}

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
	fmt.Printf("Processing...\n")
	for i := cfg.Debug.FromBlock; i <= cfg.Debug.EndBlock; i++ {
		if i%100000 == 0 {
			fmt.Printf("Processing block %v...\n", i)
		}
		block = store.ReadBlock(i)
		bi = store.ReadBlockInfo(block.Hash(), block.Height())
		if err = processBlockInfo(store, block, bi); err != nil {
			fmt.Printf("Error while processing block %v... err: %v\n", i, err)
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
	if block.NumTxs() == 0 {
		// skip empty blocks
		return nil
	}

	if bi == nil || block.NumTxs() == uint64(len(bi.Receipts)) {
		// Normal block detected
		// UPDATE TxLookupIndex of every good tx which also has the corresponding receipt on the way because we deleted it all
		store.WriteTxLookupEntries(block)
		return nil
	}

	// Bad block detected
	// When we meet a bad tx:
	//    1. if there is a correct tx receipt before: insert the whole transaction and update the bad blockInfo; NOT UPDATE TxLookupIndex
	//    2. if there isn't a correct tx receipt: insert a fake one and update the blockInfo; NOT UPDATE TxLookupIndex
	// Need to UPDATE TxLookupIndex one-by-one, not in batch because the whole block tx entries will be updated and
	// overwrite the correct TxLookupIndex before
	txs := types.Transactions{}
	receipts := bi.Receipts
	for i := uint64(0); i < block.NumTxs(); i++ {
		if r, _ := getReceiptInList(block.Transactions()[i].Hash(), receipts); r != nil {
			// a correct tx  which has the corresponding receipt within the block
			txs = append(txs, block.Transactions()[i])
			// update the correct TxLookupIndex like usual
			store.WriteTxLookupEntry(block.Hash(), block.Height(), block.Transactions()[i].Hash(), i)
			continue
		}
		// caught a bad tx since it doesn't have the corresponding receipt within the block
		// trying to retrieve if there is any correct tx before
		tx, blockHash, blockHeight, index := store.ReadTransaction(block.Transactions()[i].Hash())
		if tx == nil {
			// there isn't a correct tx before, inserting a fake one and not update the TxLookupIndex
			fmt.Printf("Inserting fake receipt of a bad tx, hash: %v, block height %v\n", block.Transactions()[i].Hash().Hex(), block.Height())
			badReceipt := reconstructBadReceipt(block.Transactions()[i])
			receipts = insertReceipts(receipts, i, badReceipt)
		} else {
			// there is a correct tx before, replicating the receipt and not update the TxLookupIndex
			correctBi := store.ReadBlockInfo(blockHash, blockHeight)
			correctReceipt, _ := getReceiptInList(block.Transactions()[i].Hash(), correctBi.Receipts)
			if correctReceipt != nil {
				fmt.Printf("Correcting receipt of a bad tx, hash: %v, wrong block height %v, correct block height %v\n", block.Transactions()[i].Hash().Hex(), block.Height(), blockHeight)
				receipts = insertReceipts(receipts, i, correctReceipt)
			} else {
				fmt.Printf("WARNING! Tx has entry but not found. Hash %s, blockHash %v, blockHeight %d, index %d\n",
					block.Transactions()[i].Hash().Hex(), blockHash.Hex(), blockHeight, index)
			}
		}
	}
	// update the receipts inside blockInfo
	bi.Receipts = receipts
	store.WriteBlockInfo(block.Hash(), block.Height(), bi)
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

func insertReceipts(a types.Receipts, index uint64, value *types.Receipt) types.Receipts {
	if uint64(len(a)) >= index { // nil or empty slice or after last element
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}

func getReceiptInList(txHash common.Hash, receipts types.Receipts) (*types.Receipt, uint64) {
	for i := 0; i < receipts.Len(); i++ {
		if receipts[i].TxHash.Equal(txHash) {
			return receipts[i], uint64(i)
		}
	}
	return nil, 0
}
