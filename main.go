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
	dualMode := flag.Bool("dual", false, "whether to run in dual mode")
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

	flag.Parse()

	// Setups log to Stdout.
	level, err := log.LvlFromString(*logLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		level = log.LvlInfo
	}
	log.Root().SetHandler(log.LvlFilterHandler(level, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

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
	config.AcceptTxs = uint32(*acceptTxs)
	var devEnv *development.DevEnvironmentConfig

	if *rpcEnabled {
		if config.HTTPHost = *rpcAddr; config.HTTPHost == "" {
			config.HTTPHost = node.DefaultHTTPHost
		}
		config.HTTPPort = *rpcPort
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
		config.NumValidators = *numValid

		// Setup config for kardia service
		config.ChainData = development.ChainData
		config.DbHandles = development.DbHandles
		config.DbCache = development.DbCache

		// Create genesis block with dev.genesisAccounts
		config.Genesis = blockchain.DefaulTestnetFullGenesisBlock(development.GenesisAccounts, development.GenesisContracts)
	}

	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.TxPool = *blockchain.GetDefaultTxPoolConfig(nodeDir)

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
		votingSmcAddress := devEnv.GetContractAddressAt(0)
		votingAbiStr := devEnv.GetContractAbiByAddress(votingSmcAddress.String())
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
		if err!= nil {
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

	if *dualMode {
		config := &dual.DefaultEthKardiaConfig
		config.LightNode = *lightNode
		config.LightServ = *lightServ
		config.ReportStats = *ethStat
		if *ethStatName != "" {
			config.StatName = *ethStatName
		}
		config.CacheSize = *cacheSize

		ethNode, err := dual.NewEthKardia(config)
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
		go displaySyncStatus(client)
	}

	go displayKardiaPeers(n)
	waitForever()
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
