package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualchain/service"
	"github.com/kardiachain/go-kardia/dualnode/dual_proxy"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/p2p/nat"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	kai "github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"
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
func (c *Config)getP2PConfig() (*p2p.Config, error) {
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
		PrivateKey:      privKey,
		MaxPeers:        peer.MaxPeers,
		ListenAddr:      peer.ListenAddress,
		NAT:             nat.Any(),
	}, nil
}

// getDbInfo gets database information from config. Currently, it only supports levelDb and Mondodb
func (c *Config)getDbInfo(isDual bool) storage.DbInfo {
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
func (c *Config)getTxPoolConfig() tx_pool.TxPoolConfig {
	txPool := c.MainChain.TxPool
	return tx_pool.TxPoolConfig{
		GlobalSlots:  txPool.GlobalSlots,
		GlobalQueue:  txPool.GlobalQueue,

		NumberOfWorkers: txPool.NumberOfWorkers,
		WorkerCap: txPool.WorkerCap,
		BlockSize: txPool.BlockSize,
	}
}

// getGenesis gets genesis data from config
func (c *Config)getGenesis(isDual bool) (*genesis.Genesis, error) {
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
func (c *Config)getMainChainConfig() (*node.MainChainConfig, error) {
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
		EnvConfig:        nil,
		BaseAccount:      baseAccount,
	}
	return &mainChainConfig, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config)getDualChainConfig() (*node.DualChainConfig, error) {
	dbInfo := c.getDbInfo(true)
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesis(true)
	if err != nil {
		return nil, err
	}
	eventPool := event_pool.Config{
		GlobalSlots:     c.DualChain.EventPool.GlobalSlots,
		GlobalQueue:     c.DualChain.EventPool.GlobalQueue,
		NumberOfWorkers: c.DualChain.EventPool.NumberOfWorkers,
		WorkerCap:       c.DualChain.EventPool.WorkerCap,
		BlockSize:       c.DualChain.EventPool.BlockSize,
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
		EnvConfig:        nil,
		BaseAccount:      baseAccount,
	}
	return &dualChainConfig, nil
}

// getNodeConfig gets NodeConfig from config
func (c *Config)getNodeConfig() (*node.NodeConfig, error) {
	n := c.Node
	p2pConfig, err := c.getP2PConfig()
	if err != nil {
		return nil, err
	}
	p2pConfig.Name = n.Name
	nodeConfig := node.NodeConfig{
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
func (c *Config)newLog() log.Logger {
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
func (c *Config)Start() {
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
	n, err := node.NewNode(nodeConfig)
	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	if err := n.RegisterService(kai.NewKardiaService); err != nil {
		logger.Error("error while adding kardia service", "err", err)
		return
	}

	if c.DualChain != nil {
		if err := n.RegisterService(service.NewDualService); err != nil {
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
		if err := n.AddPeer(peer); err != nil {
			logger.Error("error while adding peer", "err", err, "peer", peer)
		}
	}

	if c.MainChain.Events != nil {
		var kardiaService *kai.KardiaService
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
			if err := n.AddPeer(peer); err != nil {
				logger.Error("error while adding peer", "err", err, "peer", peer)
			}
		}
	}

	if err := c.StartDual(n); err != nil {
		logger.Error("error while starting dual", "err", err)
		return
	}

	if err := n.StartServiceRPC(); err != nil {
		logger.Error("Fail to start RPC", "err", err)
		return
	}

	go displayKardiaPeers(n)
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
			for _, action := range event.Watchers{
				watchers = append(watchers, &types.Watcher{
					Method:     action.Method,
					WatcherActions: action.WatcherActions,
					DualActions: action.DualActions,
				})
			}
			smc := &types.KardiaSmartcontract{
				MasterSmc:      event.MasterSmartContract,
				MasterAbi:      masterAbi,
				SmcAddress:     event.ContractAddress,
				SmcAbi:         abi,
				Watchers:       watchers,
			}
			service.DB().WriteEvent(smc)
		}
	}
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
		if err = os.RemoveAll(filepath.Join(dir, name)); err  != nil {
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
