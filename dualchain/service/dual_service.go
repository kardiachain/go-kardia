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
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/dualchain/blockchain"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/node"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/kardiachain/go-kardiamain/types/evidence"
)

const DualServiceName = "DUAL"

// TODO: evaluates using this subservice as dual mode or light subprotocol.
// DualService implements Service for running full dual group protocol, for group consensus.
type DualService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Dual service

	config      *DualConfig
	chainConfig *typesCfg.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	groupDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	eventPool           *event_pool.Pool
	blockchain          *blockchain.DualBlockChain
	csManager           *consensus.ConsensusManager
	dualBlockOperations *blockchain.DualBlockOperations

	networkID uint64
}

// New creates a new DualService object (including the
// initialisation of the common DualService object)
func newDualService(ctx *node.ServiceContext, config *DualConfig) (*DualService, error) {
	var err error
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(DualServiceName)
	logger.Info("newDualService", "chaintype", config.DBInfo.Name())

	groupDb := ctx.BlockStore

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, groupDb, config.DualGenesis, nil)
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

	// Create a new blockchain to attach to this GroupService struct
	dualService.blockchain, err = blockchain.NewBlockChain(logger, groupDb, dualService.chainConfig, config.IsPrivate)
	if err != nil {
		return nil, err
	}

	dualService.eventPool = event_pool.NewPool(logger, config.DualEventPool, dualService.blockchain)

	lastBlockState, err := ctx.StateDB.LoadStateFromDBOrGenesisDoc(config.DualGenesis)
	if err != nil {
		return nil, err
	}

	evPool, err := evidence.NewPool(ctx.StateDB, groupDb.DB(), dualService.blockchain)
	if err != nil {
		return nil, err
	}
	//evReactor := evidence.NewReactor(evPool)

	dualService.dualBlockOperations = blockchain.NewDualBlockOperations(dualService.logger, dualService.blockchain, dualService.eventPool, evPool)
	blockExec := cstate.NewBlockExecutor(ctx.StateDB, evPool, dualService.dualBlockOperations)

	consensusState := consensus.NewConsensusState(
		dualService.logger,
		config.Consensus,
		lastBlockState,
		dualService.dualBlockOperations,
		blockExec,
		evPool,
	)
	dualService.csManager = consensus.NewConsensusManager(consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewDefaultPrivValidator(ctx.Config.NodeKey())
	dualService.csManager.SetPrivValidator(privValidator)

	//namdoh@ dualService.protocolManager.acceptTxs = config.AcceptTxs
	return dualService, nil
}

// NewDualService Implements ServiceConstructor, return a dual service from node service context.
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
		Consensus:     chainConfig.Consensus,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *DualService) SetDualBlockChainManager(bcManager *blockchain.DualBlockChainManager) {
	s.dualBlockOperations.SetDualBlockChainManager(bcManager)
}

func (s *DualService) IsListening() bool  { return true } // Always listening
func (s *DualService) NetVersion() uint64 { return s.networkID }
func (s *DualService) DB() types.StoreDB  { return s.groupDb }

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *DualService) Start(srvr *p2p.Switch) error {
	return nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *DualService) Stop() error {
	s.csManager.Stop()

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
