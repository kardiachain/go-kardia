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
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/staking"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/node"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/kardiachain/go-kardiamain/types/evidence"
)

const (
	kaiProtocolName = "KAI"
)

// TODO: evaluates using this subservice as dual mode or light subprotocol.
type KardiaSubService interface {
	Start(srvr *p2p.Switch)
	Stop()
}

// KardiaService implements Service for running full Kardia protocol.
type KardiaService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Kardia service

	config      *Config
	chainConfig *typesCfg.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	kaiDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	txPool     *tx_pool.TxPool
	blockchain *blockchain.BlockChain
	csManager  *consensus.ConsensusManager
	txpoolR    *tx_pool.Reactor
	evR        *evidence.Reactor

	subService KardiaSubService

	networkID uint64

	eventBus *types.EventBus
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

	logger.Info("Setup staking utils...")
	stakingUtil, err := staking.NewSmcStakingnUtil()
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
	}

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, kai.chainConfig, config.IsPrivate)
	if err != nil {
		return nil, err
	}

	evPool, err := evidence.NewPool(ctx.StateDB, kaiDb.DB(), kai.blockchain)
	if err != nil {
		return nil, err
	}
	kai.txPool = tx_pool.NewTxPool(config.TxPool, kai.chainConfig, kai.blockchain)
	kai.txpoolR = tx_pool.NewReactor(config.TxPool, kai.txPool)
	kai.txpoolR.SetLogger(kai.logger)

	logger.Info("Create new block operations...")
	bOper := blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool, evPool, stakingUtil)

	kai.evR = evidence.NewReactor(evPool)
	kai.evR.SetLogger(kai.logger)
	logger.Info("Setup block executor...")
	blockExec := cstate.NewBlockExecutor(ctx.StateDB, evPool, bOper)

	logger.Info("Load state...")
	state, err := ctx.StateDB.LoadStateFromDBOrGenesisDoc(config.Genesis)
	if err != nil {
		return nil, err
	}

	consensusState := consensus.NewConsensusState(
		kai.logger,
		config.Consensus,
		state,
		bOper,
		blockExec,
		evPool,
	)
	kai.csManager = consensus.NewConsensusManager(consensusState)
	// Set private validator for consensus manager.
	logger.Info("Start setup private validator")
	privValidator := types.NewDefaultPrivValidator(ctx.Config.NodeKey())
	kai.csManager.SetPrivValidator(privValidator)
	kai.csManager.SetEventBus(kai.eventBus)
	return kai, nil
}

// NewKardiaService Implements ServiceConstructor, return a Kardia node service from node service context.
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
		IsPrivate:   chainConfig.IsPrivate,
		Consensus:   chainConfig.Consensus,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *KardiaService) IsListening() bool  { return true } // Always listening
func (s *KardiaService) NetVersion() uint64 { return s.networkID }

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *KardiaService) Start(srvr *p2p.Switch) error {
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
	}
}

func (s *KardiaService) TxPool() *tx_pool.TxPool            { return s.txPool }
func (s *KardiaService) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *KardiaService) DB() types.StoreDB                  { return s.kaiDb }
