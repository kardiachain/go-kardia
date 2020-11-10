// Package main
package main

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"gopkg.in/yaml.v2"

	"github.com/kardiachain/go-kardiamain/configs"
	_default "github.com/kardiachain/go-kardiamain/configs/default"
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
	"github.com/kardiachain/go-kardiamain/dualchain/blockchain"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/dualchain/service"
	"github.com/kardiachain/go-kardiamain/dualnode/dual_proxy"
	"github.com/kardiachain/go-kardiamain/dualnode/kardia"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	kai "github.com/kardiachain/go-kardiamain/mainchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/node"
	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
)

// Config read system configuration and start load require modules for network
type Config struct {
	typesCfg.Node `yaml:"Node"`
	MainChain     *typesCfg.Chain `yaml:"MainChain"`
	DualChain     *typesCfg.Chain `yaml:"DualChain,omitempty"`
}

// Load attempts to load the config from given path and filename.
func LoadConfig(args flags) (*Config, error) {
	// Read custom flag for target network
	finalizeConfigParams(&args)
	config := loadDefaultMainnet()
	if err := loadMainnetConfig(args, config); err != nil {
		return nil, err
	}

	if args.network == Testnet {
		if err := loadTestnetConfig(args, config); err != nil {
			return nil, err
		}
	}

	if args.network == Devnet {
		if err := loadTestnetConfig(args, config); err != nil {
			return nil, err
		}
		if err := loadDevnetConfig(args, config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func loadDefaultMainnet() *Config {
	defaultCfg := &Config{
		MainChain: _default.Chain(),
	}
	defaultCfg.Node = _default.Node()
	return defaultCfg
}

func loadMainnetConfig(args flags, cfg *Config) error {
	args.network = Mainnet
	return loadConfigFile(args, cfg)
}

func loadTestnetConfig(args flags, cfg *Config) error {
	args.network = Testnet
	return loadConfigFile(args, cfg)
}

func loadDevnetConfig(args flags, cfg *Config) error {
	args.network = Devnet
	return loadConfigFile(args, cfg)
}

func loadConfigFile(args flags, cfg *Config) error {
	finalizeConfigParams(&args)
	var (
		wd  string
		err error
	)
	wd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("Read config with args", args)
	genesisCfgFilePath := filepath.Join(wd, args.genesis)
	genesisCfgData, err := ioutil.ReadFile(genesisCfgFilePath)
	if err == nil { // Read when input files exist, otherwise
		err = yaml.Unmarshal(genesisCfgData, &cfg)
		if err != nil {
			return errors.Wrap(err, "cannot unmarshal node config")
		}
	}

	// Parse kai node config
	kaiCfgFilePath := filepath.Join(wd, args.kardia)
	kaiCfgData, err := ioutil.ReadFile(kaiCfgFilePath)
	if err != nil {
		return errors.Wrap(err, "cannot read kai config")
	}
	err = yaml.Unmarshal(kaiCfgData, &cfg)
	if err != nil {
		return errors.Wrap(err, "cannot unmarshal kai config")
	}
	cfg.Genesis = cfg.MainChain.Genesis
	if args.dual != "" {
		chainCfgFile := filepath.Join(wd, args.dual)
		chainCfg, err := ioutil.ReadFile(chainCfgFile)
		if err != nil {
			return errors.Wrap(err, "cannot read dual node config")
		}
		err = yaml.Unmarshal(chainCfg, &cfg.DualChain)
		if err != nil {
			return errors.Wrap(err, "cannot unmarshal dual node config")
		}
	}

	return nil
}

//region Config util

// getP2P gets p2p's config from config
func (c *Config) getP2PConfig() (*configs.P2PConfig, error) {
	var privKey *ecdsa.PrivateKey
	var err error
	peer := c.P2P
	if peer.PrivateKey != "" {
		privKey, err = crypto.HexToECDSA(peer.PrivateKey)
	} else {
		privKey, err = crypto.GenerateKey()
	}
	if err != nil {
		return nil, err
	}
	p2pConfig := configs.DefaultP2PConfig()
	p2pConfig.Seeds = c.MainChain.Seeds
	p2pConfig.ListenAddress = c.P2P.ListenAddress
	p2pConfig.RootDir = c.DataDir
	p2pConfig.AddrBook = filepath.Join(c.DataDir, "addrbook.json")
	p2pConfig.PrivateKey = privKey
	return p2pConfig, nil
}

// getDbInfo gets database information from config. Currently, it only supports levelDb
func (c *Config) getDbInfo(isDual bool) storage.DbInfo {
	database := c.MainChain.Database
	if isDual {
		database = c.DualChain.Database
	}
	nodeDir := filepath.Join(c.DataDir, c.Name, database.Dir)
	if database.Drop == 1 {
		// Clear all contents within data dir
		if err := removeDirContents(nodeDir); err != nil {
			panic(err)
		}
	}
	return storage.NewLevelDbInfo(nodeDir, database.Caches, database.Handles)
}

// getTxPoolConfig gets txPoolConfig from config, based on target network
func (c *Config) getTxPoolConfig() tx_pool.TxPoolConfig {
	txPool := c.Genesis.TxPool
	if args.network == Mainnet {
		return tx_pool.DefaultTxPoolConfig
	}
	return tx_pool.TxPoolConfig{
		AccountSlots:  txPool.AccountSlots,
		AccountQueue:  txPool.AccountQueue,
		GlobalSlots:   txPool.GlobalSlots,
		GlobalQueue:   txPool.GlobalQueue,
		MaxBatchBytes: txPool.MaxBatchBytes,
		Broadcast:     txPool.Broadcast,
	}
}

// getGenesisConfig gets node data from config
func (c *Config) getGenesisConfig(isDual bool) (*genesis.Genesis, error) {
	var (
		ga  genesis.GenesisAlloc
		err error
	)
	g := c.MainChain.Genesis
	if isDual {
		g = c.DualChain.Genesis
	}

	if g == nil {
		ga = make(genesis.GenesisAlloc, 0)
	} else {
		var genesisAccounts = make(map[string]*big.Int)
		genesisContracts := make(map[string]string)

		amount, _ := big.NewInt(0).SetString(g.GenesisAmount, 10)
		for _, address := range g.Addresses {
			genesisAccounts[address] = amount
		}

		for key, contract := range g.Contracts {
			configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
			if key != "Staking" {
				genesisContracts[contract.Address] = contract.ByteCode
			}
		}
		ga, err = genesis.GenesisAllocFromAccountAndContract(genesisAccounts, genesisContracts)
		if err != nil {
			return nil, err
		}
	}
	var validators []*genesis.Validator
	for _, v := range g.Validators {
		validators = append(validators, &genesis.Validator{
			Address: v.Address,
			Power:   v.Power,
			Name:    v.Name,
		})
	}

	return &genesis.Genesis{
		Config:          c.getChainConfig(),
		Alloc:           ga,
		Validators:      validators,
		ConsensusParams: c.getConsensusParams(),
		Consensus:       c.getConsensusConfig(),
	}, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config) getMainChainConfig() (*node.MainChainConfig, error) {
	chain := c.MainChain
	dbInfo := c.getDbInfo(false)
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesisConfig(false)
	if err != nil {
		return nil, err
	}
	mainChainConfig := node.MainChainConfig{
		DBInfo:      dbInfo,
		Genesis:     genesisData,
		TxPool:      c.getTxPoolConfig(),
		AcceptTxs:   chain.AcceptTxs,
		IsZeroFee:   chain.ZeroFee == 1,
		NetworkId:   chain.NetworkID,
		ChainId:     chain.ChainID,
		ServiceName: chain.ServiceName,
		Consensus:   genesisData.Consensus,
	}
	return &mainChainConfig, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config) getDualChainConfig() (*node.DualChainConfig, error) {
	dbInfo := c.getDbInfo(true)
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesisConfig(true)
	if err != nil {
		return nil, err
	}
	var eventPool = event_pool.Config{
		GlobalSlots: c.DualChain.EventPool.GlobalSlots,
		GlobalQueue: c.DualChain.EventPool.GlobalQueue,
		BlockSize:   c.DualChain.EventPool.BlockSize,
	}

	dualChainConfig := node.DualChainConfig{
		DBInfo:           dbInfo,
		DualGenesis:      genesisData,
		DualEventPool:    eventPool,
		DualNetworkID:    c.DualChain.NetworkID,
		ChainId:          c.DualChain.ChainID,
		DualProtocolName: *c.DualChain.Protocol,
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
	nodeConfig := node.Config{
		Name:             n.Name,
		DataDir:          n.DataDir,
		P2P:              p2pConfig,
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

// getConsensusConfig gets consensus timeout configs
func (c *Config) getConsensusConfig() *typesCfg.ConsensusConfig {
	if args.network == Mainnet {
		return configs.DefaultConsensusConfig()
	}
	return &typesCfg.ConsensusConfig{
		TimeoutPropose:              time.Duration(c.Genesis.Consensus.TimeoutPropose) * time.Millisecond,
		TimeoutProposeDelta:         time.Duration(c.Genesis.Consensus.TimeoutProposeDelta) * time.Millisecond,
		TimeoutPrevote:              time.Duration(c.Genesis.Consensus.TimeoutPrevote) * time.Millisecond,
		TimeoutPrevoteDelta:         time.Duration(c.Genesis.Consensus.TimeoutPrevoteDelta) * time.Millisecond,
		TimeoutPrecommit:            time.Duration(c.Genesis.Consensus.TimeoutPrecommit) * time.Millisecond,
		TimeoutPrecommitDelta:       time.Duration(c.Genesis.Consensus.TimeoutPrecommitDelta) * time.Millisecond,
		TimeoutCommit:               time.Duration(c.Genesis.Consensus.TimeoutCommit) * time.Millisecond,
		IsSkipTimeoutCommit:         c.Genesis.Consensus.IsSkipTimeoutCommit,
		IsCreateEmptyBlocks:         c.Genesis.Consensus.IsCreateEmptyBlocks,
		CreateEmptyBlocksInterval:   time.Duration(c.Genesis.Consensus.CreateEmptyBlocksInterval) * time.Millisecond,
		PeerGossipSleepDuration:     time.Duration(c.Genesis.Consensus.PeerGossipSleepDuration) * time.Millisecond,
		PeerQueryMaj23SleepDuration: time.Duration(c.Genesis.Consensus.PeerQueryMaj23SleepDuration) * time.Millisecond,
	}
}

// getConsensusConfig gets consensus config params
func (c *Config) getConsensusParams() *kaiproto.ConsensusParams {
	defaultCsParams := _default.ConsensusParams()
	if args.network == Mainnet {
		return defaultCsParams
	}
	return &kaiproto.ConsensusParams{
		Block: kaiproto.BlockParams{
			MaxBytes:   c.Genesis.ConsensusParams.Block.MaxBytes,
			MaxGas:     c.Genesis.ConsensusParams.Block.MaxGas,
			TimeIotaMs: defaultCsParams.Block.TimeIotaMs,
		},
		Evidence: kaiproto.EvidenceParams{
			MaxAgeNumBlocks: c.Genesis.ConsensusParams.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  time.Duration(c.Genesis.ConsensusParams.Evidence.MaxAgeDuration) * time.Hour,
			MaxBytes:        c.Genesis.ConsensusParams.Evidence.MaxBytes,
		},
	}
}

func (c *Config) getChainConfig() *typesCfg.ChainConfig {

	if args.network == Mainnet {
		return configs.MainnetChainConfig
	}
	return c.Genesis.ChainConfig
}

// Start starts chain with given config
func (c *Config) Start() {
	logger := c.newLog()

	logger.AddTag("Loader")
	// System settings
	logger.Info("Adjust runtime settings...")
	if err := runtimeSystemSettings(); err != nil {
		logger.Error("Fail to update system settings", "err", err)
		return
	}

	// get nodeConfig from config
	logger.Info("Read node config...")
	nodeConfig, err := c.getNodeConfig()
	if err != nil {
		logger.Error("Cannot get node config", "err", err)
		return
	}

	genesisCfg, err := c.getGenesisConfig(false)
	if err != nil {
		panic(err)
	}

	nodeConfig.Genesis = genesisCfg
	// init new node from nodeConfig
	logger.Info("Building node from config...")
	n, err := node.New(nodeConfig)
	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	logger.Info("Register new kai service...")
	if err := n.Register(kai.NewKardiaService); err != nil {
		logger.Error("error while adding kardia service", "err", err)
		return
	}

	logger.Info("Register dual node service...")
	if c.DualChain != nil {
		if err := n.Register(service.NewDualService); err != nil {
			logger.Error("error while adding dual service", "err", err)
			return
		}
	}

	logger.Info("Start running node...")
	if err := n.Start(); err != nil {
		logger.Error("error while starting node", "err", err)
		return
	}

	if c.MainChain.Events != nil {
		logger.Info("Register mainchain event...")
		var kardiaService *kai.KardiaService
		if err := n.Service(&kardiaService); err != nil {
			logger.Error("cannot get Kardia service", "err", err)
			return
		}
		// save watchers to db
		c.SaveWatchers(kardiaService, c.MainChain.Events)
	}

	if c.DualChain != nil {
		logger.Info("Start dual node...")
		if err := c.StartDual(n); err != nil {
			logger.Error("error while starting dual", "err", err)
			return
		}
	}

	if c.Debug != nil {
		logger.Info("Start debugger...")
		if err := c.StartDebug(); err != nil {
			logger.Error("Failed to start debug", "err", err)
		}
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

func (c *Config) StartDebug() error {
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
	return nil
}

func (c *Config) SaveWatchers(service node.Service, events []typesCfg.Event) {
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
			smc := &types.KardiaSmartcontract{
				MasterSmc:  event.MasterSmartContract,
				MasterAbi:  masterAbi,
				SmcAddress: event.ContractAddress,
				SmcAbi:     abi,
			}
			service.DB().WriteEvent(smc)
		}
	}
}

//endregion config util
