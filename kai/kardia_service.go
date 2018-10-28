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
	"encoding/hex"
	"errors"

	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/dual"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	"github.com/kardiachain/go-kardia/kai/dev"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/storage"
	"github.com/kardiachain/go-kardia/types"
)

const (
	KardiaServiceName = "KARDIA"
	DefaultNetworkID  = 100
	kaiProtocolName   = "kaiptc"
)

var (
	ErrFailedGetState = errors.New("Fail to get Kardia state")
	ErrCreateKardiaTx = errors.New("Fail to create Kardia's Tx from DualEvent")
	ErrAddKardiaTx    = errors.New("Fail to add Tx to Kardia's TxPool")
)

// TODO: evaluates using this subservice as dual mode or light subprotocol.
type KardiaSubService interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// Kardia implements Service for running full Kardia full protocol.
type Kardia struct {
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
	protocolManager *ProtocolManager
	blockchain      *blockchain.BlockChain
	csManager       *consensus.ConsensusManager

	subService KardiaSubService

	networkID uint64
}

func (n *Kardia) SubmitTx(event *types.EventData) error {
	kardiaStateDB, err := n.blockchain.State()
	if err != nil {
		log.Error("Fail to get Kardia state", "error", err)
		return ErrFailedGetState
	}

	// TODO(thientn,namdoh): Remove hard-coded genesisAccount here.
	addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[dev.MockKardiaAccountForMatchEthTx])
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := dual.CreateKardiaMatchAmountTx(addrKey, kardiaStateDB, event.Data.TxValue, 1)
	if tx == nil {
		log.Error("Fail to create Kardia's tx from DualEvent")
		return ErrCreateKardiaTx
	}

	if err := n.txPool.AddLocal(tx); err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return ErrAddKardiaTx
	}
	log.Info("Add Kardia's tx successfully", "txHash", tx.Hash().Hex())

	return nil
}

func (s *Kardia) AddKaiServer(ks KardiaSubService) {
	s.subService = ks
}

// New creates a new Kardia object (including the
// initialisation of the common Kardia object)
func newKardia(ctx *node.ServiceContext, config *Config) (*Kardia, error) {
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag(KardiaServiceName)
	logger.Info("newKardia", "chaindata", config.ChainData)

	kaiDb, err := ctx.Config.StartDatabase(config.ChainData, config.DbCaches, config.DbHandles)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := blockchain.SetupGenesisBlock(logger, kaiDb, config.Genesis)
	if genesisErr != nil {
		return nil, genesisErr
	}
	logger.Info("Initialised Kardia chain configuration", "config", chainConfig)

	kai := &Kardia{
		logger:       logger,
		config:       config,
		kaiDb:        kaiDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}
	logger.Info("Initialising protocol", "versions", kcmn.ProtocolVersions, "network", config.NetworkId)

	// TODO(huny@): Do we need to check for blockchain version mismatch ?

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(logger, kaiDb, kai.chainConfig)
	if err != nil {
		return nil, err
	}

	kai.txPool = blockchain.NewTxPool(logger, config.TxPool, kai.chainConfig, kai.blockchain)

	// Initialization for consensus.
	block := kai.blockchain.CurrentBlock()
	log.Info("KARDIA Validators: ", "valIndex", ctx.Config.MainChainConfig.ValidatorIndices)
	validatorSet := ctx.Config.DevEnvConfig.GetValidatorSetByIndex(ctx.Config.MainChainConfig.ValidatorIndices)
	state := state.LastestBlockState{
		ChainID:                     "kaicon",
		LastBlockHeight:             cmn.NewBigUint64(block.Height()),
		LastBlockID:                 block.BlockID(),
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt32(-1),
	}
	consensusState := consensus.NewConsensusState(
		kai.logger,
		configs.DefaultConsensusConfig(),
		state,
		consensus.NewBlockOperations(kai.logger, kai.blockchain, kai.txPool),
		ctx.Config.DevEnvConfig.VotingStrategy,
	)
	kai.csManager = consensus.NewConsensusManager(KardiaServiceName, consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	kai.csManager.SetPrivValidator(privValidator)

	// Initialize protocol manager.

	if kai.protocolManager, err = NewProtocolManager(kaiProtocolName, kai.logger, config.NetworkId, kai.blockchain, kai.chainConfig, kai.txPool, kai.csManager); err != nil {
		return nil, err
	}
	kai.protocolManager.acceptTxs = config.AcceptTxs
	kai.csManager.SetProtocol(kai.protocolManager)

	return kai, nil
}

// Implements ServiceConstructor, return a Kardia node service from node service context.
// TODO: move this outside of kai package to customize kai.Config
func NewKardiaService(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.MainChainConfig
	kai, err := newKardia(ctx, &Config{
		NetworkId: DefaultNetworkID,
		ChainData: chainConfig.ChainData,
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

func (s *Kardia) IsListening() bool  { return true } // Always listening
func (s *Kardia) KaiVersion() int    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Kardia) NetVersion() uint64 { return s.networkID }

// Protocols implements Service, returning all the currently configured
// network protocols to start.
func (s *Kardia) Protocols() []p2p.Protocol {
	if s.subService == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.subService.Protocols()...)
}

// Start implements Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *Kardia) Start(srvr *p2p.Server) error {
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
func (s *Kardia) Stop() error {
	s.csManager.Stop()
	s.protocolManager.Stop()
	if s.subService != nil {
		s.subService.Stop()
	}

	close(s.shutdownChan)

	return nil
}

func (s *Kardia) APIs() []rpc.API {
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

func (s *Kardia) TxPool() *blockchain.TxPool         { return s.txPool }
func (s *Kardia) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *Kardia) ChainConfig() *configs.ChainConfig  { return s.chainConfig }
