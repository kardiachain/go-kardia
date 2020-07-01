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

package node

import (
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"os"
	"os/user"
	"path/filepath"

	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/p2p/nat"
)

const (
	DefaultHTTPHost = "0.0.0.0" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 8545      // Default TCP port for the HTTP RPC server

	DefaultDbCache   = 16 // 16MB memory allocated for leveldb cache, for each chains
	DefaultDbHandles = 32 // 32 file handlers allocated for leveldb, for each chains

	MainChainDataDir = "chaindata" // directory of database storage for main chain data
	DualChainDataDir = "dualdata"  // directory of database storage for dual chain data

	DefaultNetworkID  = 100
	MainChainID = 1
	KardiaServiceName = "KARDIA"
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = NodeConfig{
	DataDir:          DefaultDataDir(),
	HTTPPort:         DefaultHTTPPort,
	HTTPModules:      []string{"node", "kai", "tx", "account", "dual", "neo"},
	HTTPVirtualHosts: []string{"0.0.0.0", "localhost"},
	HTTPCors:         []string{"*"},
	P2P: p2p.Config{
		ListenAddr: ":30303",
		MaxPeers:   25,
		NAT:        nat.Any(),
	},
	MainChainConfig: MainChainConfig{
		ServiceName:  KardiaServiceName,
		ChainId:      MainChainID,
		NetworkId:    DefaultNetworkID,
		AcceptTxs:    1, // 1 is to allow new transactions, 0 is not
	},
	DualChainConfig: DualChainConfig{
		DBInfo: storage.NewLevelDbInfo(DualChainDataDir, DefaultDbCache, DefaultDbHandles),
	},
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		return filepath.Join(home, ".kardia")

		// TODO: may need to handle non-unix OS.
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
