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
	bcReactor "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence"
)

const DualServiceName = "DUAL"

// TODO: evaluates using this subservice as dual mode or light subprotocol.
// DualService implements Service for running full dual group protocol, for group consensus.
type DualService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Dual service

	config      *DualConfig
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	groupDb types.StoreDB // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	eventPool           *event_pool.Pool
	blockchain          *blockchain.DualBlockChain
	csManager           *consensus.ConsensusManager
	dualBlockOperations *blockchain.DualBlockOperations
	bcR                 p2p.Reactor // for fast-syncing

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

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(groupDb.DB(), config.DualGenesis)
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
	dualService.blockchain, err = blockchain.NewBlockChain(logger, groupDb, dualService.chainConfig)
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
	blockExec := cstate.NewBlockExecutor(ctx.StateDB, logger, evPool, dualService.dualBlockOperations)

	// state starting configs
	// Set private validator for consensus manager.
	privValidator := types.NewDefaultPrivValidator(ctx.Config.NodeKey())
	// Determine whether we should do fast sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	config.FastSync.Enable = config.FastSync.Enable && !onlyValidatorIsUs(lastBlockState, privValidator.GetAddress())
	// Make BlockchainReactor. Don't start fast sync if we're doing a state sync first.
	bcR := bcReactor.NewBlockchainReactor(lastBlockState, blockExec, dualService.dualBlockOperations, config.FastSync)
	dualService.bcR = bcR

	consensusState := consensus.NewConsensusState(
		dualService.logger,
		config.Consensus,
		lastBlockState,
		dualService.dualBlockOperations,
		blockExec,
		evPool,
	)
	dualService.csManager = consensus.NewConsensusManager(consensusState, config.FastSync)
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
		Consensus:     chainConfig.Consensus,
		FastSync:      chainConfig.FastSync,
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
func onlyValidatorIsUs(state cstate.LatestBlockState, privValAddress common.Address) bool {
	if state.Validators.Size() > 1 {
		return false
	}
	addr, _ := state.Validators.GetByIndex(0)
	return privValAddress.Equal(addr)
}

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
