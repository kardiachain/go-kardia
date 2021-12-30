package main

import (
	"flag"
	"fmt"

	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"gopkg.in/urfave/cli.v1"
)

var (
	rollbackStateCmd = cli.Command{
		Name:  "rollback",
		Usage: "rollback kardiachain state by one height",
		Action: func(ctx *cli.Context) error {
			flag.Parse()
			cfg, err := LoadConfig(args)
			if err != nil {
				return err
			}

			height, hash, err := RollbackState(cfg)
			if err != nil {
				return fmt.Errorf("failed to rollback state: %w", err)
			}

			fmt.Printf("Rolled back state to height %d and hash %X", height, hash)
			return nil
		},
	}
)

func RollbackState(cfg *Config) (uint64, common.Hash, error) {
	// use the parsed config to load the block and state store
	blockStore, stateStore, err := loadStateAndBlockStore(cfg)
	if err != nil {
		return 0, common.Hash{}, err
	}
	// rollback the last state
	return cstate.Rollback(blockStore, stateStore)
}

func loadStateAndBlockStore(cfg *Config) (cstate.BlockStore, cstate.Store, error) {
	nodeCfg, err := cfg.getNodeConfig()
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.NewLevelDBDatabase(nodeCfg.ResolvePath("chaindata"), 16, 32, "chaindata")
	if err != nil {
		return nil, nil, err
	}
	stateDB := cstate.NewStore(db.DB())

	bc, err := blockchain.NewBlockChain(log.NewNopLogger(), db, cfg.getChainConfig())
	if err != nil {
		return nil, nil, err
	}
	bOper := blockchain.NewBlockOperations(log.NewNopLogger(), bc, nil, nil, nil)
	return bOper, stateDB, nil
}
