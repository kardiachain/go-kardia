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
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/rpc"
)

const (
	DefaultHTTPHost = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 8545        // Default TCP port for the HTTP RPC server
	DefaultWSHost   = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort   = 8546        // Default TCP port for the websocket RPC server

	DefaultGraphQLPort = 8547 // Default TCP port for the GraphQL server

	DefaultDbCache   = 16 // 16MB memory allocated for leveldb cache, for each chains
	DefaultDbHandles = 32 // 32 file handlers allocated for leveldb, for each chains

	DualChainDataDir = "dualdata" // directory of database storage for dual chain data

	DefaultNetworkID  = 100
	MainChainID       = 1
	KardiaServiceName = "KARDIA"
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = Config{
	DataDir:             configs.DefaultDataDir(),
	HTTPPort:            DefaultHTTPPort,
	HTTPModules:         []string{"node", "kai", "tx", "account", "dual", "neo"},
	HTTPVirtualHosts:    []string{"0.0.0.0", "localhost"},
	HTTPCors:            []string{"*"},
	HTTPTimeouts:        rpc.DefaultHTTPTimeouts,
	WSPort:              DefaultWSPort,
	WSModules:           []string{"node", "kai", "tx", "account", "dual", "neo"},
	GraphQLPort:         DefaultGraphQLPort,
	GraphQLVirtualHosts: []string{"localhost"},
	P2P:                 configs.DefaultP2PConfig(),
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
