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

package dual

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/user"
	"path/filepath"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/params"
	"github.com/kardiachain/go-kardia/lib/log"

	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/blockchain/dual"
	"github.com/kardiachain/go-kardia/dual/ethsmc"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// // headChannelSize is the size of channel listening to ChainHeadEvent.
	headChannelSize = 5
	// GenesisAccount used for matchEth tx
	genesisAccount = "0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28"
)

var DefaultEthKardiaConfig = EthKardiaConfig{
	Name:            "GethKardia", // Don't need to change, default instance name for geth is "geth".
	ListenAddr:      ":30303",
	MaxPeers:        10,
	LightNode:       false,
	LightPeers:      5,
	LightServ:       0,
	StatName:        "eth-kardia-1",
	ContractAddress: ethsmc.EthContractAddress,

	CacheSize: 1024,
}

// EthKardiaConfig provides configuration when starting Eth subnode.
type EthKardiaConfig struct {
	ContractAddress string // address of Eth smart contract to watch.

	// Network configs
	Name        string
	ListenAddr  string
	MaxPeers    int
	LightNode   bool   // Starts with light sync, otherwise starts with fast sync.
	LightPeers  int    // Max number of light peers.
	LightServ   int    // Max percentage of time allowed for serving LES requests (0-90)"
	ReportStats bool   // Reports node statistics to network centralized statistics collection system.
	StatName    string // Node name to use when report to Rinkeby stats collection.

	// Performance configs
	CacheSize int // Cache memory size in MB for database & trie. This must be small enough to leave enough memory for separate Kardia chain cache.
}

// EthKarida is a full Ethereum node running inside Karida
type EthKardia struct {
	geth        *node.Node
	config      *EthKardiaConfig
	ethSmc      *ethsmc.EthSmc
	kardiaChain *blockchain.BlockChain
	txPool      *blockchain.TxPool // Transaction pool of KARDIA service.
	eventPool   *dual.EventPool    // Event pool of DUAL service.
}

// EthKardia creates a Ethereum sub node.
func NewEthKardia(config *EthKardiaConfig, kardiaChain *blockchain.BlockChain, txPool *blockchain.TxPool, dualEventPool *dual.EventPool) (*EthKardia, error) {
	datadir := defaultEthDataDir()

	// Creates datadir with testnet follow eth standards.
	// TODO(thientn) : options to choose different networks.
	datadir = filepath.Join(datadir, "rinkeby")
	bootUrls := params.RinkebyBootnodes
	bootstrapNodes := make([]*discover.Node, 0, len(bootUrls))
	bootstrapNodesV5 := make([]*discv5.Node, 0, len(bootUrls)) // rinkeby set default bootnodes as also discv5 nodes.
	for _, url := range bootUrls {
		peer, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodes = append(bootstrapNodes, peer)

		peerV5, err := discv5.ParseNode(url)
		if err != nil {
			log.Error("BootstrapV5 URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodesV5 = append(bootstrapNodesV5, peerV5)
	}

	// similar to utils.SetNodeConfig
	nodeConfig := &node.Config{
		DataDir: datadir,
		IPCPath: "geth.ipc",
		Name:    config.Name,
	}

	// similar to utils.SetP2PConfig
	nodeConfig.P2P = p2p.Config{
		BootstrapNodes:   bootstrapNodes,
		ListenAddr:       config.ListenAddr,
		MaxPeers:         config.MaxPeers,
		DiscoveryV5:      config.LightNode, // Force using discovery if light node, as in flags.go.
		BootstrapNodesV5: bootstrapNodesV5,
	}

	// similar to cmd/eth/config.go/makeConfigNode
	ethConf := &eth.DefaultConfig
	ethConf.NetworkId = 4 // Rinkeby Id
	ethConf.Genesis = core.DefaultRinkebyGenesisBlock()

	ethConf.LightServ = config.LightServ
	ethConf.LightPeers = config.LightPeers

	// similar to cmd/utils/flags.go
	ethConf.DatabaseCache = config.CacheSize * 75 / 100
	ethConf.TrieCache = config.CacheSize * 25 / 100
	// Hardcode to 50% of 2048 file descriptor limit for whole process, as in flags.go/makeDatabaseHandles()
	ethConf.DatabaseHandles = 1024

	// Creates new node.
	ethNode, err := node.New(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("protocol node: %v", err)
	}
	if config.LightNode {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return les.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	} else {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return eth.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	}

	// Registers ethstats service to report node stat to testnet system.
	if config.ReportStats {
		url := fmt.Sprintf("[EthKardia]%s:Respect my authoritah!@stats.rinkeby.io", config.StatName)
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
	return &EthKardia{
		geth: ethNode, config: config, ethSmc: ethsmc.NewEthSmc(), kardiaChain: kardiaChain, txPool: txPool, eventPool: dualEventPool}, nil
}

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

// Start starts the Ethereum node.
func (n *EthKardia) Start() error {
	err := n.geth.Start()

	if err != nil {
		return err
	}
	go n.syncHead()
	return nil
}

// Stop shut down the Ethereum node.
func (n *EthKardia) Stop() error {
	return n.geth.Stop()
}

// EthNode returns the standard Eth Node.
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

func (n *EthKardia) BlockChain() *core.BlockChain {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.BlockChain()
}

func (n *EthKardia) TxPool() *core.TxPool {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.TxPool()
}

// syncHead syncs with latest events from Eth network to Kardia.
func (n *EthKardia) syncHead() {
	var ethService *eth.Ethereum

	n.geth.Service(&ethService)

	if ethService == nil {
		log.Error("Not implement dual sync for Eth light mode yet")
		return
	}

	ethChain := ethService.BlockChain()

	chainHeadEventCh := make(chan core.ChainHeadEvent, headChannelSize)
	headSubCh := ethChain.SubscribeChainHeadEvent(chainHeadEventCh)
	defer headSubCh.Unsubscribe()

	blockCh := make(chan *ethTypes.Block, 1)

	// Follow other examples.
	// Listener to exhaust extra event while sending block to our channel.
	go func() {
	ListenerLoop:
		for {
			select {
			// Gets chain head events, drop if overload.
			case head := <-chainHeadEventCh:
				select {
				case blockCh <- head.Block:
					// Block field would be nil here.
				default:
					// TODO(thientn): improves performance/handling here.
				}
			case <-headSubCh.Err():
				break ListenerLoop
			}
		}
	}()

	// Handler loop for new blocks.
	for {
		select {
		case block := <-blockCh:
			if !n.config.LightNode {
				go n.handleBlock(block)
			}
		}
	}
}

func (n *EthKardia) handleBlock(block *ethTypes.Block) {
	// TODO(thientn): block from this event is not guaranteed newly update. May already handled before.

	// Some events has nil block.
	if block == nil {
		// TODO(thientn): could call blockchain.CurrentBlock() here.
		log.Info("handleBlock with nil block")
		return
	}

	header := block.Header()
	txns := block.Transactions()

	log.Info("handleBlock...", "header", header, "txns size", len(txns))

	/* Can be use to check contract state, but currently has memory leak.
	b := n.BlockChain()
	state, err := b.State()
	if err != nil {
		log.Error("Get Geth state() error", "err", err)
		return
	}
	*/

	contractAddr := ethCommon.HexToAddress(n.config.ContractAddress)

	for _, tx := range block.Transactions() {
		// TODO(thientn): Make this tx matcher more robust.
		if tx.To() != nil && *tx.To() == contractAddr {
			log.Info("New tx detected on smart contract", "addr", contractAddr.Hex(), "value", tx.Value())
			statedb, err := n.BlockChain().State()
			if err != nil {
				log.Error("Fail to get DUAL's service statedb", "err", err)
				return
			}
			nonce := statedb.GetNonce(ethCommon.HexToAddress(dual.DualStateAddressHex))
			ethTxHash := tx.Hash()
			txHash := common.BytesToHash(ethTxHash[:])
			eventSummary, err := n.ExtractEthTxSummary(tx)
			if err != nil {
				log.Error("Error when extracting Eth's tx summary.")
				// TODO(#140): Handle smart contract failure correctly.
				panic("Not yet implemented!")
			}
			dualEvent := types.NewDualEvent(nonce, "ETH", &txHash, &eventSummary)

			// TODO(namdoh@): Move the creation of Kardia Tx to under a util func.
			// Compute Kardia's tx from the Eth's event.
			kardiaStateDB, err := n.kardiaChain.State()
			if err != nil {
				log.Error("Fail to get Kardia state", "error", err)
				return
			}
			addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[genesisAccount])
			addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
			// TODO(namdoh@): Pass eventSummary.TxSource to matchType.
			kardiaTx := CreateKardiaMatchAmountTx(addrKey, kardiaStateDB, eventSummary.TxValue, 1)
			dualEvent.PendingTx = types.TxData{
				TxHash: kardiaTx.Hash(),
				Target: "KARDIA",
			}

			log.Info("Create dual's event", "dualEvent", dualEvent)
			if err := n.eventPool.AddEvent(dualEvent); err != nil {
				log.Error("Fail to add dual's event", "error", err)
				return
			}
			log.Info("Add dual's event to event pool successfully", "eventHash", dualEvent.Hash().Hex())
		}
	}
}

func (n *EthKardia) ExtractEthTxSummary(tx *ethTypes.Transaction) (types.EventSummary, error) {
	input := tx.Data()
	method, err := n.ethSmc.InputMethodName(input)
	if err != nil {
		log.Error("Error when unpack Eth smc input", "error", err)
		return types.EventSummary{}, err
	}

	return types.EventSummary{
		TxMethod: method,
		TxValue:  tx.Value(),
	}, nil
}

func (n *EthKardia) SendEthFromContract(value *big.Int) {

	statedb, err := n.BlockChain().State()
	if err != nil {
		log.Error("Fail to get Ethereum state to create release tx", "err", err)
		return
	}
	// Nonce of account to sign tx
	contractAddr := ethCommon.HexToAddress(ethsmc.EthAccountSign)
	nonce := statedb.GetNonce(contractAddr)
	if nonce == 0 {
		log.Error("Eth state return 0 for nonce of contract address, SKIPPING TX CREATION", "addr", ethsmc.EthContractAddress)
	}
	tx := n.ethSmc.CreateEthReleaseTx(value, nonce)

	log.Info("Create Eth tx to release", "value", value, "nonce", nonce, "txhash", tx.Hash().Hex())
	if err := n.TxPool().AddLocal(tx); err != nil {
		log.Error("Fail to add Ether tx", "error", err)
	} else {
		log.Info("Add Eth release tx successfully", "txhash", tx.Hash().Hex())
	}
}
