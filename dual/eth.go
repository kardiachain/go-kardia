package dual

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/params"
	"github.com/kardiachain/go-kardia/log"
)

const (
	NodeName     = "GethKardia" // Client for Eth network
	NodePort     = 30303
	NodeMaxPeers = 4 // Eth max peers
)

// EthKarida is a full Ethereum node running inside Karida
type EthKardia struct {
	geth *node.Node
}

// DefaultEthDataDir is the default data directory for Ethereum.
func DefaultEthDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		return filepath.Join(home, ".ethereum")

		// TODO: may need to handle non-unix OS.
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
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

// EthKardia creates a Ethereum node with
func NewEthKardia(lightNode bool, reportStat bool) (*EthKardia, error) {
	datadir := DefaultEthDataDir()

	// Creates datadir with testnet follow eth standards.
	// TODO(thientn) : options to choose different networks.
	datadir = filepath.Join(datadir, "rinkeby")
	bootUrls := params.RinkebyBootnodes
	bootstrapNodes := make([]*discover.Node, 0, len(bootUrls))
	for _, url := range bootUrls {
		node, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodes = append(bootstrapNodes, node)
	}

	// similar to utils.SetNodeConfig
	nodeConfig := &node.Config{
		DataDir: datadir,
		IPCPath: "geth.ipc",
		Name:    NodeName,
	}

	// similar to utils.SetP2PConfig
	nodeConfig.P2P = p2p.Config{
		BootstrapNodes: bootstrapNodes,
		ListenAddr:     fmt.Sprintf(":%d", NodePort),
		MaxPeers:       NodeMaxPeers,
	}

	// TODO(thientn): set eth config to match with Rinkeby or other test networks.
	// verify on cmd/utils/flags.go
	// similar to cmd/eth/config.go/makeConfigNode
	ethConf := &eth.DefaultConfig
	// By default memory cache is 1GB, 75% db, 25% trie
	// Tests increase it to 2GB
	ethConf.DatabaseCache = ethConf.DatabaseCache * 2
	ethConf.TrieCache = ethConf.TrieCache * 2
	ethConf.NetworkId = 4 // Rinkeby Id
	ethConf.Genesis = core.DefaultRinkebyGenesisBlock()

	ethNode, err := node.New(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("protocol node: %v", err)
	}
	if lightNode {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return les.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	} else {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return eth.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	}

	// Registers ethstats service to report node stat to testnet system.
	if reportStat {
		url := "[EthKardia]eth-kardia-1:Respect my authoritah!@stats.rinkeby.io"
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			// Retrieve both eth and les services
			var ethServ *eth.Ethereum
			ctx.Service(&ethServ)

			var lesServ *les.LightEthereum
			ctx.Service(&lesServ)

			return ethstats.New(url, ethServ, lesServ)
		}); err != nil {
			log.Error("Failed to register the Ethereum Stats service", "err", err)
		}
	}
	return &EthKardia{geth: ethNode}, nil
}

// Start starts the Ethereum node.
func (n *EthKardia) Start() error {
	return n.geth.Start()
}

// Stop shut down the Ethereum node.
func (n *EthKardia) Stop() error {
	return n.geth.Stop()
}

// GethNode returns the standard Eth Node.
func (n *EthKardia) EthNode() *node.Node {
	return n.geth
}

// Client return the KardiaEthClient to acess Eth subnode.
func (n *EthKardia) Client() (*KardiaEthClient, error) {
	rpcClient, err := n.geth.Attach()
	if err != nil {
		return nil, err
	}
	return &KardiaEthClient{ethClient: ethclient.NewClient(rpcClient), stack: n.geth}, nil
}
