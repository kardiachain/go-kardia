/*
 *  Copyright 2019 KardiaChain
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
	"errors"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"math/big"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"
	"github.com/kardiachain/go-kardia/lib/sysutils"
	kai "github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	kaiproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"

	// Force-load the tracer engines to trigger registration
	_ "github.com/kardiachain/go-kardia/mainchain/tracers/js"
	_ "github.com/kardiachain/go-kardia/mainchain/tracers/native"
)

var args flags

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
	p2pConfig.PrivateKey = privKey
	p2pConfig.Seeds = c.MainChain.Seeds
	p2pConfig.ListenAddress = c.P2P.ListenAddress
	p2pConfig.MaxNumInboundPeers = c.P2P.InboundPeers
	p2pConfig.MaxNumOutboundPeers = c.P2P.OutboundPeers
	p2pConfig.RootDir = c.DataDir
	p2pConfig.AddrBook = filepath.Join(c.DataDir, "addrbook.json")
	return p2pConfig, nil
}

// getDbInfo gets database information from config. Currently, it only supports levelDb
func (c *Config) getDbInfo() storage.DbInfo {
	database := c.MainChain.Database
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
		AccountSlots: txPool.AccountSlots,
		AccountQueue: txPool.AccountQueue,
		GlobalSlots:  txPool.GlobalSlots,
		GlobalQueue:  txPool.GlobalQueue,
		Broadcast:    txPool.Broadcast,
	}
}

// getGenesisConfig gets node data from config
func (c *Config) getGenesisConfig() (*genesis.Genesis, error) {
	var (
		ga  genesis.GenesisAlloc
		err error
	)
	g := c.MainChain.Genesis

	if g == nil {
		ga = make(genesis.GenesisAlloc, 0)
	} else {
		genesisAccounts := make(map[string]*big.Int)
		genesisContracts := make(map[string]string)

		for _, account := range g.Accounts {
			amount, ok := new(big.Int).SetString(account.Amount, 10)
			if !ok {
				return nil, errors.New("cannot convert genesis amount")
			}
			genesisAccounts[account.Address] = amount
		}

		for key, contract := range g.Contracts {
			configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
			if key != configs.StakingContractKey && contract.Address != "" {
				genesisContracts[contract.Address] = contract.ByteCode
			}
		}
		ga, err = genesis.GenesisAllocFromAccountAndContract(genesisAccounts, genesisContracts)
		if err != nil {
			return nil, err
		}
	}

	return &genesis.Genesis{
		Config:          c.getChainConfig(),
		InitialHeight:   1,
		Alloc:           ga,
		Validators:      g.Validators,
		ConsensusParams: c.getConsensusParams(),
		Consensus:       c.getConsensusConfig(),
		Timestamp:       time.Unix(g.Timestamp, 0),
	}, nil
}

// getMainChainConfig gets mainchain's config from config
func (c *Config) getMainChainConfig() (*node.MainChainConfig, error) {
	chain := c.MainChain
	dbInfo := c.getDbInfo()
	if dbInfo == nil {
		return nil, fmt.Errorf("cannot get dbInfo")
	}
	genesisData, err := c.getGenesisConfig()
	if err != nil {
		return nil, err
	}
	mainChainConfig := node.MainChainConfig{
		DBInfo:      dbInfo,
		Genesis:     genesisData,
		TxPool:      c.getTxPoolConfig(),
		AcceptTxs:   chain.AcceptTxs,
		ServiceName: chain.ServiceName,
		Consensus:   genesisData.Consensus,
		FastSync:    c.getFastSyncConfig(),
		GasOracle:   c.getGasOracleConfig(),
	}
	if args.network == Mainnet {
		mainChainConfig.ChainId = configs.MainnetChainID
		mainChainConfig.NetworkId = configs.MainnetNetworkID
	} else {
		mainChainConfig.ChainId = configs.TestnetChainID
		mainChainConfig.NetworkId = configs.TestnetNetworkID
	}
	return &mainChainConfig, nil
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
		Version:          configs.Version,
		DataDir:          n.DataDir,
		P2P:              p2pConfig,
		HTTPHost:         n.HTTPHost,
		HTTPPort:         n.HTTPPort,
		HTTPCors:         n.HTTPCors,
		HTTPVirtualHosts: n.HTTPVirtualHosts,
		HTTPModules:      n.HTTPModules,
		WSHost:           n.WSHost,
		WSPort:           n.WSPort,
		WSOrigins:        n.WSOrigins,
		MainChainConfig:  node.MainChainConfig{},
		Metrics:          n.Metrics,
		FastSync:         c.getFastSyncConfig(),
		GasOracle:        c.getGasOracleConfig(),
	}
	mainChainConfig, err := c.getMainChainConfig()
	if err != nil {
		return nil, err
	}
	if mainChainConfig == nil {
		return nil, fmt.Errorf("mainChainConfig is empty")
	}
	nodeConfig.MainChainConfig = *mainChainConfig
	if c.TimeOutForStaticCall > 0 {
		configs.TimeOutForStaticCall = c.TimeOutForStaticCall
	} else {
		configs.TimeOutForStaticCall = configs.DefaultTimeOutForStaticCall
	}
	return &nodeConfig, nil
}

func (c *Config) getFastSyncConfig() *configs.FastSyncConfig {
	if c.FastSync == nil {
		return configs.DefaultFastSyncConfig()
	}
	return &configs.FastSyncConfig{
		ServiceName:   c.FastSync.ServiceName,
		Enable:        c.FastSync.Enable,
		MaxPeers:      c.FastSync.MaxPeers,
		TargetPending: c.FastSync.TargetPending,
		PeerTimeout:   time.Duration(c.FastSync.PeerTimeout) * time.Second,
		MinRecvRate:   c.FastSync.MinRecvRate,
	}
}

func (c *Config) getGasOracleConfig() *oracles.Config {
	if c.GasOracle == nil {
		return oracles.DefaultOracleConfig()
	}
	defaultGasPrice, ok := new(big.Int).SetString(c.GasOracle.Default, 10)
	if !ok {
		defaultGasPrice = oracles.DefaultOracleConfig().Default
	}
	maxGasPrice, ok := new(big.Int).SetString(c.GasOracle.MaxPrice, 10)
	if !ok {
		maxGasPrice = oracles.DefaultOracleConfig().MaxPrice
	}
	return &oracles.Config{
		Blocks:     c.GasOracle.Blocks,
		Percentile: c.GasOracle.Percentile,
		Default:    defaultGasPrice,
		MaxPrice:   maxGasPrice,
	}
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
func (c *Config) getConsensusConfig() *configs.ConsensusConfig {
	if args.network == Mainnet {
		consensusConfig := configs.DefaultConsensusConfig()
		consensusConfig.WalPath = filepath.Join(c.DataDir, "cs.wal", "wal")
		return consensusConfig
	}
	return &configs.ConsensusConfig{
		WalPath:                     filepath.Join(c.DataDir, "cs.wal", "wal"),
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
	defaultCsParams := configs.DefaultConsensusParams()
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

func (c *Config) getChainConfig() *configs.ChainConfig {
	if args.network == Mainnet {
		return configs.MainnetChainConfig
	}
	return c.Genesis.ChainConfig
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

	genesisCfg, err := c.getGenesisConfig()
	if err != nil {
		panic(err)
	}

	nodeConfig.Genesis = genesisCfg
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

	if err := n.Start(); err != nil {
		logger.Error("error while starting node", "err", err)
		return
	}

	if c.Metrics {
		logger.Warn("Collect metrics enabled")
		metrics.Enabled = true
	}

	if c.Debug != nil {
		if err := c.StartDebug(); err != nil {
			logger.Error("Failed to start debug", "err", err)
		}
	}

	waitForever()
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
		router.Handle("/debug/metrics/cstate", prometheus.Handler(metrics.CStateRegistry))
		router.Handle("/metrics", promhttp.Handler())

		if err := http.ListenAndServe(c.Debug.Port, cors.AllowAll().Handler(router)); err != nil {
			panic(err)
		}
	}()
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

func waitForever() {
	select {}
}

func main() {
	flag.Parse()
	config, err := LoadConfig(args)
	if err != nil {
		panic(err)
	}
	config.Start()
}
