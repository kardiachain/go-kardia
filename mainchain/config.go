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

package kai

import (
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
)

// Defaults contains default settings for use on the Kardia main net.
var Defaults = Config{
	SyncMode:                configs.FastSync,
	NetworkId:               24,
	TxLookupLimit:           2350000,
	DatabaseCache:           512,
	TrieCleanCache:          154,
	TrieCleanCacheJournal:   "triecache",
	TrieCleanCacheRejournal: 60 * time.Minute,
	TrieDirtyCache:          256,
	TrieTimeout:             60 * time.Minute,
	SnapshotCache:           102,
	TxPool:                  tx_pool.DefaultTxPoolConfig,
	AcceptTxs:               true,
	GasOracle:               oracles.DefaultOracleConfig(),
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Kardia main net block is used.
	Genesis *genesis.Genesis `toml:"-"`

	// Protocol options
	ChainId   *big.Int         `toml:",omitempty"`
	NetworkId uint64           `toml:",omitempty"`
	SyncMode  configs.SyncMode `toml:",omitempty"`

	NoPruning  bool // Whether to disable pruning and flush everything to disk
	NoPrefetch bool // Whether to disable prefetching and only load state on deman

	TxLookupLimit uint64 `toml:",omitempty"` // The maximum number of blocks from head whose tx indices are reserved.

	DatabaseCache int `toml:",omitempty"`

	TrieCleanCache          int           `toml:",omitempty"`
	TrieCleanCacheJournal   string        `toml:",omitempty"` // Disk journal directory for trie cache to survive node restarts
	TrieCleanCacheRejournal time.Duration `toml:",omitempty"` // Time interval to regenerate the journal for clean cache
	TrieDirtyCache          int           `toml:",omitempty"`
	TrieTimeout             time.Duration `toml:",omitempty"`
	SnapshotCache           int           `toml:",omitempty"`
	Preimages               bool          `toml:",omitempty"`

	// Transaction pool options
	TxPool tx_pool.TxPoolConfig `toml:",omitempty"`

	// DbInfo stores configuration information to setup database
	DBInfo rawdb.DbInfo `toml:",omitempty"`

	// acceptTxs accept tx sync processes
	AcceptTxs bool `toml:",omitempty"`

	// ServiceName is used to display as log's prefix
	ServiceName string `toml:",omitempty"`

	// Consensus defines the configuration for the Kardia consensus service,
	// including timeouts and details about the block structure.
	Consensus *configs.ConsensusConfig `toml:",omitempty"`

	FastSync *configs.FastSyncConfig `toml:",omitempty"`

	GasOracle *oracles.Config `toml:",omitempty"`
}
