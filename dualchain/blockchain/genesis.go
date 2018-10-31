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

package blockchain

import (
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/chaindb"
	"github.com/kardiachain/go-kardia/kai/state"
	kaidb "github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

// Genesis specifies the header fields, state of a genesis block.
type DualGenesis struct {
	Config    *configs.ChainConfig `json:"config"`
	Timestamp uint64               `json:"timestamp"`

	// TODO(huny@): Add default validators?
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The returned chain configuration is never nil.
func SetupGenesisBlock(logger log.Logger, db kaidb.Database, genesis *DualGenesis) (*configs.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		// TODO(huny@): should we return another default config?
		return configs.TestnetChainConfig, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := chaindb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			logger.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			logger.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(logger, db)
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		logger.Info("Create new genesis block")
		hash := genesis.ToBlock(logger, nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	storedcfg := chaindb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		logger.Warn("Found genesis block without chain config")
		chaindb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != configs.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	chaindb.WriteChainConfig(db, stored, newcfg)
	return newcfg, stored, nil
}

func (dg *DualGenesis) configOrDefault(ghash common.Hash) *configs.ChainConfig {
	switch {
	case dg != nil:
		return dg.Config
	case ghash == configs.MainnetGenesisHash:
		return configs.MainnetChainConfig
	case ghash == configs.TestnetGenesisHash:
		return configs.TestnetChainConfig
	default:
		return configs.TestnetChainConfig
	}
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (dg *DualGenesis) ToBlock(logger log.Logger, db kaidb.Database) *types.Block {
	if db == nil {
		db = kaidb.NewMemStore()
	}
	statedb, _ := state.New(logger, common.Hash{}, state.NewDatabase(db))

	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		//@huny: convert timestamp here
		// Time:           g.Timestamp,
		Root: root,
	}
	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	return types.NewBlock(logger, head, nil, nil, &types.Commit{})
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (dg *DualGenesis) Commit(logger log.Logger, db kaidb.Database) (*types.Block, error) {
	block := dg.ToBlock(logger, db)
	if block.Height() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with height > 0")
	}
	chaindb.WriteBlock(db, block)
	chaindb.WriteReceipts(db, block.Hash(), block.Height(), nil)
	chaindb.WriteCanonicalHash(db, block.Hash(), block.Height())
	chaindb.WriteHeadBlockHash(db, block.Hash())
	chaindb.WriteHeadHeaderHash(db, block.Hash())

	config := dg.Config
	if config == nil {
		config = configs.TestnetChainConfig
	}
	chaindb.WriteChainConfig(db, block.Hash(), config)

	return block, nil
}

// DefaultGenesisBlock returns the main net genesis block.
func DefaultGenesisBlock() *DualGenesis {
	return &DualGenesis{
		Config: configs.MainnetChainConfig,
	}
}
