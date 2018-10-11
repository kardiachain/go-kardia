package kai

import (
	"github.com/kardiachain/go-kardia/blockchain/dual"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/storage"
	"github.com/kardiachain/go-kardia/types"
)

const DualServiceName = "DUAL"
const DualNetworkID = 100 // TODO: change this to be diff than main kardia service or the same
const dualProtocolName = "dualptc"

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
	groupDb storage.Database // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	protocolManager *ProtocolManager
	blockchain      *dual.DualBlockChain
	csManager       *consensus.ConsensusManager

	networkID uint64
}

// New creates a new DualService object (including the
// initialisation of the common DualService object)
func newDualService(ctx *node.ServiceContext, config *DualConfig) (*DualService, error) {
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(DualServiceName)
	logger.Info("newDualService", "chaindata", config.ChainData)

	groupDb, err := ctx.Config.StartDatabase(config.ChainData, config.DbCaches, config.DbHandles)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := dual.SetupGenesisBlock(logger, groupDb, config.DualGenesis)
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
	logger.Info("Initialising protocol", "versions", kcmn.ProtocolVersions, "network", config.NetworkId)

	// Create a new blockchain to attach to this GroupService struct
	dualService.blockchain, err = dual.NewBlockChain(logger, groupDb, dualService.chainConfig)
	if err != nil {
		return nil, err
	}

	// Initialization for consensus.
	block := dualService.blockchain.CurrentBlock()
	validatorSet := ctx.Config.DevEnvConfig.GetValidatorSet(ctx.Config.DualChainConfig.NumValidators)
	state := state.LastestBlockState{
		ChainID:                     "kaigroupcon",
		LastBlockHeight:             cmn.NewBigUint64(block.Height()),
		LastBlockID:                 block.BlockID(),
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt32(-1),
	}
	consensusState := consensus.NewConsensusState(
		dualService.logger,
		configs.DefaultConsensusConfig(),
		state,
		consensus.NewDualBlockOperations(dualService.logger, dualService.blockchain),
		ctx.Config.DevEnvConfig.VotingStrategy,
	)
	dualService.csManager = consensus.NewConsensusManager(DualServiceName, consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	dualService.csManager.SetPrivValidator(privValidator)

	if dualService.protocolManager, err = NewProtocolManager(dualProtocolName, dualService.logger, config.NetworkId, dualService.blockchain, dualService.chainConfig, nil, dualService.csManager); err != nil {
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
		NetworkId:   DualNetworkID,
		ChainData:   chainConfig.ChainData,
		DbHandles:   chainConfig.DbHandles,
		DbCaches:    chainConfig.DbCache,
		DualGenesis: chainConfig.DualGenesis,
	})

	if err != nil {
		return nil, err
	}

	return kai, nil
}

func (s *DualService) IsListening() bool       { return true } // Always listening
func (s *DualService) DualServiceVersion() int { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *DualService) NetVersion() uint64      { return s.networkID }

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

/* TODO: API for this service
func (s *DualService) APIs() []rpc.API {
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
*/
func (s *DualService) APIs() []rpc.API { return []rpc.API{} }

func (s *DualService) BlockChain() *dual.DualBlockChain      { return s.blockchain }
func (s *DualService) DualChainConfig() *configs.ChainConfig { return s.chainConfig }
