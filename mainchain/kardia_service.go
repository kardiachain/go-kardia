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
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/kai/service"
	serviceconst "github.com/kardiachain/go-kardia/kai/service/const"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/storage"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

const (
	KardiaServiceName = "KARDIA"
	DefaultNetworkID  = 100
	kaiProtocolName   = "kaiptc"
	MainChainID       = 1
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
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	kaiDb storage.Database // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	txPool          *blockchain.TxPool
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
	logger.AddTag(KardiaServiceName)
	logger.Info("newKardiaService", "chaindata", config.ChainData)

	kaiDb, err := ctx.Config.StartDatabase(config.ChainData, config.DbCaches, config.DbHandles)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := blockchain.SetupGenesisBlock(logger, kaiDb, config.Genesis)
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
	kai.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, kai.chainConfig)
	if err != nil {
		return nil, err
	}

	kai.txPool = blockchain.NewTxPool(logger, config.TxPool, kai.chainConfig, kai.blockchain)

	// Initialization for consensus.
	block := kai.blockchain.CurrentBlock()
	log.Info("KARDIA Validators: ", "valIndex", ctx.Config.MainChainConfig.ValidatorIndexes)
	var validatorSet *types.ValidatorSet
	if ctx.Config.DevEnvConfig != nil {
		validatorSet = ctx.Config.DevEnvConfig.GetValidatorSetByIndex(ctx.Config.MainChainConfig.ValidatorIndexes)
	} else {
		validatorSet = &types.ValidatorSet{}
	}

	state := state.LastestBlockState{
		ChainID:                     "kaicon", // TODO(thientn): considers merging this with protocolmanger.ChainID
		LastBlockHeight:             cmn.NewBigUint64(block.Height()),
		LastBlockID:                 block.BlockID(),
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt32(-1),
	}
<<<<<<< HEAD
	var votingStrategy map[dev.VoteTurn]int
	if ctx.Config.DevEnvConfig != nil {
		votingStrategy = ctx.Config.DevEnvConfig.VotingStrategy
	}

	consensusState := consensus.NewConsensusState(
		kai.logger,
		configs.DefaultConsensusConfig(),
		state,
		blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool),
		votingStrategy,
	)
=======
	var consensusState *consensus.ConsensusState
	if ctx.Config.DevEnvConfig != nil {
		consensusState = consensus.NewConsensusState(
			kai.logger,
			configs.DefaultConsensusConfig(),
			state,
			blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool),
			ctx.Config.DevEnvConfig.VotingStrategy,
		)
	} else {
		consensusState = consensus.NewConsensusState(
			kai.logger,
			configs.DefaultConsensusConfig(),
			state,
			blockchain.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool),
			nil,
		)
	}
>>>>>>> 39ec7e77a2c68d4c66a5dde6a98197a1e6939536
	kai.csManager = consensus.NewConsensusManager(KardiaServiceName, consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	kai.csManager.SetPrivValidator(privValidator)

	// Initialize protocol manager.

	if kai.protocolManager, err = service.NewProtocolManager(
		kaiProtocolName,
		kai.logger,
		config.NetworkId,
		MainChainID,
		kai.blockchain,
		kai.chainConfig,
		kai.txPool,
		kai.csManager); err != nil {
		return nil, err
	}
	kai.protocolManager.SetAcceptTxs(config.AcceptTxs)
	kai.csManager.SetProtocol(kai.protocolManager)

	return kai, nil
}

// Implements ServiceConstructor, return a Kardia node service from node service context.
// TODO: move this outside of kai package to customize kai.Config
func NewKardiaService(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.MainChainConfig
	kai, err := newKardiaService(ctx, &Config{
		NetworkId: DefaultNetworkID,
		ChainData: chainConfig.ChainDataDir,
		DbHandles: chainConfig.DbHandles,
		DbCaches:  chainConfig.DbCache,
		Genesis:   chainConfig.Genesis,
		TxPool:    chainConfig.TxPool,
		AcceptTxs: chainConfig.AcceptTxs,
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

func (s *KardiaService) TxPool() *blockchain.TxPool         { return s.txPool }
func (s *KardiaService) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *KardiaService) ChainConfig() *configs.ChainConfig  { return s.chainConfig }
