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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
)

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

// newLog inits new logger for kardia
func newLog() log.Logger {
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
	return log.New()
}

//func main() {
//	flag.Parse()
//	logger := newLog()
//
//	// System settings
//	if err := runtimeSystemSettings(); err != nil {
//		logger.Error("Fail to update system settings", "err", err)
//		return
//	}
//
//	var nodeIndex int
//	if len(args.name) == 0 {
//		logger.Error("Invalid node name", "name", args.name)
//	} else {
//		index, err := node.GetNodeIndex(args.name)
//		if err != nil {
//			logger.Error("Node name must be formatted as \"\\c*\\d{1,2}\"", "name", args.name)
//		}
//		nodeIndex = index - 1
//	}
//
//	// Setups config.
//	config := &node.DefaultConfig
//	config.P2P.ListenAddr = args.listenAddr
//	config.Name = args.name
//
//	// Setup bootNode
//	if args.rpcEnabled {
//		if config.HTTPHost = args.rpcAddr; config.HTTPHost == "" {
//			config.HTTPHost = node.DefaultHTTPHost
//		}
//		config.HTTPPort = args.rpcPort
//		config.HTTPVirtualHosts = []string{"*"} // accepting RPCs from all source hosts
//	}
//
//	if args.dev {
//		config.MainChainConfig.EnvConfig = node.NewEnvironmentConfig()
//		// Set P2P max peers for testing on dev environment
//		config.P2P.MaxPeers = args.maxPeers
//		if nodeIndex < 0 {
//			logger.Error(fmt.Sprintf("Node index %v must greater than 0", nodeIndex+1))
//		}
//		// Subtract 1 from the index because we specify node starting from 1 onward.
//		config.MainChainConfig.EnvConfig.SetProposerIndex(args.proposal-1, len(dev.Nodes))
//		// Only set DevNodeConfig if this is a known node from Kardia default set
//		if nodeIndex < len(dev.Nodes) {
//			nodeMetadata, err := dev.GetNodeMetadataByIndex(nodeIndex)
//			if err != nil {
//				logger.Error("Cannot get node by index", "err", err)
//			}
//			config.NodeMetadata = nodeMetadata
//		}
//		// Simulate the voting strategy
//		config.MainChainConfig.EnvConfig.SetVotingStrategy(args.votingStrategy)
//		config.MainChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)
//
//		// Create genesis block with dev.genesisAccounts
//		config.MainChainConfig.Genesis = genesis.DefaulTestnetFullGenesisBlock(configs.GenesisAccounts, configs.GenesisContracts)
//	}
//	nodeDir := filepath.Join(config.DataDir, config.Name)
//	config.MainChainConfig.TxPool = *tx_pool.GetDefaultTxPoolConfig(nodeDir)
//	config.MainChainConfig.TxPool.GlobalSlots = uint64(args.maxPending) // for pending
//	config.MainChainConfig.TxPool.GlobalQueue =  uint64(args.maxAll) // for all
//	config.MainChainConfig.TxPool.NumberOfWorkers = args.workers
//	config.MainChainConfig.TxPool.WorkerCap = args.workerCap
//	config.MainChainConfig.TxPool.BlockSize = args.blockSize
//
//	config.MainChainConfig.IsZeroFee = args.isZeroFee
//	config.MainChainConfig.IsPrivate = args.isPrivate
//
//	if args.networkId > 0 {
//		config.MainChainConfig.NetworkId = args.networkId
//	}
//	if args.chainId > 0 {
//		config.MainChainConfig.ChainId = args.chainId
//	}
//	if args.serviceName != "" {
//		config.MainChainConfig.ServiceName = args.serviceName
//	}
//	if args.clearDataDir {
//		// Clear all contents within data dir
//		err := removeDirContents(nodeDir)
//		if err != nil {
//			logger.Error("Cannot remove contents in directory", "dir", nodeDir, "err", err)
//			return
//		}
//	}
//	// check dbtype
//	if args.db == MongoDb {
//		if args.dbUri == "" || args.dbName == "" {
//			panic("dbUri and DbName must not be empty")
//		}
//		config.MainChainConfig.DBInfo = storage.NewMongoDbInfo(args.dbUri, args.dbName, args.dbDrop)
//	} else {
//		config.MainChainConfig.DBInfo = storage.NewLevelDbInfo(config.ResolvePath(node.MainChainDataDir), node.DefaultDbCache, node.DefaultDbHandles)
//	}
//
//	config.PeerProxyIP = args.peerProxyIP
//
//	n, err := node.NewNode(config)
//	if err != nil {
//		logger.Error("Cannot create node", "err", err)
//		return
//	}
//
//	n.RegisterService(kai.NewKardiaService)
//	if args.dualChain {
//		if args.dev {
//			// Set env config for dualchain config
//			config.DualChainConfig.EnvConfig = node.NewEnvironmentConfig()
//			// Subtract 1 from the index because we specify node starting from 1 onward.
//			config.MainChainConfig.EnvConfig.SetProposerIndex(args.proposal-1, len(dev.Nodes))
//			config.DualChainConfig.DualGenesis = genesis.DefaulTestnetFullGenesisBlock(configs.GenesisAccounts, configs.GenesisContracts)
//		}
//
//		if len(args.dualChainValIndexes) > 0 {
//			config.DualChainConfig.ValidatorIndexes = getIntArray(args.dualChainValIndexes)
//		} else {
//			config.DualChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)
//		}
//		config.DualChainConfig.DualEventPool = *event_pool.GetDefaultEventPoolConfig(nodeDir)
//		config.DualChainConfig.IsPrivate = args.isPrivateDual
//		config.DualChainConfig.ChainId = args.devDualChainID
//		config.DualChainConfig.DualNetworkID = args.dualNetworkId
//		if args.ethDual {
//			config.DualChainConfig.ChainId = configs.EthDualChainID
//			config.DualChainConfig.DualProtocolName = configs.ProtocolDualETH
//		} else if args.neoDual {
//			config.DualChainConfig.ChainId = configs.NeoDualChainID
//			config.DualChainConfig.DualProtocolName = configs.ProtocolDualNEO
//		} else if args.tronDual {
//			config.DualChainConfig.ChainId = configs.TronDualChainID
//			config.DualChainConfig.DualProtocolName = configs.ProtocolDualTRX
//		} else {
//			config.DualChainConfig.ChainId = configs.DefaultChainID
//			// if it is not default duals (ETH, NEO, TRX) then get value from dualProtocolName args
//			// check if args.dualProtocolName is empty then stop program.
//			if args.dualProtocolName == "" {
//				panic("--dualProtocolName is empty")
//			}
//			config.DualChainConfig.DualProtocolName = args.dualProtocolName
//		}
//		n.RegisterService(dualservice.NewDualService)
//	}
//
//	if err := n.Start(); err != nil {
//		logger.Error("Cannot start node", "err", err)
//		return
//	}
//
//	var kardiaService *kai.KardiaService
//	if err := n.Service(&kardiaService); err != nil {
//		logger.Error("Cannot get Kardia Service", "err", err)
//		return
//	}
//	var dualService *dualservice.DualService
//	if args.dualChain {
//		if err := n.Service(&dualService); err != nil {
//			logger.Error("Cannot get Dual Service", "err", err)
//			return
//		}
//
//		if args.dualEvent {
//			go genDualEvent(dualService.EventPool())
//		}
//	}
//	logger.Info("Genesis block", "genesis", *kardiaService.BlockChain().Genesis())
//
//	if !args.noProxy && len(args.peerProxyIP) == 0 {
//		logger.Error("flag noProxy=false but peerProxyIP is empty, will ignore proxy.")
//		args.noProxy = true // TODO(thientn): removes when finish cleaning up proxy.
//	}
//
//	if !args.noProxy {
//		if err := n.CallProxy("Startup", n.Server().Self(), nil); err != nil {
//			logger.Error("Error when startup proxy connection", "err", err)
//		}
//	}
//
//	// Connect with other peers.
//	if args.dev {
//		// Add Mainchain peers directly as static nodes
//		for i := 0; i < config.MainChainConfig.EnvConfig.GetNodeSize(); i++ {
//			peerURL := config.MainChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
//			logger.Info("Adding static peer", "peerURL", peerURL)
//			if err := n.AddPeer(peerURL); err != nil {
//				log.Error("Error adding static peer", "err", err)
//			}
//		}
//
//		if args.dualChain {
//			// Add dual peers
//			for i := 0; i < config.DualChainConfig.EnvConfig.GetNodeSize(); i++ {
//				peerURL := config.DualChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
//				logger.Info("Adding static peer", "peerURL", peerURL)
//				if err := n.AddPeer(peerURL); err != nil {
//					log.Error("Error adding static peer", "err", err)
//				}
//			}
//		}
//	}
//
//	if args.bootNode != "" {
//		logger.Info("Adding Peer", "Boot Node:", args.bootNode)
//		if args.noProxy {
//			if err := n.AddPeer(args.bootNode); err != nil {
//				log.Error("Error adding bootNode", "err", err)
//			}
//
//		} else {
//			if err := n.BootNode(args.bootNode); err != nil {
//				log.Error("Unable to connect to bootnode", "err", err, "bootNode", args.bootNode)
//			}
//		}
//	}
//
//	if len(args.peer) > 0 {
//		urls := strings.Split(args.peer, ",")
//		for _, peerURL := range urls {
//			logger.Info("Adding static peer", "peerURL", peerURL)
//			if err := n.AddPeer(peerURL); err != nil {
//				log.Error("Error adding static peer", "err", err)
//			}
//		}
//	}
//
//	if err := StartProxy(args, logger, kardiaService, dualService); err != nil {
//		logger.Error("error while starting proxies", "err", err)
//	}
//
//	// Start RPC for all services
//	if args.rpcEnabled {
//		err := n.StartServiceRPC()
//		if err != nil {
//			logger.Error("Fail to start RPC", "err", err)
//			return
//		}
//	}
//	go displayKardiaPeers(n)
//
//	if args.dev && args.txs {
//		go genTxsLoop(args.numTxs, kardiaService.TxPool())
//	}
//
//	waitForever()
//}

// genTxsLoop generate & add a batch of transfer txs, repeat after delay flag.
// Warning: Set txsDelay < 5 secs may build up old subroutines because previous subroutine to add txs won't be finished before new one starts.
func genTxsLoop(numTxs int, txPool *tx_pool.TxPool) {
	accounts := tool.GetAccounts(configs.GenesisAddrKeys)
	genTool := tool.NewGeneratorTool(accounts)
	time.Sleep(60 * time.Second)
	genRound := 0
	for {
		go genTxs(genTool, numTxs, txPool, genRound)
		genRound++
		time.Sleep(time.Duration(args.txsDelay) * time.Second)
	}
}

func genTxs(genTool *tool.GeneratorTool, numTxs int, txPool *tx_pool.TxPool, genRound int) {
	txList := genTool.GenerateRandomTxWithState(numTxs, txPool.State().StateDB)
	log.Info("GenTxs Adding new transactions", "num", numTxs, "genRound", genRound)
	txPool.AddTxs(txList)
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
