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

package eth

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualnode/eth/ethsmc"
)

var DefaultEthConfig = EthConfig{
	Name:            "GethKardia", // Don't need to change, default instance name for geth is "geth".
	ListenAddr:      ":30303",
	MaxPeers:        10,
	NetworkId:       4,     // 4: rinkeby, 3: ropsten, 1: mainnet
	LightNode:       false, // Need Eth full node to support dual node mechanism.
	LightPeers:      5,
	LightServ:       0,
	StatName:        "eth-kardia-1",
	ContractAddress: ethsmc.EthContractAddress,

	HTTPHost:         "0.0.0.0", // Default host interface for the HTTP RPC server
	HTTPPort:         8546,      // Default TCP port for the HTTP RPC server
	HTTPVirtualHosts: []string{"0.0.0.0", "localhost"},
	HTTPCors:         []string{"*"},

	CacheSize: 1024,
}

var EthDualChainID = uint64(2)

// defaultEthDataDir returns default Eth root datadir.
func defaultEthDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home == "" {
		panic("Fail to get OS home directory")
	}
	return filepath.Join(home, ".ethereum")
}

// Copy from go-kardia/node
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// EthConfig provides configuration when starting Eth subnode.
type EthConfig struct {
	ContractAddress string // address of Eth smart contract to watch.

	// Network configs
	Name        string
	ListenAddr  string
	MaxPeers    int
	NetworkId   int    // 4: rinkeby, 3: ropsten, 1: mainnet
	LightNode   bool   // Starts with light sync, otherwise starts with fast sync.
	LightPeers  int    // Max number of light peers.
	LightServ   int    // Max percentage of time allowed for serving LES requests (0-90)"
	ReportStats bool   // Reports node statistics to network centralized statistics collection system.
	StatName    string // Node name to use when report to Rinkeby stats collection.

	// RPC settings
	HTTPHost         string
	HTTPPort         int
	HTTPVirtualHosts []string
	HTTPCors         []string

	// Performance configs
	CacheSize int // Cache memory size in MB for database & trie. This must be small enough to leave enough memory for separate Kardia chain cache.

	// ======== DEV ENVIRONMENT CONFIG =========
	// Configuration of this node when running in dev environment.
	DualNodeConfig *dev.DualNodeConfig
}
