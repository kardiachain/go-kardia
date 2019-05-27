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

package main

import (
	"flag"
	"fmt"
	"github.com/kardiachain/go-kardia/tool"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dev"
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	dualservice "github.com/kardiachain/go-kardia/dualchain/service"
	"github.com/kardiachain/go-kardia/dualnode/eth"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/dualnode/neo"
	"github.com/kardiachain/go-kardia/dualnode/permissioned"
	"github.com/kardiachain/go-kardia/dualnode/tron"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	"github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
)

// args
type flagArgs struct {
	logLevel string
	logTag   string

	// Kardia node's related flags
	name                string
	listenAddr          string
	maxPeers            int
	rpcEnabled          bool
	rpcAddr             string
	rpcPort             int
	bootNode            string
	peer                string
	clearDataDir        bool
	mainChainValIndexes string
	isZeroFee           bool
	isPrivate           bool
	networkId           uint64
	chainId             uint64
	serviceName         string
	noProxy             bool
	peerProxyIP         string

	// Ether/Kardia dualnode related flags
	ethDual       bool
	ethNetworkId  int
	ethStat       bool
	ethStatName   string
	ethLogLevel   string
	ethListenAddr string
	ethLightServ  int
	ethRPCPort    int

	// Neo/Kardia dualnode related flags
	neoDual bool

	// TRON dualnode
	tronDual bool

	// Private/Kardia dualnode related flags
	privateNetworkId   uint64
	privateValIndexes  string
	privateNodeName    string
	privateChainId     uint64
	privateServiceName string
	privateAddr        string

	// Dualnode's related flags
	dualProtocolName    string
	dualChain           bool
	dualChainValIndexes string
	isPrivateDual       bool
	dualNetworkId       uint64
	publishedEndpoint   string
	subscribedEndpoint  string

	// Development's related flags
	dev            bool
	proposal       int
	votingStrategy string
	mockDualEvent  bool
	devDualChainID uint64
	txs            bool
	txsDelay       int
	numTxs         int
	dualEvent      bool
}

var args flagArgs

func init() {
	flag.StringVar(&args.logLevel, "loglevel", "info", "minimum log verbosity to display")
	flag.StringVar(&args.logTag, "logtag", "", "filter logging records based on the tag value")

	// Node's related flags
	flag.StringVar(&args.name, "name", "", "Name of node")
	flag.StringVar(&args.listenAddr, "addr", ":30301", "listen address")
	flag.BoolVar(&args.rpcEnabled, "rpc", false, "whether to open HTTP RPC endpoints")
	flag.StringVar(&args.rpcAddr, "rpcaddr", "", "HTTP-RPC server listening interface")
	flag.IntVar(&args.rpcPort, "rpcport", node.DefaultHTTPPort, "HTTP-RPC server listening port")
	flag.StringVar(&args.bootNode, "bootNode", "", "Enode address of node that will be used by the p2p discovery protocol")
	flag.StringVar(&args.peer, "peer", "", "Comma separated enode URLs for P2P static peer")
	flag.BoolVar(&args.clearDataDir, "clearDataDir", false, "remove contents in data dir")
	flag.StringVar(&args.mainChainValIndexes, "mainChainValIndexes", "1,2,3", "Indexes of Main chain validators")
	flag.BoolVar(&args.isZeroFee, "zeroFee", false, "zeroFee is enabled then no gas is charged in transaction. Any gas that sender spends in a transaction will be refunded")
	flag.BoolVar(&args.isPrivate, "private", false, "private is true then peerId will be checked through smc to make sure that it has permission to access the chain")
	flag.Uint64Var(&args.networkId, "networkId", 0, "Your chain's networkId. NetworkId must be greater than 0")
	flag.Uint64Var(&args.chainId, "chainId", 0, "ChainID is used to validate which node is allowed to send message through P2P in the same blockchain")
	flag.StringVar(&args.serviceName, "serviceName", "", "ServiceName is used for displaying as log's prefix")

	// Dualnode's related flags
	flag.StringVar(&args.ethLogLevel, "ethloglevel", "warn", "minimum Eth log verbosity to display")
	flag.BoolVar(&args.ethDual, "dual", false, "whether to run in dual mode")
	flag.IntVar(&args.ethNetworkId, "ethNetworkId", 4, "run Eth network id, 4: rinkeby, 3: ropsten, 1: mainnet")
	flag.StringVar(&args.ethListenAddr, "ethAddr", ":30302", "listen address for eth")
	flag.BoolVar(&args.tronDual, "trondual", false, "whether to run TRON dual node")
	flag.BoolVar(&args.neoDual, "neodual", false, "whether to run NEO dual node")
	flag.BoolVar(&args.ethStat, "ethstat", false, "report eth stats to network")
	flag.StringVar(&args.ethStatName, "ethstatname", "", "name to use when reporting eth stats")
	flag.IntVar(&args.ethLightServ, "ethLightServ", 0, "max percentage of time serving Ethereum light client requests")
	flag.IntVar(&args.ethRPCPort, "ethRPCPort", eth.DefaultEthConfig.HTTPPort, "HTTP-RPC server listening port for Eth node. 8546 is the default port")
	flag.BoolVar(&args.dualChain, "dualchain", false, "run dual chain for group consensus")
	flag.StringVar(&args.dualChainValIndexes, "dualChainValIndexes", "", "Indexes of Dual chain validators")
	flag.BoolVar(&args.isPrivateDual, "privateDual", false, "privateDual is true then peerId will be checked through smc to make sure that it has permission to access the dualchain")
	flag.Uint64Var(&args.privateNetworkId, "privateNetworkId", 0, "Privatechain Network ID. Private Network ID must be greater than 0")
	flag.StringVar(&args.privateValIndexes, "privateValIndexes", "", "Indexes of private chain validators")
	flag.StringVar(&args.privateNodeName, "privateNodeName", "", "Name of private node")
	flag.Uint64Var(&args.privateChainId, "privateChainId", 0, "privateChainId is used to validate which node is allowed to send message through P2P in the private blockchain")
	flag.StringVar(&args.privateServiceName, "privateServiceName", "", "privateServiceName is used for displaying as log's prefix")
	flag.StringVar(&args.privateAddr, "privateAddr", ":5000", "listened address for private chain")
	flag.Uint64Var(&args.dualNetworkId, "dualNetworkId", 100, "dualNetworkID is used to differentiate amongst Kardia-based dual groups")
	flag.BoolVar(&args.noProxy, "noProxy", false, "When triggered, Kardia node is standalone and is not registered in proxy.")
	flag.StringVar(&args.peerProxyIP, "peerProxyIP", "", "IP of the peer proxy for this node to register.")
	flag.StringVar(&args.publishedEndpoint, "publishedEndpoint", "", "0MQ Endpoint that message will be published to")
	flag.StringVar(&args.subscribedEndpoint, "subscribedEndpoint", "", "0MQ Endpoint that dual node subscribes to get dual message.")
	flag.StringVar(&args.dualProtocolName, "dualProtocolName", "", "dualProtocolName is used to set protocol name for dual node.")

	// NOTE: The flags below are only applicable for dev environment. Please add the applicable ones
	// here and DO NOT add non-dev flags.
	flag.BoolVar(&args.dev, "dev", false, "deploy node with dev environment")
	flag.StringVar(&args.votingStrategy, "votingStrategy", "",
		"specify the voting script or strategy to simulate voting. Note that this flag only has effect when --dev flag is set")
	flag.IntVar(&args.proposal, "proposal", 1,
		"specify which node is the proposer. The index starts from 1, and every node needs to use the same proposer index."+
			" Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.mockDualEvent, "mockDualEvent",
		false, "generate fake dual events to trigger dual consensus. Note that this flag only has effect when --dev flag is set.")
	flag.IntVar(&args.maxPeers, "maxpeers", 25,
		"maximum number of network peers (network disabled if set to 0. Note that this flag only has effect when --dev flag is set")
	flag.Uint64Var(&args.devDualChainID, "devDualChainID", eth.EthDualChainID, "manually set dualchain ID. Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.txs, "txs", false, "generate random transfer txs")
	flag.IntVar(&args.txsDelay, "txsDelay", 10, "delay in seconds between batches of generated txs")
	flag.IntVar(&args.numTxs, "numTxs", 10, "number of of generated txs in one batch")
	flag.BoolVar(&args.dualEvent, "dualEvent", false, "generate initial dual event")
}

// runtimeSystemSettings optimizes process setting for go-kardia
func runtimeSystemSettings() error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	limit, err := sysutils.FDCurrent()
	if err != nil {
		return err
	}
	if limit < 2048 {
		if err := sysutils.FDRaise(2048); err != nil {
			return err
		}
	}
	return nil
}

// removeDirContents deletes old local node directory
func removeDirContents(dir string) error {
	log.Info("Remove directory", "dir", dir)
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("Directory does not exist", "dir", dir)
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
		if name == "rinkeby" || name == "ethereum" {
			continue
		}
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}

	return nil
}

// getIntArray converts string array to int array
func getIntArray(valIndex string) []int {
	valIndexArray := strings.Split(valIndex, ",")
	var a []int

	// keys - hashmap used to check duplicate inputs
	keys := make(map[string]bool)
	for _, stringVal := range valIndexArray {
		// if input is not seen yet
		if _, seen := keys[stringVal]; !seen {
			keys[stringVal] = true
			intVal, err := strconv.Atoi(stringVal)
			if err != nil {
				log.Error("Failed to convert string to int: ", err)
			}
			a = append(a, intVal-1)
		}
	}
	return a
}

func main() {
	flag.Parse()

	// Setups log to Stdout.
	level, err := log.LvlFromString(args.logLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		level = log.LvlInfo
	}
	if len(args.logTag) > 0 {
		log.Root().SetHandler(log.LvlAndTagFilterHandler(level, args.logTag,
			log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	} else {
		log.Root().SetHandler(log.LvlFilterHandler(level,
			log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	}
	logger := log.New()

	ethLogLevel, err := ethlog.LvlFromString(args.ethLogLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		ethLogLevel = ethlog.LvlInfo
	}
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(ethLogLevel, ethlog.StdoutHandler))

	// System settings
	if err := runtimeSystemSettings(); err != nil {
		logger.Error("Fail to update system settings", "err", err)
		return
	}

	var nodeIndex int
	if len(args.name) == 0 {
		logger.Error("Invalid node name", "name", args.name)
	} else {
		index, err := node.GetNodeIndex(args.name)
		if err != nil {
			logger.Error("Node name must be formatted as \"\\c*\\d{1,2}\"", "name", args.name)
		}
		nodeIndex = index - 1
	}

	// Setups config.
	config := &node.DefaultConfig
	config.P2P.ListenAddr = args.listenAddr
	config.Name = args.name

	// Setup bootNode
	if args.rpcEnabled {
		if config.HTTPHost = args.rpcAddr; config.HTTPHost == "" {
			config.HTTPHost = node.DefaultHTTPHost
		}
		config.HTTPPort = args.rpcPort
		config.HTTPVirtualHosts = []string{"*"} // accepting RPCs from all source hosts
	}

	if args.dev {
		config.MainChainConfig.EnvConfig = node.NewEnvironmentConfig()
		// Set P2P max peers for testing on dev environment
		config.P2P.MaxPeers = args.maxPeers
		if nodeIndex < 0 {
			logger.Error(fmt.Sprintf("Node index %v must greater than 0", nodeIndex+1))
		}
		// Subtract 1 from the index because we specify node starting from 1 onward.
		config.MainChainConfig.EnvConfig.SetProposerIndex(args.proposal-1, len(dev.Nodes))
		// Only set DevNodeConfig if this is a known node from Kardia default set
		if nodeIndex < len(dev.Nodes) {
			nodeMetadata, err := dev.GetNodeMetadataByIndex(nodeIndex)
			if err != nil {
				logger.Error("Cannot get node by index", "err", err)
			}
			config.NodeMetadata = nodeMetadata
		}
		// Simulate the voting strategy
		config.MainChainConfig.EnvConfig.SetVotingStrategy(args.votingStrategy)
		config.MainChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)

		// Create genesis block with dev.genesisAccounts
		config.MainChainConfig.Genesis = genesis.DefaulTestnetFullGenesisBlock(configs.GenesisAccounts, configs.GenesisContracts)
	}
	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.MainChainConfig.TxPool = *tx_pool.GetDefaultTxPoolConfig(nodeDir)
	config.MainChainConfig.IsZeroFee = args.isZeroFee
	config.MainChainConfig.IsPrivate = args.isPrivate

	if args.networkId > 0 {
		config.MainChainConfig.NetworkId = args.networkId
	}
	if args.chainId > 0 {
		config.MainChainConfig.ChainId = args.chainId
	}
	if args.serviceName != "" {
		config.MainChainConfig.ServiceName = args.serviceName
	}
	if args.clearDataDir {
		// Clear all contents within data dir
		err := removeDirContents(nodeDir)
		if err != nil {
			logger.Error("Cannot remove contents in directory", "dir", nodeDir, "err", err)
			return
		}
	}
	config.PeerProxyIP = args.peerProxyIP

	n, err := node.NewNode(config)
	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	n.RegisterService(kai.NewKardiaService)
	if args.dualChain {
		if args.dev {
			// Set env config for dualchain config
			config.DualChainConfig.EnvConfig = node.NewEnvironmentConfig()
			// Subtract 1 from the index because we specify node starting from 1 onward.
			config.MainChainConfig.EnvConfig.SetProposerIndex(args.proposal-1, len(dev.Nodes))
			config.DualChainConfig.DualGenesis = genesis.DefaulTestnetFullGenesisBlock(configs.GenesisAccounts, configs.GenesisContracts)
		}

		if len(args.dualChainValIndexes) > 0 {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.dualChainValIndexes)
		} else {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)
		}
		config.DualChainConfig.DualEventPool = *event_pool.GetDefaultEventPoolConfig(nodeDir)
		config.DualChainConfig.IsPrivate = args.isPrivateDual
		config.DualChainConfig.ChainId = args.devDualChainID
		config.DualChainConfig.DualNetworkID = args.dualNetworkId
		if args.ethDual {
			config.DualChainConfig.ChainId = configs.EthDualChainID
			config.DualChainConfig.DualProtocolName = configs.ProtocolDualETH
		} else if args.neoDual {
			config.DualChainConfig.ChainId = configs.NeoDualChainID
			config.DualChainConfig.DualProtocolName = configs.ProtocolDualNEO
		} else if args.tronDual {
			config.DualChainConfig.ChainId = configs.TronDualChainID
			config.DualChainConfig.DualProtocolName = configs.ProtocolDualTRX
		} else {
			config.DualChainConfig.ChainId = configs.DefaultChainID
			// if it is not default duals (ETH, NEO, TRX) then get value from dualProtocolName args
			// check if args.dualProtocolName is empty then stop program.
			if args.dualProtocolName == "" {
				panic("--dualProtocolName is empty")
			}
			config.DualChainConfig.DualProtocolName = args.dualProtocolName
 		}
		n.RegisterService(dualservice.NewDualService)
	}

	if err := n.Start(); err != nil {
		logger.Error("Cannot start node", "err", err)
		return
	}

	var kardiaService *kai.KardiaService
	if err := n.Service(&kardiaService); err != nil {
		logger.Error("Cannot get Kardia Service", "err", err)
		return
	}
	var dualService *dualservice.DualService
	if args.dualChain {
		if err := n.Service(&dualService); err != nil {
			logger.Error("Cannot get Dual Service", "err", err)
			return
		}

		if args.dualEvent {
			go genDualEvent(dualService.EventPool())
		}
	}
	logger.Info("Genesis block", "genesis", *kardiaService.BlockChain().Genesis())

	if !args.noProxy && len(args.peerProxyIP) == 0 {
		logger.Error("flag noProxy=false but peerProxyIP is empty, will ignore proxy.")
		args.noProxy = true // TODO(thientn): removes when finish cleaning up proxy.
	}

	if !args.noProxy {
		if err := n.CallProxy("Startup", n.Server().Self(), nil); err != nil {
			logger.Error("Error when startup proxy connection", "err", err)
		}
	}

	// Connect with other peers.
	if args.dev {
		// Add Mainchain peers directly as static nodes
		for i := 0; i < config.MainChainConfig.EnvConfig.GetNodeSize(); i++ {
			peerURL := config.MainChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
			logger.Info("Adding static peer", "peerURL", peerURL)
			if err := n.AddPeer(peerURL); err != nil {
				log.Error("Error adding static peer", "err", err)
			}
		}

		if args.dualChain {
			// Add dual peers
			for i := 0; i < config.DualChainConfig.EnvConfig.GetNodeSize(); i++ {
				peerURL := config.DualChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
				logger.Info("Adding static peer", "peerURL", peerURL)
				if err := n.AddPeer(peerURL); err != nil {
					log.Error("Error adding static peer", "err", err)
				}
			}
		}
	}

	if args.bootNode != "" {
		logger.Info("Adding Peer", "Boot Node:", args.bootNode)
		if args.noProxy {
			if err := n.AddPeer(args.bootNode); err != nil {
				log.Error("Error adding bootNode", "err", err)
			}

		} else {
			if err := n.BootNode(args.bootNode); err != nil {
				log.Error("Unable to connect to bootnode", "err", err, "bootNode", args.bootNode)
			}
		}
	}

	if len(args.peer) > 0 {
		urls := strings.Split(args.peer, ",")
		for _, peerURL := range urls {
			logger.Info("Adding static peer", "peerURL", peerURL)
			if err := n.AddPeer(peerURL); err != nil {
				log.Error("Error adding static peer", "err", err)
			}
		}
	}

	// TODO(namdoh): Remove the hard-code below
	exchangeContractAddress := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	exchangeContractAbi := configs.GetContractAbiByAddress(exchangeContractAddress.String())
	if args.neoDual {

		neoProxy, err := neo.NewProxy(kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			args.publishedEndpoint,
			args.subscribedEndpoint,
		)

		if err != nil {
			log.Error("Fail to initialize NeoProxy", "error", err)
			return
		}

		if args.isPrivateDual {
			var kardiaProxy *kardia.PrivateKardiaProxy
			kardiaProxy, err = kardia.NewPrivateKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
				dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize PrivateKardiaProxy", "error", err)
			}
			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, neoProxy)
			dualService.SetDualBlockChainManager(bcManager)
			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(neoProxy)
			neoProxy.RegisterInternalChain(kardiaProxy)
			kardiaProxy.Start(args.mockDualEvent)
		} else {
			var kardiaProxy *kardia.KardiaProxy
			kardiaProxy, err = kardia.NewKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
				dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize KardiaProxy", "error", err)
			}
			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, neoProxy)
			dualService.SetDualBlockChainManager(bcManager)
			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(neoProxy)
			neoProxy.RegisterInternalChain(kardiaProxy)
			kardiaProxy.Start(args.mockDualEvent)
		}
		// Start NEO proxy
		neoProxy.Start()
	}

	// Run TRX-Kardia dual node
	if args.tronDual {
		tronProxy, err := tron.NewProxy(
			kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			args.publishedEndpoint,
			args.subscribedEndpoint,
		)

		if err != nil {
			log.Error("Fail to initialize NeoProxy", "error", err)
			return
		}

		if args.isPrivateDual {
			var kardiaProxy *kardia.PrivateKardiaProxy
			kardiaProxy, err = kardia.NewPrivateKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
				dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize PrivateKardiaProxy", "error", err)
			}
			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, tronProxy)
			dualService.SetDualBlockChainManager(bcManager)
			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(tronProxy)
			tronProxy.RegisterInternalChain(kardiaProxy)
			kardiaProxy.Start(args.mockDualEvent)
		} else {
			var kardiaProxy *kardia.KardiaProxy
			kardiaProxy, err = kardia.NewKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
				dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize KardiaProxy", "error", err)
			}
			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, tronProxy)
			dualService.SetDualBlockChainManager(bcManager)
			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(tronProxy)
			tronProxy.RegisterInternalChain(kardiaProxy)
			kardiaProxy.Start(args.mockDualEvent)
		}

		// Start tron proxy
		tronProxy.Start()
	}

	// Run Eth-Kardia dual node
	if args.ethDual {
		config := &eth.DefaultEthConfig
		config.Name = "GethKardia-" + args.name
		config.ListenAddr = args.ethListenAddr
		config.LightServ = args.ethLightServ
		config.ReportStats = args.ethStat
		config.HTTPPort = args.ethRPCPort
		config.HTTPVirtualHosts = []string{"*"}
		config.NetworkId = args.ethNetworkId

		if args.ethStatName != "" {
			config.StatName = args.ethStatName
		}
		if args.dev && args.mockDualEvent {
			config.DualNodeConfig = dev.CreateDualNodeConfig()
		}

		ethNode, err := eth.NewEth(
			config,
			kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			&exchangeContractAddress,
			exchangeContractAbi)
		if err != nil {
			logger.Error("Fail to create Eth sub node", "err", err)
			return
		}
		if err := ethNode.Start(); err != nil {
			logger.Error("Fail to start Eth sub node", "err", err)
			return
		}

		client, err := ethNode.Client()
		if err != nil {
			logger.Error("Fail to create Eth client", "err", err)
			return
		}

		if args.isPrivateDual {
			var kardiaProxy *kardia.PrivateKardiaProxy
			kardiaProxy, err = kardia.NewPrivateKardiaProxy(
				kardiaService.BlockChain(),
				kardiaService.TxPool(),
				dualService.BlockChain(),
				dualService.EventPool(),
				&exchangeContractAddress,
				exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize KardiaChainProcessor", "error", err)
			}

			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, ethNode)
			dualService.SetDualBlockChainManager(bcManager)

			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(ethNode)
			ethNode.RegisterInternalChain(kardiaProxy)
			go displaySyncStatus(client)
			kardiaProxy.Start(args.mockDualEvent)
		} else {
			var kardiaProxy *kardia.KardiaProxy
			kardiaProxy, err = kardia.NewKardiaProxy(
				kardiaService.BlockChain(),
				kardiaService.TxPool(),
				dualService.BlockChain(),
				dualService.EventPool(),
				&exchangeContractAddress,
				exchangeContractAbi)
			if err != nil {
				log.Error("Fail to initialize KardiaChainProcessor", "error", err)
			}

			// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
			// submit tx to either internal or external blockchain.
			bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, ethNode)
			dualService.SetDualBlockChainManager(bcManager)

			// Register the 'other' blockchain to each internal/external blockchain. This is needed
			// for generate Tx to submit to the other blockchain.
			kardiaProxy.RegisterExternalChain(ethNode)
			ethNode.RegisterInternalChain(kardiaProxy)
			go displaySyncStatus(client)
			kardiaProxy.Start(args.mockDualEvent)
		}
	}

	if args.isPrivateDual {
		// Do some validation
		if args.privateNodeName == "" {
			logger.Error("privateNodeName is required")
			return
		}
		if args.privateValIndexes == "" {
			logger.Error("privateValIndexes is required")
			return
		}

		config := &permissioned.Config{
			Name:              &args.privateNodeName,
			NetworkId:         &args.privateNetworkId,
			ValidatorsIndices: &args.privateValIndexes,
			Proposal:          args.proposal,
			ClearData:         args.clearDataDir,
			ServiceName:       &args.privateServiceName,
			ListenAddr:        &args.privateAddr,
			ChainID:           &args.privateChainId,
		}

		if args.serviceName != "" {
			config.ServiceName = &args.serviceName
		}
		if args.chainId > 0 {
			config.ChainID = &args.chainId
		}
		// Load address and abi of Private chain CandidateDB contract to PermissionedProxy
		candidateDBContractAddress, candidateDBContractAbi := configs.GetContractDetailsByIndex(configs.PrivateChainCandidateDBSmcIndex)
		if candidateDBContractAbi == "" {
			log.Error("Cannot load candidate contract abi on private chain")
			return
		}
		permissionedProxy, err := permissioned.NewPermissionedProxy(config, kardiaService.BlockChain(),
			kardiaService.TxPool(), dualService.BlockChain(), dualService.EventPool(), &candidateDBContractAddress, candidateDBContractAbi)
		if err != nil {
			logger.Error("Init new private proxy failed", "error", err)
			return
		}

		permissionedProxy.Start()

		var kardiaProxy *kardia.PrivateKardiaProxy
		// Load address and abi of candidate exchange contract on Kardia to KardiaProxy
		candidateExchangeContractAddress, candidateExchangeContractAbi := configs.GetContractDetailsByIndex(configs.KardiaCandidateExchangeSmcIndex)
		if exchangeContractAbi == "" {
			log.Error("Failed to load exchange candidate abi contract")
			return
		}
		kardiaProxy, err = kardia.NewPrivateKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
			dualService.EventPool(), &candidateExchangeContractAddress, candidateExchangeContractAbi)
		if err != nil {
			log.Error("Fail to initialize PrivateKardiaProxy", "error", err)
		}
		// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
		// submit tx to either internal or external blockchain.
		bcManager := dualbc.NewDualBlockChainManager(kardiaProxy, permissionedProxy)
		dualService.SetDualBlockChainManager(bcManager)
		// Register the 'other' blockchain to each internal/external blockchain. This is needed
		// for generate Tx to submit to the other blockchain.
		kardiaProxy.RegisterExternalChain(permissionedProxy)
		permissionedProxy.RegisterInternalChain(kardiaProxy)

		kardiaProxy.Start(args.mockDualEvent)
	}

	// Start RPC for all services
	if args.rpcEnabled {
		err := n.StartServiceRPC()
		if err != nil {
			logger.Error("Fail to start RPC", "err", err)
			return
		}
	}
	go displayKardiaPeers(n)

	if args.dev && args.txs {
		go genTxsLoop(args.numTxs, kardiaService.TxPool())
	}

	waitForever()
}

func displayKardiaPeers(n *node.Node) {
	for {
		log.Info("Kardia peers: ", "count", n.Server().PeerCount())
		time.Sleep(20 * time.Second)
	}

}

func displaySyncStatus(client *eth.EthClient) {
	for {
		status, err := client.NodeSyncStatus()
		if err != nil {
			log.Error("Fail to check sync status of EthKarida", "err", err)
		} else {
			log.Info("Sync status", "sync", status)
		}
		time.Sleep(20 * time.Second)
	}
}

// genTxsLoop generate & add a batch of transfer txs, repeat after delay flag.
// Warning: Set txsDelay < 5 secs may build up old subroutines because previous subroutine to add txs won't be finished before new one starts.
func genTxsLoop(numTxs int, txPool *tx_pool.TxPool) {
	genTool := tool.NewGeneratorTool()
	time.Sleep(60 * time.Second)
	genRound := 0
	for {
		go genTxs(genTool, numTxs, txPool, genRound)
		genRound++
		time.Sleep(time.Duration(args.txsDelay) * time.Second)
	}
}

func genTxs(genTool *tool.GeneratorTool, numTxs int, txPool *tx_pool.TxPool, genRound int) {
	goodCount := 0
	badCount := 0
	txList := genTool.GenerateTx(numTxs)
	log.Info("GenTxs Adding new transactions", "num", numTxs, "genRound", genRound)
	errs := txPool.AddLocals(txList)
	for _, err := range errs {
		if err != nil {
			log.Error("Fail to add transaction list", "err", err)
			badCount++
		} else {
			goodCount++
		}
	}
	log.Info("GenTxs Finish adding generated txs", "success", goodCount, "failure", badCount, "genRound", genRound)
}

func genDualEvent(eventPool *event_pool.EventPool) {
	// Wait 10 seconds first for dual peers to connect.
	time.Sleep(time.Duration(10) * time.Second)

	smcArrs := make([]*types.KardiaSmartcontract, 0)
	smcArrs = append(smcArrs, &types.KardiaSmartcontract{
		EventWatcher: &types.Watcher{SmcAddress: "095e7baea6a6c7c4c2dfeb977efac326af552d87"}})
	event := &types.DualEvent{
		Nonce:      0,
		KardiaSmcs: smcArrs,
	}
	log.Info("Adding initial DualEvent", "event", event.String())
	err := eventPool.AddEvent(event)
	if err != nil {
		log.Error("Fail to add initial dual event", "err", err)
	}
}

func waitForever() {
	select {}
}
