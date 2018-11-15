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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	ethlog "github.com/ethereum/go-ethereum/log"

	"github.com/kardiachain/go-kardia/dev"
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	dualservice "github.com/kardiachain/go-kardia/dualchain/service"
	"github.com/kardiachain/go-kardia/dualnode/eth"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/dualnode/neo"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p/discover"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	"github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/node"
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
	bootnodes           string
	peer                string
	clearDataDir        bool
	mainChainValIndexes string

	// Ether/Kardia dualnode related flags
	ethDual       bool
	ethStat       bool
	ethStatName   string
	ethLogLevel   string
	ethListenAddr string
	ethLightServ  int

	// Neo/Kardia dualnode related flags
	neoDual            bool
	neoSubmitTxUrl     string
	neoCheckTxUrl      string
	neoReceiverAddress string

	// Dualnode's related flags
	dualChain           bool
	dualChainValIndexes string

	// Development's related flags
	dev            bool
	proposal       int
	votingStrategy string
	mockDualEvent  bool
	devDualChainID uint64
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
	flag.StringVar(&args.bootnodes, "bootnodes", "", "Comma separated enode URLs for P2P discovery bootstrap")
	flag.StringVar(&args.peer, "peer", "", "Comma separated enode URLs for P2P static peer")
	flag.BoolVar(&args.clearDataDir, "clearDataDir", false, "remove contents in data dir")
	flag.StringVar(&args.mainChainValIndexes, "mainChainValIndexes", "1,2,3", "Indexes of Main chain validator")

	// Dualnode's related flags
	flag.StringVar(&args.ethLogLevel, "ethloglevel", "warn", "minimum Eth log verbosity to display")
	flag.BoolVar(&args.ethDual, "dual", false, "whether to run in dual mode")
	flag.StringVar(&args.ethListenAddr, "ethAddr", ":30302", "listen address for eth")
	flag.BoolVar(&args.neoDual, "neodual", false, "whether to run in dual mode")
	flag.BoolVar(&args.ethStat, "ethstat", false, "report eth stats to network")
	flag.StringVar(&args.ethStatName, "ethstatname", "", "name to use when reporting eth stats")
	flag.IntVar(&args.ethLightServ, "ethLightServ", 0, "max percentage of time serving Ethereum light client requests")
	flag.BoolVar(&args.dualChain, "dualchain", false, "run dual chain for group consensus")
	flag.StringVar(&args.dualChainValIndexes, "dualChainValIndexes", "", "Indexes of Dual chain validator")
	flag.StringVar(&args.neoSubmitTxUrl, "neoSubmitTxUrl", "", "url to submit tx to neo")
	flag.StringVar(&args.neoCheckTxUrl, "neoCheckTxUrl", "", "url to check tx status from neo")
	flag.StringVar(&args.neoReceiverAddress, "neoReceiverAddress", "", "neo address to release to")

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
	flag.Uint64Var(&args.devDualChainID, "devDualChainID", eth.EthDualChainID, "manually set dualchain ID")
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
	var devEnv *dev.DevEnvironmentConfig

	// Setup bootnodes
	if len(args.bootnodes) > 0 {
		urls := strings.Split(args.bootnodes, ",")
		config.P2P.BootstrapNodes = make([]*discover.Node, 0, len(urls))
		for _, url := range urls {
			bootnode, err := discover.ParseNode(url)
			if err != nil {
				logger.Error("Bootstrap URL invalid", "enode", url, "err", err)
			} else {
				config.P2P.BootstrapNodes = append(config.P2P.BootstrapNodes, bootnode)
			}
		}
	}

	if args.rpcEnabled {
		if config.HTTPHost = args.rpcAddr; config.HTTPHost == "" {
			config.HTTPHost = node.DefaultHTTPHost
		}
		config.HTTPPort = args.rpcPort
		config.HTTPVirtualHosts = []string{"*"} // accepting RPCs from all source hosts
	}

	if args.dev {
		devEnv = dev.CreateDevEnvironmentConfig()
		// Set P2P max peers for testing on dev environment
		config.P2P.MaxPeers = args.maxPeers
		if nodeIndex < 0 {
			logger.Error(fmt.Sprintf("Node index %v must greater than 0", nodeIndex+1))
		}
		// Subtract 1 from the index because we specify node starting from 1 onward.
		devEnv.SetProposerIndex(args.proposal - 1)
		// Only set DevNodeConfig if this is a known node from Kardia default set
		if nodeIndex < devEnv.GetNodeSize() {
			config.DevNodeConfig = devEnv.GetDevNodeConfig(nodeIndex)
		}
		// Simulate the voting strategy
		devEnv.SetVotingStrategy(args.votingStrategy)
		config.DevEnvConfig = devEnv
		config.MainChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)

		// Create genesis block with dev.genesisAccounts
		config.MainChainConfig.Genesis = blockchain.DefaulTestnetFullGenesisBlock(dev.GenesisAccounts, dev.GenesisContracts)
	}
	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.MainChainConfig.TxPool = *blockchain.GetDefaultTxPoolConfig(nodeDir)

	if args.clearDataDir {
		// Clear all contents within data dir
		err := removeDirContents(nodeDir)
		if err != nil {
			logger.Error("Cannot remove contents in directory", "dir", nodeDir, "err", err)
			return
		}
	}

	n, err := node.NewNode(config)
	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	n.RegisterService(kai.NewKardiaService)
	if args.dualChain {
		if len(args.dualChainValIndexes) > 0 {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.dualChainValIndexes)
		} else {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)
		}
		config.DualChainConfig.DualEventPool = *dualbc.GetDefaultEventPoolConfig(nodeDir)

		config.DualChainConfig.ChainId = args.devDualChainID
		if args.ethDual {
			config.DualChainConfig.ChainId = eth.EthDualChainID
		} else if args.neoDual {
			config.DualChainConfig.ChainId = neo.NeoDualChainID
		}

		n.RegisterService(dualservice.NewDualService)
	}
	if err := n.Start(); err != nil {
		logger.Error("Cannot start node", "err", err)
		return
	}

	var kardiaService *kai.Kardia
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
	}
	logger.Info("Genesis block", "genesis", *kardiaService.BlockChain().Genesis())

	// Connect with other peers.
	if args.dev {
		for i := 0; i < devEnv.GetNodeSize(); i++ {
			peerURL := devEnv.GetDevNodeConfig(i).NodeID
			logger.Info("Adding static peer", "peerURL", peerURL)
			success, err := n.AddPeer(peerURL)
			if !success {
				logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
			}
		}
	}

	if len(args.peer) > 0 {
		urls := strings.Split(args.peer, ",")
		for _, peerURL := range urls {
			logger.Info("Adding static peer", "peerURL", peerURL)
			success, err := n.AddPeer(peerURL)
			if !success {
				logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
			}
		}
	}

	// TODO(namdoh): Remove the hard-code below
	exchangeContractAddress := dev.GetContractAddressAt(2)
	exchangeContractAbi := dev.GetContractAbiByAddress(exchangeContractAddress.String())
	if args.neoDual {
		dualP, err := neo.NewDualNeo(
			kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			&exchangeContractAddress,
			exchangeContractAbi)
		if err != nil {
			log.Error("Fail to initialize DualNeo", "error", err)
		} else {
			if args.neoReceiverAddress != "" {
				dualP.SetNeoReceiver(args.neoReceiverAddress)
			}
			if args.neoCheckTxUrl != "" {
				dualP.SetCheckTxUrl(args.neoCheckTxUrl)
			}
			if args.neoSubmitTxUrl != "" {
				dualP.SetSubmitTxUrl(args.neoSubmitTxUrl)
			}
			logger.Info("Neo config", "config", dualP.ReportConfig())
			dualP.Start()
		}

	}

	// Run Eth-Kardia dual node
	if args.ethDual {
		config := &eth.DefaultEthConfig
		config.Name = "GethKardia-" + args.name
		config.ListenAddr = args.ethListenAddr
		config.LightServ = args.ethLightServ
		config.ReportStats = args.ethStat
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
		kardiaProxy.Start()
	}

	go displayKardiaPeers(n)
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

func waitForever() {
	select {}
}
