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
	"crypto/ecdsa"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"gopkg.in/yaml.v2"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/dualchain/blockchain"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/dualchain/service"
	"github.com/kardiachain/go-kardiamain/dualnode/dual_proxy"
	"github.com/kardiachain/go-kardiamain/dualnode/kardia"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/p2p/enode"
	"github.com/kardiachain/go-kardiamain/lib/p2p/nat"
	"github.com/kardiachain/go-kardiamain/lib/sysutils"
	kai "github.com/kardiachain/go-kardiamain/mainchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/node"
	"github.com/kardiachain/go-kardiamain/tool"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	LevelDb = iota
	MongoDb
)

type flags struct {
	config string
}

func initFlag(args *flags) {
	flag.StringVar(&args.config, "config", "", "path to config file, if config is defined then it is priority used.")
}

var args flags

func init() {
	initFlag(&args)
}

// Load attempts to load the config from given path and filename.
func LoadConfig(path string) (*Config, error) {
	configPath := filepath.Join(path)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "Unable to load config")
	}
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read config")
	}
	config := Config{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return nil, errors.Wrap(err, "Problem unmarshaling config json data")
	}
	return &config, nil
}

// getP2P gets p2p's config from config
func (c *Config) getP2PConfig() (*p2p.Config, error) {
	peer := c.P2P
	var privKey *ecdsa.PrivateKey
	var err error

	if peer.PrivateKey != "" {
		privKey, err = crypto.HexToECDSA(peer.PrivateKey)
	} else {
		privKey, err = crypto.GenerateKey()
	}
	if err != nil {
		return nil, err
	}
	return &p2p.Config{
		PrivateKey: privKey,
		MaxPeers:   peer.MaxPeers,
		ListenAddr: peer.ListenAddress,
		NAT:        nat.Any(),
	}, nil
}

// getDbInfo gets database information from config. Currently, it only supports levelDb and Mondodb
func (c *Config) getDbInfo(isDual bool) storage.DbInfo {
	database := c.MainChain.Database
	if isDual {
		database = c.DualChain.Database
	}
	switch database.Type {
	case LevelDb:
		nodeDir := filepath.Join(c.DataDir, c.Name, database.Dir)
		if database.Drop == 1 {
			// Clear all contents within data dir
			if err := removeDirContents(nodeDir); err != nil {
				panic(err)
			}
		}
		return storage.NewLevelDbInfo(nodeDir, database.Caches, database.Handles)
	case MongoDb:
		return storage.NewMongoDbInfo(database.URI, database.Name, database.Drop == 1)
	default:
		return nil
	}
}

// getTxPoolConfig gets txPoolConfig from config
func (c *Config) getTxPoolConfig() tx_pool.TxPoolConfig {
	txPool := c.MainChain.TxPool
	return tx_pool.TxPoolConfig{
		GlobalSlots: txPool.GlobalSlots,
		GlobalQueue: txPool.GlobalQueue,
	}
}

// getGenesis gets genesis data from config
func (c *Config) getGenesis(isDual bool) (*genesis.Genesis, error) {
	var ga genesis.GenesisAlloc
	var err error
	g := c.MainChain.Genesis
	if isDual {
		g = c.DualChain.Genesis
	}
	if g == nil {
		ga = make(genesis.GenesisAlloc, 0)
	} else {
		genesisAccounts := make(map[string]*big.Int)
		genesisContracts := make(map[string]string)

		amount, _ := big.NewInt(0).SetString(g.GenesisAmount, 10)
		for _, address := range g.Addresses {
			genesisAccounts[address] = amount
		}

		for _, contract := range g.Contracts {
			genesisContracts[contract.Address] = contract.ByteCode
		}
		ga, err = genesis.GenesisAllocFromAccountAndContract(genesisAccounts, genesisContracts)
		if err != nil {
			return nil, err
		}
	}
	return &genesis.Genesis{
		Config:   configs.TestnetChainConfig,
		GasLimit: 16777216, // maximum number of uint24
		Alloc:    ga,
	}, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config) getMainChainConfig() (*node.MainChainConfig, error) {
	chain := c.MainChain
	dbInfo := c.getDbInfo(false)
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesis(false)
	if err != nil {
		return nil, err
	}
	baseAccount, err := c.getBaseAccount(false)
	if err != nil {
		return nil, err
	}
	mainChainConfig := node.MainChainConfig{
		ValidatorIndexes: c.MainChain.Validators,
		DBInfo:           dbInfo,
		Genesis:          genesisData,
		TxPool:           c.getTxPoolConfig(),
		AcceptTxs:        chain.AcceptTxs,
		IsZeroFee:        chain.ZeroFee == 1,
		NetworkId:        chain.NetworkID,
		ChainId:          chain.ChainID,
		ServiceName:      chain.ServiceName,
		BaseAccount:      baseAccount,
	}
	return &mainChainConfig, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config) getDualChainConfig() (*node.DualChainConfig, error) {
	dbInfo := c.getDbInfo(true)
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesis(true)
	if err != nil {
		return nil, err
	}
	eventPool := event_pool.Config{
		GlobalSlots: c.DualChain.EventPool.GlobalSlots,
		GlobalQueue: c.DualChain.EventPool.GlobalQueue,
		BlockSize:   c.DualChain.EventPool.BlockSize,
	}

	baseAccount, err := c.getBaseAccount(true)
	if err != nil {
		return nil, err
	}

	dualChainConfig := node.DualChainConfig{
		ValidatorIndexes: c.DualChain.Validators,
		DBInfo:           dbInfo,
		DualGenesis:      genesisData,
		DualEventPool:    eventPool,
		DualNetworkID:    c.DualChain.NetworkID,
		ChainId:          c.DualChain.ChainID,
		DualProtocolName: *c.DualChain.Protocol,
		BaseAccount:      baseAccount,
	}
	return &dualChainConfig, nil
}

// getNodeConfig gets NodeConfig from config
func (c *Config) getNodeConfig() (*node.Config, error) {
	n := c.Node
	p2pConfig, err := c.getP2PConfig()
	if err != nil {
		return nil, err
	}
	p2pConfig.Name = n.Name
	nodeConfig := node.Config{
		Name:             n.Name,
		DataDir:          n.DataDir,
		P2P:              *p2pConfig,
		HTTPHost:         n.HTTPHost,
		HTTPPort:         n.HTTPPort,
		HTTPCors:         n.HTTPCors,
		HTTPVirtualHosts: n.HTTPVirtualHosts,
		HTTPModules:      n.HTTPModules,
		MainChainConfig:  node.MainChainConfig{},
		DualChainConfig:  node.DualChainConfig{},
		PeerProxyIP:      "",
		Metrics:          n.Metrics,
	}
	mainChainConfig, err := c.getMainChainConfig()
	if err != nil {
		return nil, err
	}
	if mainChainConfig == nil {
		return nil, fmt.Errorf("mainChainConfig is empty")
	}
	nodeConfig.MainChainConfig = *mainChainConfig
	if c.DualChain != nil {
		if dualChainConfig, err := c.getDualChainConfig(); err != nil {
			return nil, err
		} else {
			nodeConfig.DualChainConfig = *dualChainConfig
		}
	}
	return &nodeConfig, nil
}

// newLog inits new logger for kardia
func (c *Config) newLog() log.Logger {
	// Setups log to Stdout.
	level, err := log.LvlFromString(c.LogLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		level = log.LvlInfo
	}
	log.Root().SetHandler(log.LvlFilterHandler(level,
		log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
	return log.New()
}

// getBaseAccount gets base account that is used to execute internal smart contract
func (c *Config) getBaseAccount(isDual bool) (*types.BaseAccount, error) {
	var privKey *ecdsa.PrivateKey
	var err error
	var address common.Address

	if isDual {
		address = common.HexToAddress(c.DualChain.BaseAccount.Address)
		privKey, err = crypto.HexToECDSA(c.DualChain.BaseAccount.PrivateKey)
	} else {
		address = common.HexToAddress(c.MainChain.BaseAccount.Address)
		privKey, err = crypto.HexToECDSA(c.MainChain.BaseAccount.PrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("baseAccount: Invalid privatekey: %v", err)
	}
	return &types.BaseAccount{
		Address:    address,
		PrivateKey: *privKey,
	}, nil
}

// Start starts chain with given config
func (c *Config) Start() {
	logger := c.newLog()

	// System settings
	if err := runtimeSystemSettings(); err != nil {
		logger.Error("Fail to update system settings", "err", err)
		return
	}

	// get nodeConfig from config
	nodeConfig, err := c.getNodeConfig()
	if err != nil {
		logger.Error("Cannot get node config", "err", err)
		return
	}

	// init new node from nodeConfig
	n, err := node.New(nodeConfig)
	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	if err := n.Register(kai.NewKardiaService); err != nil {
		logger.Error("error while adding kardia service", "err", err)
		return
	}

	if c.DualChain != nil {
		if err := n.Register(service.NewDualService); err != nil {
			logger.Error("error while adding dual service", "err", err)
			return
		}
	}

	if err := n.Start(); err != nil {
		logger.Error("error while starting node", "err", err)
		return
	}

	// Add peers
	for _, peer := range c.MainChain.Seeds {
		n.Server().AddPeer(enode.MustParse(peer))
	}
	var kardiaService *kai.KardiaService

	if c.MainChain.Events != nil {
		if err := n.Service(&kardiaService); err != nil {
			logger.Error("cannot get Kardia service", "err", err)
			return
		}
		// save watchers to db
		c.SaveWatchers(kardiaService, c.MainChain.Events)
	}

	if c.DualChain != nil {
		// Add peers
		for _, peer := range c.DualChain.Seeds {
			n.Server().AddPeer(enode.MustParse(peer))
		}
	}

	if err := c.StartDual(n); err != nil {
		logger.Error("error while starting dual", "err", err)
		return
	}

	go displayKardiaPeers(n)

	if err := c.StartDebug(); err != nil {
		logger.Error("Failed to start debug", "err", err)
	}

	if err := c.StartPump(kardiaService.TxPool()); err != nil {
		logger.Error("Failed to start pump txs", "err", err)
	}

	waitForever()
}

// StartDual reads dual config and start dual service
func (c *Config) StartDual(n *node.Node) error {
	if c.DualChain != nil {
		var kardiaService *kai.KardiaService
		var dualService *service.DualService
		var dualProxy *dual_proxy.Proxy
		var err error

		if err = n.Service(&kardiaService); err != nil {
			return fmt.Errorf("cannot get Kardia service: %v", err)
		}

		if err = n.Service(&dualService); err != nil {
			return fmt.Errorf("cannot get Dual service: %v", err)
		}

		// save watchers to db
		if c.DualChain.Events != nil {
			c.SaveWatchers(dualService, c.DualChain.Events)
		}

		// init kardia proxy
		kardiaProxy := &kardia.KardiaProxy{}
		if err = kardiaProxy.Init(kardiaService.BlockChain(), kardiaService.TxPool(),
			dualService.BlockChain(), dualService.EventPool(), nil, nil); err != nil {
			panic(err)
		}

		if dualProxy, err = dual_proxy.NewProxy(
			c.DualChain.ServiceName,
			kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			*c.DualChain.PublishedEndpoint,
			*c.DualChain.SubscribedEndpoint,
		); err != nil {
			log.Error("Fail to initialize proxy", "error", err, "proxy", c.DualChain.ServiceName)
			return err
		}

		// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
		// submit tx to either internal or external blockchain.
		bcManager := blockchain.NewDualBlockChainManager(kardiaProxy, dualProxy)
		dualService.SetDualBlockChainManager(bcManager)

		// Register the 'other' blockchain to each internal/external blockchain. This is needed
		// for generate Tx to submit to the other blockchain.
		kardiaProxy.RegisterExternalChain(dualProxy)
		dualProxy.RegisterInternalChain(kardiaProxy)

		dualProxy.Start()
		kardiaProxy.Start()
	}
	return nil
}

func (c *Config) SaveWatchers(service node.Service, events []Event) {
	if events != nil {
		for _, event := range events {
			abi := ""
			masterAbi := ""
			if event.ABI != nil {
				abi = *event.ABI
			}
			if event.MasterABI != nil {
				masterAbi = *event.MasterABI
			}
			watchers := make(types.Watchers, 0)
			for _, action := range event.Watchers {
				watchers = append(watchers, &types.Watcher{
					Method:         action.Method,
					WatcherActions: action.WatcherActions,
					DualActions:    action.DualActions,
				})
			}
			smc := &types.KardiaSmartcontract{
				MasterSmc:  event.MasterSmartContract,
				MasterAbi:  masterAbi,
				SmcAddress: event.ContractAddress,
				SmcAbi:     abi,
				Watchers:   watchers,
			}
			service.DB().WriteEvent(smc)
		}
	}
}

// StartPump reads dual config and start dual service
func (c *Config) StartPump(txPool *tx_pool.TxPool) error {
	if c.GenTxs != nil {
		go genTxsLoop(c.GenTxs, txPool, c.MainChain.TxPool.GlobalQueue)
	} else {
		return fmt.Errorf("cannot start pump txs: %v", c.GenTxs)
	}
	return nil
}

// genTxsLoop generate & add a batch of transfer txs, repeat after delay flag.
// Warning: Set txsDelay < 5 secs may build up old subroutines because previous subroutine to add txs won't be finished before new one starts.
func genTxsLoop(genTxs *GenTxs, txPool *tx_pool.TxPool, globalQueue uint64) {
	time.Sleep(15 * time.Second) //decrease it if you want to test it locally
	var accounts = make([]tool.Account, 0)
	// get accounts
	switch genTxs.Index {
	case 1:
		accounts = tool.GetAccounts(GenesisAddrKeys1)
	case 2:
		accounts = tool.GetAccounts(GenesisAddrKeys2)
	case 3:
		accounts = tool.GetAccounts(GenesisAddrKeys3)
	case 4:
		accounts = tool.GetAccounts(GenesisAddrKeys4)
	case 5:
		accounts = tool.GetAccounts(GenesisAddrKeys5)
	case 6:
		accounts = tool.GetAccounts(GenesisAddrKeys6)
	case 7:
		accounts = tool.GetAccounts(GenesisAddrKeys7)
	case 8:
		accounts = tool.GetAccounts(GenesisAddrKeys8)
	case 9:
		accounts = tool.GetAccounts(GenesisAddrKeys9)
	case 10:
		accounts = tool.GetAccounts(GenesisAddrKeys10)
	case 11:
		accounts = tool.GetAccounts(GenesisAddrKeys11)
	case 12:
		accounts = tool.GetAccounts(GenesisAddrKeys12)
	default:
		accounts = tool.GetAccounts(GenesisAddrKeys1)
	}
	genTool := tool.NewGeneratorTool(accounts)
	initHeight := txPool.GetBlockChain().CurrentBlock().Height()
	for {
		if genTxs.NumTxs == 0 {
			break
		}

		height := txPool.GetBlockChain().CurrentBlock().Height()
		pendingSize := txPool.PendingSize()
		// Let's assume that current height is greater than oldHeight, continue generate txs
		if height > initHeight && uint64(pendingSize) < globalQueue {
			initHeight = height
			generateTxs(genTxs, genTool, txPool)
		} else {
			log.Warn("Skip GenTxs due to height or max pending txs", "prevHeight", initHeight, "currentHeight", height, "pending", pendingSize)
		}

		time.Sleep(time.Duration(genTxs.Delay) * time.Second)
	}
}

func generateTxs(genTxs *GenTxs, genTool *tool.GeneratorTool, txPool *tx_pool.TxPool) {
	var txList types.Transactions
	// Depends on generate txs
	switch genTxs.Type {
	case tool.DefaultGenRandomWithStateTx:
		txList = genTool.GenerateRandomTxWithAddressState(genTxs.NumTxs, txPool)
	case tool.DefaultGenRandomTx:
		txList = genTool.GenerateRandomTx(genTxs.NumTxs)
	}
	txPool.AddLocals(txList)
	log.Info("GenTxs Adding new transactions", "num", genTxs.NumTxs, "genType", genTxs.Type)
}

// StartPump reads dual config and start dual service
func (c *Config) StartDebug() error {
	if c.Debug != nil {
		go func() {
			router := mux.NewRouter()
			router.HandleFunc("/debug/pprof/", pprof.Index)
			router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			router.HandleFunc("/debug/pprof/profile", pprof.Profile)
			router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			router.HandleFunc("/debug/pprof/trace", pprof.Trace)
			router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
			router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
			router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
			router.Handle("/debug/pprof/block", pprof.Handler("block"))
			router.Handle("/debug/vars", http.DefaultServeMux)

			if err := http.ListenAndServe(c.Debug.Port, cors.AllowAll().Handler(router)); err != nil {
				panic(err)
			}
		}()
	} else {
		return fmt.Errorf("cannot start debug: %v", c.Debug)
	}
	return nil
}

// removeDirContents deletes old local node directory
func removeDirContents(dir string) error {
	var err error
	var directory *os.File

	log.Info("Remove directory", "dir", dir)
	if _, err = os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			log.Info("Directory does not exist", "dir", dir)
			return nil
		} else {
			return err
		}
	}
	if directory, err = os.Open(dir); err != nil {
		return err
	}

	defer directory.Close()

	var dirNames []string
	if dirNames, err = directory.Readdirnames(-1); err != nil {
		return err
	}
	for _, name := range dirNames {
		if err = os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// runtimeSystemSettings optimizes process setting for go-kardia
func runtimeSystemSettings() error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	limit, err := sysutils.FDCurrent()
	if err != nil {
		return err
	}
	if limit < 2048 { // if rlimit is less than 2048 try to raise it to 2048
		if err := sysutils.FDRaise(2048); err != nil {
			return err
		}
	}
	return nil
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

func main() {
	flag.Parse()
	if args.config != "" {
		config, err := LoadConfig(args.config)
		if err != nil {
			panic(err)
		}
		config.Start()
	}
}
