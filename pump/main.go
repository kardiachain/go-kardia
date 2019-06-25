package main

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

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/rs/cors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	"github.com/kardiachain/go-kardia/mainchain"
	bc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
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

	// Development's related flags
	dev            bool
	proposal       int
	votingStrategy string
	numTxs         int
	txsDelay       int
	index          int
	genTxsPort     string
}

type Response struct {
	NumTxs   int             `json:"numTxs"`
	Delay    int             `json:"delay"`
	Accounts []Account       `json:"accounts"`
	Pending  int64           `json:"pending"`
}

type Tps struct {
	Blocks     uint64     `json:"blocks"`
	BlockTime  int64   `json:"blockTime"`
	Txs        uint64     `json:"txs"`
	Tps        float64 `json:"tps"`
}

var args flagArgs
var accounts = make([]Account, 0)
var genTool *GeneratorTool
var blockchain *bc.BlockChain
var kardiaService *kai.KardiaService

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

	// NOTE: The flags below are only applicable for dev environment. Please add the applicable ones
	// here and DO NOT add non-dev flags.
	flag.BoolVar(&args.dev, "dev", false, "deploy node with dev environment")
	flag.StringVar(&args.votingStrategy, "votingStrategy", "",
		"specify the voting script or strategy to simulate voting. Note that this flag only has effect when --dev flag is set")
	flag.IntVar(&args.proposal, "proposal", 1,
		"specify which node is the proposer. The index starts from 1, and every node needs to use the same proposer index."+
			" Note that this flag only has effect when --dev flag is set")
	flag.IntVar(&args.maxPeers, "maxpeers", 25,
		"maximum number of network peers (network disabled if set to 0. Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.noProxy, "noProxy", false, "When triggered, Kardia node is standalone and is not registered in proxy.")
	flag.StringVar(&args.peerProxyIP, "peerProxyIP", "", "IP of the peer proxy for this node to register.")
	flag.IntVar(&args.numTxs, "numTxs", 0, "number of of generated txs in one batch")
	flag.IntVar(&args.txsDelay, "txsDelay", 1000, "delay in seconds between batches of generated txs")
	flag.StringVar(&args.genTxsPort,"genTxsPort",":5000", "port of generate tx")
	flag.IntVar(&args.index, "index", 1, "")
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
		config.MainChainConfig.Genesis = genesis.DefaulTestnetFullGenesisBlock(GenesisAccounts, GenesisContracts)
	}
	nodeDir := filepath.Join(config.DataDir, config.Name)
	config.MainChainConfig.TxPool = tx_pool.TxPoolConfig{
		Journal:   filepath.Join(nodeDir, "transactions.rlp"),
		Rejournal: time.Hour,
		PriceLimit:   1,
		PriceBump:    10,
		AccountSlots: 16,
		GlobalSlots:  65536, // for pending
		AccountQueue: 64,
		GlobalQueue:  131072, // for all
		Lifetime: 3 * time.Hour,
		NumberOfWorkers: 32,
		WorkerCap: 2048,
		BlockSize: 16384,
	}
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
	if err := n.Start(); err != nil {
		logger.Error("Cannot start node", "err", err)
		return
	}

	if err := n.Service(&kardiaService); err != nil {
		logger.Error("Cannot get Kardia Service", "err", err)
		return
	}
	blockchain = kardiaService.BlockChain()
	logger.Info("Genesis block", "genesis", *blockchain.Genesis())

	if !args.noProxy && len(args.peerProxyIP) == 0 {
		logger.Error("flag noProxy=false but peerProxyIP is empty, will ignore proxy.")
		args.noProxy = true // TODO(thientn): removes when finish cleaning up proxy.
	}

	if !args.noProxy {
		if err := n.CallProxy("Startup", n.Server().Self(), nil); err != nil {
			logger.Error("Error when startup proxy connection", "err", err)
		}
	}

	if args.dev {
		for i := 0; i < config.MainChainConfig.EnvConfig.GetNodeSize(); i++ {
			peerURL := config.MainChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
			logger.Info("Adding static peer", "peerURL", peerURL)
			if err := n.AddPeer(peerURL); err != nil {
				log.Error("Error adding static peer", "err", err)
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

	// Start RPC for all services
	if args.rpcEnabled {
		err := n.StartServiceRPC()
		if err != nil {
			logger.Error("Fail to start RPC", "err", err)
			return
		}
	}

	go displayKardiaPeers(n)

	// get accounts
	idx := args.index
	if idx == 1 {
		accounts = GetAccounts(GenesisAddrKeys1)
	} else if idx == 2 {
		accounts = GetAccounts(GenesisAddrKeys2)
	} else if idx == 3 {
		accounts = GetAccounts(GenesisAddrKeys3)
	} else if idx == 4 {
		accounts = GetAccounts(GenesisAddrKeys4)
	} else if idx == 5 {
		accounts = GetAccounts(GenesisAddrKeys5)
	} else if idx == 6 {
		accounts = GetAccounts(GenesisAddrKeys6)
	} else if idx == 7 {
		accounts = GetAccounts(GenesisAddrKeys7)
	} else if idx == 8 {
		accounts = GetAccounts(GenesisAddrKeys8)
	} else if idx == 9 {
		accounts = GetAccounts(GenesisAddrKeys9)
	} else if idx == 10 {
		accounts = GetAccounts(GenesisAddrKeys10)
	}

	// gen txs from args.numTxs
	go genTxsLoop(kardiaService.TxPool())

	// start an api that receives pump configure
	go func(){
		router := mux.NewRouter()
		router.HandleFunc("/pump", pump).Methods("POST")
		router.HandleFunc("/status", status).Methods("GET")
		router.HandleFunc("/tps", tps).Methods("GET")
		router.HandleFunc("/config", configPool).Methods("GET")

		if err := http.ListenAndServe(args.genTxsPort, cors.AllowAll().Handler(router)); err != nil {
			panic(err)
		}
	}()

	waitForever()
}

func displayKardiaPeers(n *node.Node) {
	for {
		log.Info("Kardia peers: ", "count", n.Server().PeerCount())
		time.Sleep(20 * time.Second)
	}
}

func waitForever() {
	select {}
}

// genTxsLoop generate & add a batch of transfer txs, repeat after delay flag.
// Warning: Set txsDelay < 5 secs may build up old subroutines because previous subroutine to add txs won't be finished before new one starts.
func genTxsLoop(txPool *tx_pool.TxPool) {
	time.Sleep(30 * time.Second) //decrease it if you want to test it locally
	genTool = NewGeneratorTool(accounts, make(map[string]uint64))
	for {
		genTxs(genTool, uint64(args.numTxs), txPool)
		time.Sleep(time.Duration(args.txsDelay) * time.Second)
	}
}

func genTxs(genTool *GeneratorTool, numTxs uint64, txPool *tx_pool.TxPool) {
	txList := genTool.GenerateRandomTxWithState(numTxs, txPool.State())
	log.Info("GenTxs Adding new transactions", "num", numTxs, "generatedTxList", len(txList), "pendingPool", txPool.PendingSize())
	if err := txPool.AddTxs(txList); err != nil {
		log.Error("Error while adding txs", "err", err)
	}
}

func pump(w http.ResponseWriter, r *http.Request) {
	log.Info("pumping txs")
	data, err := HandlePost(r)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("%v", err))
		return
	}
	m := data.(map[string]interface{})
	_, ok := m["numTxs"]
	if !ok {
		respondWithError(w, 500, fmt.Sprintf("numTxs is required"))
		return
	}

	// get numTx
	numTxs := int(m["numTxs"].(float64))
	if numTxs < 0 {
		respondWithError(w, 500, fmt.Sprintf("invalid numTxs %v", numTxs))
		return
	}

	delay := args.txsDelay
	_, ok = m["delay"]
	if ok {
		delay = int(m["delay"].(float64))
		if delay <= 0 {
			respondWithError(w, 500, fmt.Sprintf("invalid delay %v", delay))
			return
		}
	}

	idx, genesisIdx := m["index"]
	if genesisIdx {
		if idx == "1" {
			accounts = GetAccounts(GenesisAddrKeys1)
		} else if idx == "2" {
			accounts = GetAccounts(GenesisAddrKeys2)
		} else if idx == "3" {
			accounts = GetAccounts(GenesisAddrKeys3)
		} else if idx == "4" {
			accounts = GetAccounts(GenesisAddrKeys4)
		} else if idx == "5" {
			accounts = GetAccounts(GenesisAddrKeys5)
		} else if idx == "6" {
			accounts = GetAccounts(GenesisAddrKeys6)
		} else if idx == "7" {
			accounts = GetAccounts(GenesisAddrKeys7)
		} else if idx == "8" {
			accounts = GetAccounts(GenesisAddrKeys8)
		} else if idx == "9" {
			accounts = GetAccounts(GenesisAddrKeys9)
		} else if idx == "10" {
			accounts = GetAccounts(GenesisAddrKeys10)
		} else {
			respondWithError(w, 500, "invalid genesis index")
			return
		}
	} else {
		_, ok = m["accounts"]
		if !ok && len(accounts) == 0 {
			respondWithError(w, 500, "accounts are required")
			return
		}

		accs := m["accounts"].([]interface{})
		if len(accs) == 0 {
			respondWithError(w, 500, "accounts cannot be empty")
			return
		}

		// reset accounts
		accounts = make([]Account, 0)
		for _, acc := range accs {
			m := acc.(map[string]interface{})
			account := Account{
				Address: m["address"].(string),
				PrivateKey: m["privateKey"].(string),
			}
			accounts = append(accounts, account)
		}
	}

	args.numTxs = int(numTxs)
	args.txsDelay = int(delay)
	genTool = NewGeneratorTool(accounts, genTool.nonceMap)

	respondWithJSON(w, 200, "OK")
}

func status(w http.ResponseWriter, r *http.Request) {

	response := Response{
		NumTxs: args.numTxs,
		Delay: args.txsDelay,
		Accounts: accounts,
		Pending: kardiaService.TxPool().PendingSize(),
	}

	respondWithJSON(w, 200, response)
}

func tps(w http.ResponseWriter, r *http.Request) {
	result := make([]Tps, 0)

	blocks, err := strconv.ParseInt(r.FormValue("blocks"), 10, 64)
	if err != nil {
		blocks = 5
	}

	currentHeight := blockchain.CurrentBlock().Height()
	blockTime := int64(0)
	numTxs := uint64(0)

	for {
		if blocks == 0 || currentHeight == 1 {
			break
		}
		// get block by height
		block := blockchain.GetBlockByHeight(currentHeight)
		previousBlock := blockchain.GetBlockByHeight(currentHeight - 1)

		currentBlockTime := block.Time().Int64()
		previousBlockTime := previousBlock.Time().Int64()

		// calculate blocktime and numtxs
		blockTime += currentBlockTime - previousBlockTime
		numTxs += block.NumTxs()

		blocks--
		currentHeight--
	}

	result = append(result, Tps{
		Blocks: uint64(blocks),
		BlockTime: blockTime,
		Txs: numTxs,
		Tps: float64(int64(numTxs)/blockTime),
	})

	respondWithJSON(w, 200, result)
}

func configPool(w http.ResponseWriter, r *http.Request) {
	result := make([]Tps, 0)

	workers, err := strconv.ParseInt(r.FormValue("workers"), 10, 64)
	if err != nil {
		workers = 5
	}

	workerCap, err := strconv.ParseInt(r.FormValue("cap"), 10, 64)
	if err != nil {
		workerCap = 600
	}

	kardiaService.TxPool().ResetWorker(int(workers), int(workerCap))
	respondWithJSON(w, 200, result)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// HandlePost handles post data from http.Request and return data as json format
func HandlePost(r *http.Request) (interface{}, error) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
