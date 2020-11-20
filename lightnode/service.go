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
package lightnode

import (
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/kai/api"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/blockchain"
	"github.com/kardiachain/go-kardiamain/kai/genesis"
	"github.com/kardiachain/go-kardiamain/kai/staking"
	"github.com/kardiachain/go-kardiamain/kai/tx_pool"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/node"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/kardiachain/go-kardiamain/types/evidence"
)

// NodeService implement Service
type NodeService interface {
	base.Service
	api.LightNodeAPI
}

// nodeService implements KardiaService for running full Kardia protocol.
type nodeService struct {
	logger log.Logger // Logger for Kardia service

	// General config
	// todo: should we move chainConfig to config also
	config      *Config
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	kaiDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Internal component should using interface instead
	txPool     tx_pool.TxPool
	blockchain blockchain.Blockchain

	csManager *consensus.ConsensusManager
	txpoolR   *tx_pool.Reactor
	evR       *evidence.Reactor

	networkID uint64

	eventBus *types.EventBus
}

// New creates a new KardiaService object (including the
// initialisation of the common KardiaService object)
func buildService(ctx *node.ServiceContext, config *Config) (node.Service, error) {
	// Create new logger and assign with light node
	logger := log.New()
	logger.AddTag(config.ServiceName)
	logger.Info("newKardiaService", "dbType", config.DBInfo.Name())

	// Create db component
	kaiDb := ctx.BlockStore
	stakingUtils, err := staking.NewSmcStakingUtil()
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, kaiDb, config.Genesis, stakingUtils)
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

	nodeService := &nodeService{
		logger:       logger,
		config:       config,
		kaiDb:        kaiDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
		eventBus:     eventBus,
	}

	// Create a new blockchain to attach to this Kardia object
	nodeService.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, nodeService.chainConfig, config.IsPrivate)
	if err != nil {
		return nil, err
	}

	evPool, err := evidence.NewPool(ctx.StateDB, kaiDb.DB(), nodeService.blockchain)
	if err != nil {
		return nil, err
	}
	nodeService.txPool = tx_pool.NewTxPool(config.TxPool, nodeService.blockchain)
	nodeService.txpoolR = tx_pool.NewReactor(config.TxPool, nodeService.txPool)
	nodeService.txpoolR.SetLogger(nodeService.logger)

	//bOper := blockchain.NewBlockOperations(nodeService.logger, nodeService.blockchain, nodeService.txPool, evPool, stakingUtils)

	nodeService.evR = evidence.NewReactor(evPool)
	nodeService.evR.SetLogger(nodeService.logger)

	// Look like we do not need this one
	// or atleast full
	//blockExec := cstate.NewBlockExecutor(ctx.StateDB, evPool, bOper)
	//// Clone or create new stateDB
	//stateDB, err := ctx.StateDB.LoadStateFromDBOrGenesisDoc(config.Genesis)
	//if err != nil {
	//	return nil, err
	//}

	//consensusState := consensus.NewConsensusState(
	//	nodeService.logger,
	//	config.Consensus,
	//	stateDB,
	//	bOper,
	//	blockExec,
	//	evPool,
	//)
	//nodeService.csManager = consensus.NewConsensusManager(consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewDefaultPrivValidator(ctx.Config.NodeKey())
	nodeService.csManager.SetPrivValidator(privValidator)
	nodeService.csManager.SetEventBus(nodeService.eventBus)
	return nodeService, nil
}

// NewKardiaService Implements ServiceConstructor, return a Kardia node service from node service context.
func NewLightNode(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.MainChainConfig
	kai, err := buildService(ctx, &Config{
		NetworkId:   chainConfig.NetworkId,
		ServiceName: chainConfig.ServiceName,
		ChainId:     chainConfig.ChainId,
		DBInfo:      chainConfig.DBInfo,
		// ignore now, lets see if we should create new type or not
		//Genesis:     chainConfig.Genesis,
		//TxPool:      chainConfig.TxPool,
		AcceptTxs: chainConfig.AcceptTxs,
		IsPrivate: chainConfig.IsPrivate,
		Consensus: chainConfig.Consensus,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *nodeService) IsListening() bool  { return true } // Always listening
func (s *nodeService) NetVersion() uint64 { return s.networkID }

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *nodeService) Start(srvr *p2p.Switch) error {
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
func (s *nodeService) Stop() error {
	close(s.shutdownChan)
	return nil
}

func (s *nodeService) APIs() []rpc.API {
	return []rpc.API{
		//{
		//	Namespace: "kai",
		//	Version:   "1.0",
		//	Service:   NewPublicKaiAPI(s),
		//	Public:    true,
		//},
		//{
		//	Namespace: "tx",
		//	Version:   "1.0",
		//	Service:   NewPublicTransactionAPI(s),
		//	Public:    true,
		//},
		//{
		//	Namespace: "account",
		//	Version:   "1.0",
		//	Service:   NewPublicAccountAPI(s),
		//	Public:    true,
		//},
	}
}

func (s *nodeService) TxPool() tx_pool.TxPool            { return s.txPool }
func (s *nodeService) BlockChain() blockchain.Blockchain { return s.blockchain }
func (s *nodeService) DB() types.StoreDB                 { return s.kaiDb }