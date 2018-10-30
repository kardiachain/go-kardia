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
	"math/big"
	"runtime"
	"strconv"
	"time"

	"encoding/hex"
	elog "github.com/ethereum/go-ethereum/log"
	"github.com/kardiachain/go-kardia/dev"
	dualbc "github.com/kardiachain/go-kardia/dual/blockchain"
	dualeth "github.com/kardiachain/go-kardia/dual/external/eth"
	dualservice "github.com/kardiachain/go-kardia/dual/service"
	"github.com/kardiachain/go-kardia/kardia"
	"github.com/kardiachain/go-kardia/kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"os"
	"path/filepath"
	"strings"
)

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

func RemoveDirContents(dir string) error {
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

func GetNodeIndex(nodeName string) (int, error) {
	return strconv.Atoi((nodeName)[len(nodeName)-1:])
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

// args
type flagArgs struct {
	logLevel string
	logTag   string

	// Kardia node's related flags
	listenAddr          string
	name                string
	rpcEnabled          bool
	rpcAddr             string
	rpcPort             int
	addTxn              bool
	addSmcCall          bool
	genNewTxs           bool
	newTxDelay          int
	lightNode           bool
	lightServ           int
	cacheSize           int
	bootnodes           string
	peer                string
	clearDataDir        bool
	mainChainValIndexes string
	acceptTxs           int

	// Ether/Kardia dualnode related flags
	ethDual       bool
	ethStat       bool
	ethStatName   string
	ethLogLevel   string
	ethListenAddr string

	// Neo/Kardia dualnode related flags
	neoDual bool

	// Dualnode's related flags
	dualChain           bool
	dualChainValIndexes string

	// Development's related flags
	dev            bool
	proposal       int
	votingStrategy string
	mockDualEvent  bool
}

var args flagArgs

func init() {
	flag.StringVar(&args.logLevel, "loglevel", "info", "minimum log verbosity to display")
	flag.StringVar(&args.logTag, "logtag", "", "for log record with log, use this to further filter records based on the tag value")
	flag.StringVar(&args.ethLogLevel, "ethloglevel", "warn", "minimum Eth log verbosity to display")
	flag.StringVar(&args.listenAddr, "addr", ":30301", "listen address")
	flag.StringVar(&args.name, "name", "", "Name of node")
	flag.BoolVar(&args.rpcEnabled, "rpc", false, "whether to open HTTP RPC endpoints")
	flag.StringVar(&args.rpcAddr, "rpcaddr", "", "HTTP-RPC server listening interface")
	flag.IntVar(&args.rpcPort, "rpcport", node.DefaultHTTPPort, "HTTP-RPC server listening port")
	flag.BoolVar(&args.addTxn, "txn", false, "whether to add a transfer txn")
	flag.BoolVar(&args.addSmcCall, "smc", false, "where to add smart contract call")
	flag.BoolVar(&args.genNewTxs, "genNewTxs", false, "whether to run routine that regularly add new transactions.")
	flag.IntVar(&args.newTxDelay, "newTxDelay", 10, "how often new txs are added.")
	flag.BoolVar(&args.ethDual, "dual", false, "whether to run in dual mode")
	flag.StringVar(&args.ethListenAddr, "ethAddr", ":30302", "listen address for eth")
	flag.BoolVar(&args.neoDual, "neodual", false, "whether to run in dual mode")
	flag.BoolVar(&args.ethStat, "ethstat", false, "report eth stats to network")
	flag.StringVar(&args.ethStatName, "ethstatname", "", "name to use when reporting eth stats")
	flag.BoolVar(&args.lightNode, "light", false, "connect to Eth as light node")
	flag.IntVar(&args.lightServ, "lightserv", 0, "max percentage of time serving light client reqs")
	flag.IntVar(&args.cacheSize, "cacheSize", 1024, "cache memory size for Eth node")
	flag.StringVar(&args.bootnodes, "bootnodes", "", "Comma separated enode URLs for P2P discovery bootstrap")
	flag.StringVar(&args.peer, "peer", "", "Comma separated enode URLs for P2P static peer")
	flag.BoolVar(&args.clearDataDir, "clearDataDir", false, "remove contents in data dir")
	flag.IntVar(&args.acceptTxs, "acceptTxs", 1, "accept process tx or not, 1 is yes and 0 is no")
	flag.BoolVar(&args.dualChain, "dualchain", false, "run dual chain for group concensus")
	flag.StringVar(&args.mainChainValIndexes, "mainChainValIndexes", "1,2,3", "Indexes of Main chain validator")
	flag.StringVar(&args.dualChainValIndexes, "dualChainValIndexes", "", "Indexes of Dual chain validator")
	// NOTE: The flags below are only applicable for dev environment. Please add the applicable ones
	// here and DO NOT add non-dev flags.
	flag.BoolVar(&args.dev, "dev", false, "deploy node with dev environment")
	flag.StringVar(&args.votingStrategy, "votingStrategy", "", "specify the voting script or strategy to simulate voting. Note that this flag only has effect when --dev flag is set")
	flag.IntVar(&args.proposal, "proposal", 1, "specify which node is the proposer. The index starts from 1, and every node needs to use the same proposer index. Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.mockDualEvent, "mockDualEvent", false, "generate fake dual events to trigger dual consensus. Note that this flag only has effect when --dev flag is set.")
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
		log.Root().SetHandler(log.LvlAndTagFilterHandler(level, args.logTag, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	} else {
		log.Root().SetHandler(log.LvlFilterHandler(level, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	}

	logger := log.New()

	elevel, err := elog.LvlFromString(args.ethLogLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		elevel = elog.LvlInfo
	}
	elog.Root().SetHandler(elog.LvlFilterHandler(elevel, elog.StdoutHandler))

	// System settings
	if err := runtimeSystemSettings(); err != nil {
		logger.Error("Fail to update system settings", "err", err)
		return
	}

	var nodeIndex int
	if len(args.name) == 0 {
		logger.Error("Invalid node name", "name", args.name)
	} else {
		index, err := GetNodeIndex(args.name)
		if err != nil {
			logger.Error("Node name must be formmated as \"\\c*\\d{1,2}\"", "name", args.name)
		}
		nodeIndex = index - 1
	}

	// Setups config.
	config := &node.DefaultConfig
	config.P2P.ListenAddr = args.listenAddr
	config.Name = args.name
	config.MainChainConfig.AcceptTxs = uint32(args.acceptTxs)
	var devEnv *dev.DevEnvironmentConfig

	// Setup bootnodes
	if len(args.bootnodes) > 0 {
		urls := strings.Split(args.bootnodes, ",")
		config.P2P.BootstrapNodes = make([]*discover.Node, 0, len(urls))
		for _, url := range urls {
			node, err := discover.ParseNode(url)
			if err != nil {
				logger.Error("Bootstrap URL invalid", "enode", url, "err", err)
			} else {
				config.P2P.BootstrapNodes = append(config.P2P.BootstrapNodes, node)
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
		if nodeIndex < 0 && nodeIndex >= devEnv.GetNodeSize() {
			logger.Error(fmt.Sprintf("Node index %v must be within %v and %v", nodeIndex+1, 1, devEnv.GetNodeSize()))

		}
		// Substract 1 from the index because we specify node starting from 1 onward.
		devEnv.SetProposerIndex(args.proposal - 1)
		config.DevNodeConfig = devEnv.GetDevNodeConfig(nodeIndex)
		// Simulate the voting strategy
		devEnv.SetVotingStrategy(args.votingStrategy)
		config.DevEnvConfig = devEnv
		config.MainChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)

		// Setup config for kardia service
		config.MainChainConfig.ChainData = dev.ChainData
		config.MainChainConfig.DbHandles = dev.DbHandles
		config.MainChainConfig.DbCache = dev.DbCache

		// Create genesis block with dev.genesisAccounts
		config.MainChainConfig.Genesis = blockchain.DefaulTestnetFullGenesisBlock(dev.GenesisAccounts, dev.GenesisContracts)
	}
	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.MainChainConfig.TxPool = *blockchain.GetDefaultTxPoolConfig(nodeDir)

	if args.dualChain {
		if len(args.dualChainValIndexes) > 0 {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.dualChainValIndexes)
		} else {
			config.DualChainConfig.ValidatorIndexes = getIntArray(args.mainChainValIndexes)
		}

		config.DualChainConfig.ChainData = "dualdata"
		config.DualChainConfig.DbHandles = dev.DbHandles
		config.DualChainConfig.DbCache = dev.DbCache
	}
	config.DualChainConfig.DualEventPool = *dualbc.GetDefaultEventPoolConfig(nodeDir)

	if args.clearDataDir {
		// Clear all contents within data dir
		err := RemoveDirContents(nodeDir)
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

	if args.addTxn {
		logger.Info("Adding local txn to send 10 coin from addr0 to addr1")
		//sender is account[0] in dev genesis
		senderByteK, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
		senderKey := crypto.ToECDSAUnsafe(senderByteK)

		// account[1] in dev genesis
		receiverAddr := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")

		simpleTx := types.NewTransaction(
			0,
			receiverAddr,
			big.NewInt(10),
			10,
			big.NewInt(10),
			nil,
		)
		txPool := kardiaService.TxPool()
		signedTx, _ := types.SignTx(simpleTx, senderKey)

		err := txPool.AddLocal(signedTx)
		if err != nil {
			logger.Error("Txn add error", "err", err)
		}
	}

	if args.addSmcCall {
		txPool := kardiaService.TxPool()
		statedb, err := kardiaService.BlockChain().State()
		if err != nil {
			logger.Error("Cannot get state", "state", err)
		}
		// Get first contract in genesis contracts
		/*counterSmcAddress := devEnv.GetContractAddressAt(1)
		smcAbi := devEnv.GetContractAbiByAddress(counterSmcAddress.String())
		statedb, err := kardiaService.BlockChain().State()
		// Caller is account[1] in genesis
		callerByteK, _ := hex.DecodeString("77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79")
		callerKey := crypto.ToECDSAUnsafe(callerByteK)
		counterAbi, err := abi.JSON(strings.NewReader(smcAbi))
		if err != nil {
			logger.Error("Can not read abi", err)
		}
		input, err := counterAbi.Pack("set", uint8(5))
		if err != nil {
			logger.Error("Cannot pack method call", err)
		}
		simpleContractCall := tool.GenerateSmcCall(callerKey, counterSmcAddress, input, statedb)
		signedSmcCall, _ := types.SignTx(simpleContractCall, callerKey)
		err = txPool.AddLocal(signedSmcCall)
		if err!= nil {
			logger.Error("Error adding contract call", "err", err)
		} else {
			logger.Info("Adding counter contract call successfully")
		}
		*/
		// Get voting contract address, this smc is created in genesis block
		votingSmcAddress := dev.GetContractAddressAt(0)
		votingAbiStr := dev.GetContractAbiByAddress(votingSmcAddress.String())
		votingAbi, err := abi.JSON(strings.NewReader(votingAbiStr))
		if err != nil {
			logger.Error("Can not read abi", err)
		}
		// Caller2 is account[2] in genesis
		caller2ByteK, _ := hex.DecodeString("98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb")
		caller2Key := crypto.ToECDSAUnsafe(caller2ByteK)
		voteInput, err := votingAbi.Pack("vote", uint8(1))
		if err != nil {
			logger.Error("Can not read abi", err)
		}

		votingContractCall := tool.GenerateSmcCall(caller2Key, votingSmcAddress, voteInput, statedb)
		signedSmcCall2, _ := types.SignTx(votingContractCall, caller2Key)
		err = txPool.AddLocal(signedSmcCall2)
		if err != nil {
			logger.Error("Error adding contract call", "err", err)
		} else {
			logger.Info("Adding voting contract call successfully")
		}
	}

	if args.genNewTxs {
		go runTxCreationLoop(kardiaService.TxPool(), args.newTxDelay)
	}

	// Connect with other peers.
	if args.dev {
		for i := 0; i < nodeIndex; i++ {
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
	// go displayPeers(n)

	var dualP *dualeth.DualProcessor

	// TODO(namdoh): Remove the hard-code below
	exchangeContractAddress := dev.GetContractAddressAt(2)
	exchangeContractAbi := dev.GetContractAbiByAddress(exchangeContractAddress.String())
	// TODO: This should trigger for either Eth dual or Neo dual flag, so  *ethDual || *neoDual
	if args.ethDual || args.neoDual {
		dualP, err = dualeth.NewDualProcessor(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(), dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
		if err != nil {
			log.Error("Fail to initialize DualProcessor", "error", err)
		} else {
			dualP.Start()
		}
	}

	// Run Eth-Kardia dual node
	if args.ethDual {
		config := &dualeth.DefaultEthKardiaConfig
		config.Name = "GethKardia-" + args.name
		config.ListenAddr = args.ethListenAddr
		config.LightNode = args.lightNode
		config.LightServ = args.lightServ
		config.ReportStats = args.ethStat
		if args.ethStatName != "" {
			config.StatName = args.ethStatName
		}
		config.CacheSize = args.cacheSize
		if args.dev && args.mockDualEvent {
			config.DualNodeConfig = dev.CreateDualNodeConfig()
		}

		ethNode, err := dualeth.NewEthKardia(config, kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(), dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)

		// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
		// submit tx to either internal or external blockchain.
		bcManager := dualbc.NewDualBlockChainManager(kardiaService, ethNode)
		dualService.SetDualBlockChainManager(bcManager)

		if err != nil {
			logger.Error("Fail to create Eth sub node", "err", err)
			return
		}
		if err := ethNode.Start(); err != nil {
			logger.Error("Fail to start Eth sub node", "err", err)
			return
		}
		go displayEthPeers(ethNode)

		client, err := ethNode.Client()
		if err != nil {
			logger.Error("Fail to create EthKardia client", "err", err)
			return
		}

		// Register to dual processor
		dualP.RegisterEthDualNode(ethNode)

		go displaySyncStatus(client)
	}

	go displayKardiaPeers(n)
	waitForever()
}

func updateAmountToSend(b *blockchain.BlockChain, txPool *blockchain.TxPool) {
	statedb, err := b.State()

	if err != nil {
		log.Error("Error getting state. Cannot make contract call")
		return
	} else {
		log.Info("Preparing to update amount in master smc")
	}

	caller2ByteK, _ := hex.DecodeString("98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb")
	caller2Key := crypto.ToECDSAUnsafe(caller2ByteK)
	rand := common.NewRand()
	quantity := big.NewInt(rand.Int63n(100))
	tx1 := dualservice.CreateKardiaMatchAmountTx(caller2Key, statedb, quantity, 1)
	// txPool.AddLocal(tx1)
	log.Info("Match eth", "quantity successfully", quantity, "txhash:", tx1.Hash())

	caller3ByteK, _ := hex.DecodeString("32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe")
	caller3Key := crypto.ToECDSAUnsafe(caller3ByteK)
	tx2 := dualservice.CreateKardiaMatchAmountTx(caller3Key, statedb, quantity, 2)
	txs := make(types.Transactions, 2)
	txs[0] = tx1
	txs[1] = tx2
	// txPool.AddLocal(tx2)
	txPool.AddLocals(txs)
	log.Info("Match neo", "quantity successfully", quantity, "txhash:", tx2.Hash())
}

func removeAmountToSend(b *blockchain.BlockChain, txPool *blockchain.TxPool, quantity *big.Int) {
	statedb, err := b.State()

	if err != nil {
		log.Error("Error getting state. Cannot make contract call")
		return
	} else {
		log.Info("Preparing to remove amount in master smc")
	}

	caller2ByteK, _ := hex.DecodeString("98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb")
	caller2Key := crypto.ToECDSAUnsafe(caller2ByteK)

	tx1 := dualservice.CreateKardiaRemoveAmountTx(caller2Key, statedb, quantity, 1)
	// txPool.AddLocal(tx1)
	log.Info("Remove eth", "quantity successfully", quantity, "txhash:", tx1.Hash())
	caller3ByteK, _ := hex.DecodeString("32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe")
	caller3Key := crypto.ToECDSAUnsafe(caller3ByteK)
	tx2 := dualservice.CreateKardiaRemoveAmountTx(caller3Key, statedb, quantity, 2)
	txs := make(types.Transactions, 2)
	txs[0] = tx1
	txs[1] = tx2
	// txPool.AddLocal(tx2)
	txPool.AddLocals(txs)
	log.Info("Remove neo", "quantity successfully", quantity, "txhash:", tx2.Hash())
}

func displayEthPeers(n *dualeth.EthKardia) {
	for {
		log.Info("Ethereum peers: ", "count", n.EthNode().Server().PeerCount())
		time.Sleep(20 * time.Second)
	}
}

func displayKardiaPeers(n *node.Node) {
	for {
		log.Info("Kardia peers: ", "count", n.Server().PeerCount())
		time.Sleep(20 * time.Second)
	}

}

func displaySyncStatus(client *dualeth.KardiaEthClient) {
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

func runTxCreationLoop(txPool *blockchain.TxPool, delay int) {
	for {
		txs := tool.GenerateRandomTx(1)
		log.Info("Adding new transactions", "txs", txs)
		errs := txPool.AddLocals(txs)
		for _, err := range errs {
			if err != nil {
				log.Error("Fail to add transaction list", "err", err, "txs", txs)
			}
		}
		time.Sleep(time.Duration(delay) * time.Second)
	}
}

func waitForever() {
	select {}
}
