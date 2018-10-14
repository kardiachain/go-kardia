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
	"github.com/kardiachain/go-kardia/abi"
	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/dual"
	"github.com/kardiachain/go-kardia/kai"
	development "github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	"github.com/kardiachain/go-kardia/node"
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

func main() {
	// args
	logLevel := flag.String("loglevel", "info", "minimum log verbosity to display")
	logTag := flag.String("logtag", "", "for log record with log, use this to further filter records based on the tag value")
	ethLogLevel := flag.String("ethloglevel", "warn", "minimum Eth log verbosity to display")
	listenAddr := flag.String("addr", ":30301", "listen address")
	name := flag.String("name", "", "Name of node")
	rpcEnabled := flag.Bool("rpc", false, "whether to open HTTP RPC endpoints")
	rpcAddr := flag.String("rpcaddr", "", "HTTP-RPC server listening interface")
	rpcPort := flag.Int("rpcport", node.DefaultHTTPPort, "HTTP-RPC server listening port")
	addTxn := flag.Bool("txn", false, "whether to add a transfer txn")
	addSmcCall := flag.Bool("smc", false, "where to add smart contract call")
	genNewTxs := flag.Bool("genNewTxs", false, "whether to run routine that regularly add new transactions.")
	newTxDelay := flag.Int("newTxDelay", 10, "how often new txs are added.")
	ethDual := flag.Bool("dual", false, "whether to run in dual mode")
	neoDual := flag.Bool("neodual", false, "whether to run in dual mode")
	ethStat := flag.Bool("ethstat", false, "report eth stats to network")
	ethStatName := flag.String("ethstatname", "", "name to use when reporting eth stats")
	lightNode := flag.Bool("light", false, "connect to Eth as light node")
	lightServ := flag.Int("lightserv", 0, "max percentage of time serving light client reqs")
	cacheSize := flag.Int("cacheSize", 1024, "cache memory size for Eth node")
	dev := flag.Bool("dev", false, "deploy node with dev environment")
	numValid := flag.Int("numValid", 0,
		"number of total validators in dev environment. Note that this flag only has effect when --dev flag is set.")
	proposal := flag.Int("proposal", 1, "specify which node is the proposer. The index starts from 1, and every node needs to use the same proposer index. Note that this flag only has effect when --dev flag is set")
	votingStrategy := flag.String("votingStrategy", "", "specify the voting script or strategy to simulate voting. Note that this flag only has effect when --dev flag is set")
	clearDataDir := flag.Bool("clearDataDir", false, "remove contents in data dir")
	acceptTxs := flag.Int("acceptTxs", 1, "accept process tx or not, 1 is yes and 0 is no")
	// TODO(thientn): remove dualChain & dualChainValidator flags when finish development
	dualChain := flag.Bool("dualchain", false, "run dual chain for group concensus")
	dualChainNumValid := flag.Int("dualvalidators", 0, "validators for group concensus")

	flag.Parse()

	// Setups log to Stdout.
	level, err := log.LvlFromString(*logLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		level = log.LvlInfo
	}
	if len(*logTag) > 0 {
		log.Root().SetHandler(log.LvlAndTagFilterHandler(level, *logTag, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	} else {
		log.Root().SetHandler(log.LvlFilterHandler(level, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	}

	logger := log.New()

	elevel, err := elog.LvlFromString(*ethLogLevel)
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
	if len(*name) == 0 {
		logger.Error("Invalid node name", "name", *name)
	} else {
		index, err := GetNodeIndex(*name)
		if err != nil {
			logger.Error("Node name must be formmated as \"\\c*\\d{1,2}\"", "name", *name)
		}
		nodeIndex = index - 1
	}

	// Setups config.
	config := &node.DefaultConfig
	config.P2P.ListenAddr = *listenAddr
	config.Name = *name
	config.MainChainConfig.AcceptTxs = uint32(*acceptTxs)
	var devEnv *development.DevEnvironmentConfig

	if *rpcEnabled {
		if config.HTTPHost = *rpcAddr; config.HTTPHost == "" {
			config.HTTPHost = node.DefaultHTTPHost
		}
		config.HTTPPort = *rpcPort
		config.HTTPVirtualHosts = []string{"*"} // accepting RPCs from all source hosts
	}

	if *dev {
		devEnv = development.CreateDevEnvironmentConfig()
		if nodeIndex < 0 && nodeIndex >= devEnv.GetNodeSize() {
			logger.Error(fmt.Sprintf("Node index %v must be within %v and %v", nodeIndex+1, 1, devEnv.GetNodeSize()))

		}
		// Substract 1 from the index because we specify node starting from 1 onward.
		devEnv.SetProposerIndex(*proposal - 1)
		config.DevNodeConfig = devEnv.GetDevNodeConfig(nodeIndex)
		// Simulate the voting strategy
		devEnv.SetVotingStrategy(*votingStrategy)
		config.DevEnvConfig = devEnv
		config.MainChainConfig.NumValidators = *numValid

		// Setup config for kardia service
		config.MainChainConfig.ChainData = development.ChainData
		config.MainChainConfig.DbHandles = development.DbHandles
		config.MainChainConfig.DbCache = development.DbCache

		// Create genesis block with dev.genesisAccounts
		config.MainChainConfig.Genesis = blockchain.DefaulTestnetFullGenesisBlock(development.GenesisAccounts, development.GenesisContracts)
	}

	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.MainChainConfig.TxPool = *blockchain.GetDefaultTxPoolConfig(nodeDir)

	if *dualChain {
		if *dualChainNumValid > 0 {
			config.DualChainConfig.NumValidators = *dualChainNumValid
		} else {
			config.DualChainConfig.NumValidators = *numValid
		}

		config.DualChainConfig.ChainData = "dualdata"
		config.DualChainConfig.DbHandles = development.DbHandles
		config.DualChainConfig.DbCache = development.DbCache
	}

	if *clearDataDir {
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
	if *dualChain {
		n.RegisterService(kai.NewDualService)
	}
	if err := n.Start(); err != nil {
		logger.Error("Cannot start node", "err", err)
		return
	}

	var kService *kai.Kardia
	if err := n.Service(&kService); err != nil {
		logger.Error("Cannot get Kardia Service", "err", err)
		return
	}

	logger.Info("Genesis block", "genesis", *kService.BlockChain().Genesis())

	if *addTxn {
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
		txPool := kService.TxPool()
		signedTx, _ := types.SignTx(simpleTx, senderKey)

		err := txPool.AddLocal(signedTx)
		if err != nil {
			logger.Error("Txn add error", "err", err)
		}
	}

	if *addSmcCall {
		txPool := kService.TxPool()
		statedb, err := kService.BlockChain().State()
		if err != nil {
			logger.Error("Cannot get state", "state", err)
		}
		// Get first contract in genesis contracts
		/*counterSmcAddress := devEnv.GetContractAddressAt(1)
		smcAbi := devEnv.GetContractAbiByAddress(counterSmcAddress.String())
		statedb, err := kService.BlockChain().State()
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
		votingSmcAddress := development.GetContractAddressAt(0)
		votingAbiStr := development.GetContractAbiByAddress(votingSmcAddress.String())
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

	if *genNewTxs {
		go runTxCreationLoop(kService.TxPool(), *newTxDelay)
	}

	// Connect with other peers.
	if *dev {
		for i := 0; i < nodeIndex; i++ {
			peerURL := devEnv.GetDevNodeConfig(i).NodeID
			logger.Info("Adding static peer", "peerURL", peerURL)
			success, err := n.AddPeer(peerURL)
			if !success {
				logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
			}
		}
	}

	// go displayPeers(n)

	var dualP *dual.DualProcessor

	// TODO: This should trigger for either Eth dual or Neo dual flag, so  *ethDual || *neoDual
	if *ethDual || *neoDual {
		exchangeContractAddress := development.GetContractAddressAt(2)
		exchangeContractAbi := development.GetContractAbiByAddress(exchangeContractAddress.String())
		dualP, err = dual.NewDualProcessor(kService.BlockChain(), kService.TxPool(), &exchangeContractAddress, exchangeContractAbi)
		if err != nil {
			log.Error("Fail to initialize DualProcessor", "error", err)
		} else {
			dualP.Start()
		}
	}

	// Run Eth-Kardia dual node
	if *ethDual {
		config := &dual.DefaultEthKardiaConfig
		config.LightNode = *lightNode
		config.LightServ = *lightServ
		config.ReportStats = *ethStat
		if *ethStatName != "" {
			config.StatName = *ethStatName
		}
		config.CacheSize = *cacheSize

		ethNode, err := dual.NewEthKardia(config, kService.BlockChain(), kService.TxPool())
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
		// go callAmountToSend(kService.BlockChain(), kService.TxPool(), dualP)
	}

	go displayKardiaPeers(n)
	waitForever()
}

func callAmountToSend(b *blockchain.BlockChain, txPool *blockchain.TxPool, dualP *dual.DualProcessor) {
	for {
		log.Info("Polling smc")
		statedb, err := b.State()

		if err != nil {
			log.Error("Error getting state. Cannot make contract call")
		} else {
			log.Info("Preparing to tracking master smc")
		}
		senderAddr := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
		ethToSend := dualP.CallKardiaMasterGetEthToSend(senderAddr, statedb)
		log.Info("eth to send", "master smc", ethToSend)
		neoToSend := dualP.CallKardiaMasterGetNeoToSend(senderAddr, statedb)
		log.Info("neo to send", "master smc", neoToSend)
		if ethToSend.Cmp(big.NewInt(0)) > 0 {
			log.Info("There are some ETH to send, remove it")
			removeAmountToSend(b, txPool, ethToSend)
		} else {
			log.Info("There are no ETH to send, update it")
			updateAmountToSend(b, txPool)
		}
		time.Sleep(10 * time.Second)
	}
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
	tx1 := dual.CreateKardiaMatchAmountTx(caller2Key, statedb, quantity, 1)
	// txPool.AddLocal(tx1)
	log.Info("Match eth", "quantity successfully", quantity, "txhash:", tx1.Hash())

	caller3ByteK, _ := hex.DecodeString("32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe")
	caller3Key := crypto.ToECDSAUnsafe(caller3ByteK)
	tx2 := dual.CreateKardiaMatchAmountTx(caller3Key, statedb, quantity, 2)
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

	tx1 := dual.CreateKardiaRemoveAmountTx(caller2Key, statedb, quantity, 1)
	// txPool.AddLocal(tx1)
	log.Info("Remove eth", "quantity successfully", quantity, "txhash:", tx1.Hash())
	caller3ByteK, _ := hex.DecodeString("32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe")
	caller3Key := crypto.ToECDSAUnsafe(caller3ByteK)
	tx2 := dual.CreateKardiaRemoveAmountTx(caller3Key, statedb, quantity, 2)
	txs := make(types.Transactions, 2)
	txs[0] = tx1
	txs[1] = tx2
	// txPool.AddLocal(tx2)
	txPool.AddLocals(txs)
	log.Info("Remove neo", "quantity successfully", quantity, "txhash:", tx2.Hash())
}

func displayEthPeers(n *dual.EthKardia) {
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

func displaySyncStatus(client *dual.KardiaEthClient) {
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
