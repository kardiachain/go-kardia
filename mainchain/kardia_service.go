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

// Package kai implements the Kardia protocol.
package kai

import (
	bcReactor "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/filters"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tracers"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence"
)

// TODO: evaluates using this sub-service as dual mode or light sub-protocol.
type KardiaSubService interface {
	Start(srvr *p2p.Switch)
	Stop()
}

// KardiaService implements Service for running full Kardia protocol.
type KardiaService struct {
	logger log.Logger // Logger for Kardia service

	config      *Config
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	kaiDb   types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.
	stateDB cstate.Store

	// Handlers
	txPool     *tx_pool.TxPool
	blockchain *blockchain.BlockChain
	csManager  *consensus.ConsensusManager
	txpoolR    *tx_pool.Reactor
	evR        *evidence.Reactor
	bcR        p2p.Reactor // for fast-syncing

	subService KardiaSubService

	networkID uint64

	eventBus *types.EventBus

	staking   *staking.StakingSmcUtil
	validator *staking.ValidatorSmcUtil

	bloomRequests     chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer      *BloomIndexer                  // Bloom indexer operating during block imports
	closeBloomHandler chan struct{}

	gpo *oracles.Oracle
}

func (s *KardiaService) AddKaiServer(ks KardiaSubService) {
	s.subService = ks
}

// New creates a new KardiaService object (including the
// initialisation of the common KardiaService object)
func newKardiaService(ctx *node.ServiceContext, config *Config) (*KardiaService, error) {
	var err error
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag(config.ServiceName)
	logger.Info("newKardiaService", "dbType", config.DBInfo.Name())

	kaiDb := ctx.BlockStore

	stakingUtil, err := staking.NewSmcStakingUtil()
	if err != nil {
		return nil, err
	}
	validator, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, kaiDb, config.Genesis, stakingUtil)
	if genesisErr != nil {
		return nil, genesisErr
	}
	logger.Info("Initialised Kardia chain configuration", "config", chainConfig)

	// EventBus and IndexerService must be started before the handshake because
	// we might need to index the txs of the replayed block as this might not have happened
	// when the node stopped last time (i.e. the node stopped after it saved the block
	// but before it indexed the txs, or, endblocker panicked)
	eventBus, err := createAndStartEventBus(logger)
	if err != nil {
		return nil, err
	}

	kai := &KardiaService{
		logger:       logger,
		config:       config,
		kaiDb:        kaiDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
		eventBus:     eventBus,
		staking:      stakingUtil,
		validator:    validator,
		bloomIndexer: NewBloomIndexer(kaiDb.DB(), configs.BloomBitsBlocksClient, configs.HelperTrieConfirmations),
	}

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, kai.chainConfig)
	if err != nil {
		return nil, err
	}

	kai.stateDB = ctx.StateDB
	evPool, err := evidence.NewPool(ctx.StateDB, kaiDb.DB(), kai.blockchain)
	if err != nil {
		return nil, err
	}
	kai.txPool = tx_pool.NewTxPool(config.TxPool, kai.chainConfig, kai.blockchain)
	kai.txpoolR = tx_pool.NewReactor(config.TxPool, kai.txPool)
	kai.txpoolR.SetLogger(kai.logger)

	bOper := blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool, evPool, stakingUtil)

	kai.evR = evidence.NewReactor(evPool)
	kai.evR.SetLogger(kai.logger)
	blockExec := cstate.NewBlockExecutor(ctx.StateDB, logger, evPool, bOper)

	state, err := ctx.StateDB.LoadStateFromDBOrGenesisDoc(config.Genesis)
	if err != nil {
		return nil, err
	}

	// state starting configs
	// Set private validator for consensus manager.
	privValidator := types.NewDefaultPrivValidator(ctx.Config.NodeKey())
	// Determine whether we should do fast sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	config.FastSync.Enable = config.FastSync.Enable && !onlyValidatorIsUs(state, privValidator.GetAddress())
	// Make BlockchainReactor. Don't start fast sync if we're doing a state sync first.
	bcR := bcReactor.NewBlockchainReactor(state, blockExec, bOper, config.FastSync)
	kai.bcR = bcR
	consensusState := consensus.NewConsensusState(
		kai.logger,
		config.Consensus,
		state,
		bOper,
		blockExec,
		evPool,
	)
	kai.csManager = consensus.NewConsensusManager(consensusState, config.FastSync)
	// Set private validator for consensus manager.
	kai.csManager.SetPrivValidator(privValidator)
	kai.csManager.SetEventBus(kai.eventBus)

	// init gas price oracle
	kai.gpo = oracles.NewGasPriceOracle(kai, config.GasOracle)
	return kai, nil
}

// NewKardiaService Implements ServiceConstructor, return a Kardia node service from node service context.
// TODO: move this outside of kai package to customize kai.Config
func NewKardiaService(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.MainChainConfig
	kai, err := newKardiaService(ctx, &Config{
		NetworkId:   chainConfig.NetworkId,
		ServiceName: chainConfig.ServiceName,
		ChainId:     chainConfig.ChainId,
		DBInfo:      chainConfig.DBInfo,
		Genesis:     chainConfig.Genesis,
		TxPool:      chainConfig.TxPool,
		AcceptTxs:   chainConfig.AcceptTxs,
		Consensus:   chainConfig.Consensus,
		FastSync:    chainConfig.FastSync,
		GasOracle:   chainConfig.GasOracle,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *KardiaService) IsListening() bool  { return true } // Always listening
func (s *KardiaService) NetVersion() uint64 { return s.networkID }
func onlyValidatorIsUs(state cstate.LatestBlockState, privValAddress common.Address) bool {
	if state.Validators.Size() > 1 {
		return false
	}
	addr, _ := state.Validators.GetByIndex(0)
	return privValAddress.Equal(addr)
}

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *KardiaService) Start(srvr *p2p.Switch) error {
	srvr.AddReactor("BLOCKCHAIN", s.bcR)
	srvr.AddReactor("CONSENSUS", s.csManager)
	srvr.AddReactor("TXPOOL", s.txpoolR)
	srvr.AddReactor("EVIDENCE", s.evR)
	return nil
}

func createAndStartEventBus(logger log.Logger) (*types.EventBus, error) {
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.New("module", "events"))
	if err := eventBus.Start(); err != nil {
		return nil, err
	}
	return eventBus, nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *KardiaService) Stop() error {
	if s.subService != nil {
		s.subService.Stop()
	}
	close(s.shutdownChan)
	return nil
}

func (s *KardiaService) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "kai",
			Version:   "1.0",
			Service:   NewPublicKaiAPI(s),
			Public:    true,
		},
		{
			Namespace: "kai",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s, true),
			Public:    true,
		},
		{
			Namespace: "tx",
			Version:   "1.0",
			Service:   NewPublicTransactionAPI(s),
			Public:    true,
		},
		{
			Namespace: "account",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(s),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   tracers.NewTracerAPI(s),
			Public:    true,
		},
		// Web3 endpoints support
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicWeb3API(s),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(s),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s, false),
			Public:    true,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   NewPublicNetAPI(s.networkID),
			Public:    true,
		},
	}
}

func (s *KardiaService) TxPool() *tx_pool.TxPool            { return s.txPool }
func (s *KardiaService) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *KardiaService) DB() types.StoreDB                  { return s.kaiDb }
func (s *KardiaService) Config() *configs.ChainConfig       { return s.blockchain.Config() }
