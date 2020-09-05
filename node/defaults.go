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
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/p2p/nat"
	"github.com/kardiachain/go-kardiamain/rpc"
)

const (
	DefaultHTTPHost    = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort    = 8545        // Default TCP port for the HTTP RPC server
	DefaultWSHost      = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort      = 8546        // Default TCP port for the websocket RPC server
	DefaultGraphQLHost = "localhost" // Default host interface for the GraphQL server
	DefaultGraphQLPort = 8547        // Default TCP port for the GraphQL server

	DefaultDbCache   = 16 // 16MB memory allocated for leveldb cache, for each chains
	DefaultDbHandles = 32 // 32 file handlers allocated for leveldb, for each chains

	MainChainDataDir = "chaindata" // directory of database storage for main chain data
	DualChainDataDir = "dualdata"  // directory of database storage for dual chain data

	DefaultNetworkID  = 100
	MainChainID       = 1
	KardiaServiceName = "KARDIA"
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = Config{
	DataDir:             DefaultDataDir(),
	HTTPPort:            DefaultHTTPPort,
	HTTPModules:         []string{"node", "kai", "tx", "account", "dual", "neo"},
	HTTPVirtualHosts:    []string{"0.0.0.0", "localhost"},
	HTTPCors:            []string{"*"},
	HTTPTimeouts:        rpc.DefaultHTTPTimeouts,
	WSPort:              DefaultWSPort,
	WSModules:           []string{"node", "kai", "tx", "account", "dual", "neo"},
	GraphQLPort:         DefaultGraphQLPort,
	GraphQLVirtualHosts: []string{"localhost"},
	P2P: p2p.Config{
		ListenAddr: ":30303",
		MaxPeers:   50,
		NAT:        nat.Any(),
	},
	MainChainConfig: MainChainConfig{
		ServiceName: KardiaServiceName,
		ChainId:     MainChainID,
		NetworkId:   DefaultNetworkID,
		AcceptTxs:   1, // 1 is to allow new transactions, 0 is not
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
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Ethereum")
		case "windows":
			// We used to put everything in %HOME%\AppData\Roaming, but this caused
			// problems with non-typical setups. If this fallback location exists and
			// is non-empty, use it, otherwise DTRT and check %LOCALAPPDATA%.
			fallback := filepath.Join(home, "AppData", "Roaming", "Ethereum")
			appdata := windowsAppData()
			if appdata == "" || isNonEmptyDir(fallback) {
				return fallback
			}
			return filepath.Join(appdata, "Ethereum")
		default:
			return filepath.Join(home, ".ethereum")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func windowsAppData() string {
	v := os.Getenv("LOCALAPPDATA")
	if v == "" {
		// Windows XP and below don't have LocalAppData. Crash here because
		// we don't support Windows XP and undefining the variable will cause
		// other issues.
		panic("environment variable LocalAppData is undefined")
	}
	return v
}

func isNonEmptyDir(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	names, _ := f.Readdir(1)
	f.Close()
	return len(names) > 0
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
