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

package service

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/service"
	serviceconst "github.com/kardiachain/go-kardia/kai/service/const"
	"github.com/kardiachain/go-kardia/kai/state"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

const DualServiceName = "DUAL"

// TODO: evaluates using this subservice as dual mode or light subprotocol.
// DualService implements Service for running full dual group protocol, for group consensus.
type DualService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Dual service

	config      *DualConfig
	chainConfig *types.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	groupDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	eventPool           *event_pool.Pool
	protocolManager     *service.ProtocolManager
	blockchain          *blockchain.DualBlockChain
	csManager           *consensus.ConsensusManager
	dualBlockOperations *blockchain.DualBlockOperations

	networkID uint64
}

// New creates a new DualService object (including the
// initialisation of the common DualService object)
func newDualService(ctx *node.ServiceContext, config *DualConfig) (*DualService, error) {
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(DualServiceName)
	logger.Info("newDualService", "chaintype", config.DBInfo.Name())

	groupDb, err := ctx.Config.StartDatabase(config.DBInfo)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, groupDb, config.DualGenesis, config.BaseAccount)
	if genesisErr != nil {
		return nil, genesisErr
	}
	logger.Info("Initialised dual chain configuration", "config", chainConfig)

	dualService := &DualService{
		logger:       logger,
		config:       config,
		groupDb:      groupDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}
	logger.Info("Initialising protocol", "versions", serviceconst.ProtocolVersions, "network", config.NetworkId)

	// Create a new blockchain to attach to this GroupService struct
	dualService.blockchain, err = blockchain.NewBlockChain(logger, groupDb, dualService.chainConfig, config.IsPrivate)
	if err != nil {
		return nil, err
	}

	dualService.eventPool = event_pool.NewPool(logger, config.DualEventPool, dualService.blockchain)

	// Initialization for consensus.
	block := dualService.blockchain.CurrentBlock()
	log.Info("DUAL Validators: ", "valIndex", ctx.Config.DualChainConfig.ValidatorIndexes)
	var validatorSet *types.ValidatorSet
	validatorSet, err = node.GetValidatorSet(dualService.blockchain, ctx.Config.DualChainConfig.ValidatorIndexes)
	if err != nil {
		logger.Error("Cannot get validator from indices", "indices", ctx.Config.DualChainConfig.ValidatorIndexes, "err", err)
		return nil, err
	}
	state := state.LastestBlockState{
		ChainID:                     "kaigroupcon",
		LastBlockHeight:             cmn.NewBigUint64(block.Height()),
		LastBlockID:                 block.Header().LastBlockID,
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt32(-1),
	}
	dualService.dualBlockOperations = blockchain.NewDualBlockOperations(dualService.logger, dualService.blockchain, dualService.eventPool)
	consensusState := consensus.NewConsensusState(
		dualService.logger,
		configs.DefaultConsensusConfig(),
		state,
		dualService.dualBlockOperations,
	)
	dualService.csManager = consensus.NewConsensusManager(DualServiceName, consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	dualService.csManager.SetPrivValidator(privValidator)

	if dualService.protocolManager, err = service.NewProtocolManager(
		config.ProtocolName,
		dualService.logger,
		config.NetworkId,
		config.ChainID,
		dualService.blockchain,
		dualService.chainConfig,
		nil,
		dualService.csManager); err != nil {
		return nil, err
	}
	//namdoh@ dualService.protocolManager.acceptTxs = config.AcceptTxs
	dualService.csManager.SetProtocol(dualService.protocolManager)
	return dualService, nil
}

// Implements ServiceConstructor, return a dual service from node service context.
func NewDualService(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.DualChainConfig
	kai, err := newDualService(ctx, &DualConfig{
		ProtocolName:  chainConfig.DualProtocolName,
		NetworkId:     chainConfig.DualNetworkID,
		ChainID:       chainConfig.ChainId,
		DBInfo:        chainConfig.DBInfo,
		DualEventPool: chainConfig.DualEventPool,
		DualGenesis:   chainConfig.DualGenesis,
		IsPrivate:     chainConfig.IsPrivate,
		BaseAccount:   chainConfig.BaseAccount,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *DualService) SetDualBlockChainManager(bcManager *blockchain.DualBlockChainManager) {
	s.dualBlockOperations.SetDualBlockChainManager(bcManager)
}

func (s *DualService) IsListening() bool       { return true } // Always listening
func (s *DualService) DualServiceVersion() int { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *DualService) NetVersion() uint64      { return s.networkID }
func (s *DualService) DB() types.StoreDB       { return s.groupDb }

// Protocols implements Service, returning all the currently configured
// network protocols to start.
func (s *DualService) Protocols() []p2p.Protocol {
	return s.protocolManager.SubProtocols
}

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *DualService) Start(srvr *p2p.Server) error {
	// Figures out a max peers count based on the server limits.
	maxPeers := srvr.MaxPeers

	// Starts the networking layer.
	s.protocolManager.Start(maxPeers)

	// Start consensus manager.
	s.csManager.Start()

	return nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *DualService) Stop() error {
	s.csManager.Stop()
	s.protocolManager.Stop()

	close(s.shutdownChan)

	return nil
}

func (s *DualService) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "dual",
			Version:   "1.0",
			Service:   NewPublicDualAPI(s),
			Public:    true,
		},
	}
}

func (s *DualService) EventPool() *event_pool.Pool            { return s.eventPool }
func (s *DualService) BlockChain() *blockchain.DualBlockChain { return s.blockchain }
func (s *DualService) DualChainConfig() *types.ChainConfig    { return s.chainConfig }
