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
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/kai/service"
	serviceconst "github.com/kardiachain/go-kardiamain/kai/service/const"
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
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// KardiaService implements Service for running full Kardia protocol.
type KardiaService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Kardia service

	config      *Config
	chainConfig *types.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	kaiDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	txPool          *tx_pool.TxPool
	protocolManager *service.ProtocolManager
	blockchain      *blockchain.BlockChain
	csManager       *consensus.ConsensusManager

	subService KardiaSubService

	networkID uint64
}

func (s *KardiaService) AddKaiServer(ks KardiaSubService) {
	s.subService = ks
}

// New creates a new KardiaService object (including the
// initialisation of the common KardiaService object)
func newKardiaService(ctx *node.ServiceContext, config *Config) (*KardiaService, error) {
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag(config.ServiceName)
	logger.Info("newKardiaService", "dbType", config.DBInfo.Name())

	kaiDb, err := ctx.StartDatabase(config.DBInfo)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, kaiDb, config.Genesis, config.BaseAccount)
	if genesisErr != nil {
		return nil, genesisErr
	}
	logger.Info("Initialised Kardia chain configuration", "config", chainConfig)

	kai := &KardiaService{
		logger:       logger,
		config:       config,
		kaiDb:        kaiDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}
	logger.Info("Initialising protocol", "versions", serviceconst.ProtocolVersions, "network", config.NetworkId)

	// TODO(huny@): Do we need to check for blockchain version mismatch ?

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, kai.chainConfig, config.IsPrivate)
	if err != nil {
		return nil, err
	}

	staking, err := staking.NewSmcStakingnUtil(kai.blockchain)
	if err != nil {
		return nil, err
	}
	evPool := evidence.NewPool(kaiDb.DB(), kaiDb.DB())
	// Set zeroFee to blockchain
	kai.blockchain.IsZeroFee = config.IsZeroFee
	kai.txPool = tx_pool.NewTxPool(config.TxPool, kai.chainConfig, kai.blockchain)

	bOper := blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool, evPool, staking)

	evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(kai.blockchain.DB().DB(), evPool, bOper)

	state, err := cstate.LoadStateFromDBOrGenesisDoc(kaiDb.DB(), config.Genesis)
	if err != nil {
		return nil, err
	}

	logger.Info("Validators: ", "vals", state.Validators.Validators)

	consensusState := consensus.NewConsensusState(
		kai.logger,
		configs.DefaultConsensusConfig(),
		state,
		bOper,
		blockExec,
		evPool,
	)
	kai.csManager = consensus.NewConsensusManager(config.ServiceName, consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	kai.csManager.SetPrivValidator(privValidator)

	// Initialize protocol manager.

	if kai.protocolManager, err = service.NewProtocolManager(
		kaiProtocolName,
		kai.logger,
		config.NetworkId,
		config.ChainId,
		kai.blockchain,
		kai.chainConfig,
		kai.txPool,
		kai.csManager,
		evReactor); err != nil {
		return nil, err
	}
	kai.protocolManager.SetAcceptTxs(config.AcceptTxs)
	kai.csManager.SetProtocol(kai.protocolManager)
	evReactor.SetProtocol(kai.protocolManager)

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
		IsZeroFee:   chainConfig.IsZeroFee,
		IsPrivate:   chainConfig.IsPrivate,
		BaseAccount: chainConfig.BaseAccount,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *KardiaService) IsListening() bool  { return true } // Always listening
func (s *KardiaService) KaiVersion() int    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *KardiaService) NetVersion() uint64 { return s.networkID }

// Protocols implements Service, returning all the currently configured
// network protocols to start.
func (s *KardiaService) Protocols() []p2p.Protocol {
	if s.subService == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.subService.Protocols()...)
}

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *KardiaService) Start(srvr *p2p.Server) error {
	// Figures out a max peers count based on the server limits.
	maxPeers := srvr.MaxPeers

	// Starts the networking layer.
	s.protocolManager.Start(maxPeers)

	// Start consensus manager.
	s.csManager.Start()

	// Starts optional subservice.
	if s.subService != nil {
		s.subService.Start(srvr)
	}
	return nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *KardiaService) Stop() error {
	s.csManager.Stop()
	s.protocolManager.Stop()
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
func (s *KardiaService) ChainConfig() *types.ChainConfig    { return s.chainConfig }
func (s *KardiaService) DB() types.StoreDB                  { return s.kaiDb }
