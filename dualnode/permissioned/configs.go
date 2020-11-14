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

package permissioned

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardiachain/go-kardiamain/dev"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/node"
)

const (
	DefaultHTTPPort = 8000

	MainChainDataDir = "privatechain"
	DefaultDbCache   = 16 // 16MB memory allocated for leveldb cache, for each chains
	DefaultDbHandles = 32 // 32 file handlers allocated for leveldb, for each chains
	privateNetworkId = 110
)

type Config struct {
	Proposal          int
	Name              *string
	NetworkId         *uint64
	DataDir           *string
	HTTPPort          *int
	HTTPModules       []string
	HTTPVirtualHosts  []string
	HTTPCors          []string
	ListenAddr        *string
	ChainDataDir      *string
	DbCache           *int
	DbHandles         *int
	ValidatorsIndices *string
	ServiceName       *string
	ChainID           *uint64
	ClearData         bool
}

var DefaultConfig = node.Config{
	// todo: Uncomment and update config when we support dual node
	//DataDir:          node.DefaultDataDir(),
	HTTPPort:         DefaultHTTPPort,
	HTTPModules:      []string{"node", "kai", "tx", "account"},
	HTTPVirtualHosts: []string{"0.0.0.0", "localhost"},
	HTTPCors:         []string{"*"},
	// todo: uncomment update config when we support dual node
	//P2P: p2p.Config{
	//	ListenAddr: DefaultListenAddr,
	//	MaxPeers:   25,
	//},
	MainChainConfig: node.MainChainConfig{
		NetworkId: privateNetworkId,
		DBInfo:    storage.NewLevelDbInfo(MainChainDataDir, DefaultDbCache, DefaultDbHandles),
		AcceptTxs: 1, // 1 is to allow new transactions, 0 is not
		IsPrivate: true,
		IsZeroFee: true,
		// todo: uncomment and update config when we support dual node
		//Genesis:   genesis.DefaultTestnetFullGenesisBlock(configs.GenesisAccounts, configs.GenesisContracts),
	},
}

func SetUp(config *Config) (nodeConfig *node.Config, err error) {
	nodeConfig = &DefaultConfig
	if config == nil {
		return nodeConfig, nil
	}
	if config.DataDir != nil {
		nodeConfig.DataDir = *config.DataDir
	}
	// todo: uncomment update config when we support dual node
	//if config.ListenAddr != nil {
	//	nodeConfig.P2P.ListenAddr = *config.ListenAddr
	//}

	if config.ChainDataDir != nil && config.DbCache != nil && config.DbHandles != nil {
		nodeConfig.MainChainConfig.DBInfo = storage.NewLevelDbInfo(*config.ChainDataDir, *config.DbCache, *config.DbHandles)
	}

	if config.HTTPPort != nil {
		nodeConfig.HTTPPort = *config.HTTPPort
	}
	if config.HTTPModules != nil {
		nodeConfig.HTTPModules = config.HTTPModules
	}
	if config.HTTPVirtualHosts != nil {
		nodeConfig.HTTPVirtualHosts = config.HTTPVirtualHosts
	}
	if config.HTTPCors != nil {
		nodeConfig.HTTPCors = config.HTTPCors
	}
	if config.NetworkId != nil && *config.NetworkId > 0 {
		nodeConfig.MainChainConfig.NetworkId = *config.NetworkId
	}
	if config.ServiceName != nil {
		nodeConfig.MainChainConfig.ServiceName = *config.ServiceName
	}
	if config.ChainID != nil {
		nodeConfig.MainChainConfig.ChainId = *config.ChainID
	}

	// Check config.Name
	if config.Name == nil || len(*config.Name) == 0 {
		return nil, fmt.Errorf("node name must not be empty")
	}
	nodeConfig.Name = *config.Name

	index, err := node.GetNodeIndex(*config.Name)
	if err != nil {
		return nil, fmt.Errorf("node name must be formatted as \"\\c*\\d{1,2}\"")
	}
	nodeIndex := index - 1

	// Get NodeMetadata
	nodeConfig.NodeMetadata, _ = dev.GetNodeMetadataByIndex(nodeIndex)

	nodeDir := filepath.Join(nodeConfig.DataDir, nodeConfig.Name)
	if config.ClearData {
		err := removeDirContents(nodeDir)
		if err != nil {
			return nil, fmt.Errorf("cannot remove contents in dir %v / %v", nodeDir, err)
		}
	}

	nodeConfig.MainChainConfig.TxPool = *tx_pool.GetDefaultTxPoolConfig(nodeDir)
	return nodeConfig, nil
}

// removeDirContents deletes old local node directory
func removeDirContents(dir string) error {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}

	return nil
}
