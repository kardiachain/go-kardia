package kai

import (
	"github.com/kardiachain/go-kardia/blockchain"
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

const DualNetworkID = 200 // TODO: change this to be diff than main kardia service or the same

// TODO: evaluates using this subservice as dual mode or light subprotocol.

// DualService implements Service for running full dual group protocol, for group consensus.
type DualService struct {
	// TODO(namdoh): Refactor out logger to a based Service type.
	logger log.Logger // Logger for Dual service

	config      *Config
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool

	// DB interfaces
	groupDb storage.Database // Local key-value store endpoint. Each use types should use wrapper layer with unique prefixes.

	// Handlers
	txPool          *blockchain.TxPool
	protocolManager *ProtocolManager
	blockchain      *blockchain.BlockChain
	csManager       *consensus.ConsensusManager

	networkID uint64
}

// New creates a new DualService object (including the
// initialisation of the common DualService object)
func newDualService(ctx *node.ServiceContext, config *Config) (*DualService, error) {
	log.Info("newDualService", "chaindata", config.ChainData)

	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag("DUAL")

	groupDb, err := ctx.Config.StartDatabase(config.ChainData, config.DbCaches, config.DbHandles)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := blockchain.SetupGenesisBlock(logger, groupDb, config.Genesis)
	if genesisErr != nil {
		return nil, genesisErr
	}
	logger.Info("Initialised dual chain configuration", "config", chainConfig)

	dualS := &DualService{
		logger:       logger,
		config:       config,
		groupDb:      groupDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}
	logger.Info("Initialising protocol", "versions", kcmn.ProtocolVersions, "network", config.NetworkId)

	// Create a new blockchain to attach to this GroupService struct
	dualS.blockchain, err = blockchain.NewBlockChain(logger, groupDb, dualS.chainConfig)
	if err != nil {
		return nil, err
	}

	dualS.txPool = blockchain.NewTxPool(logger, config.TxPool, dualS.chainConfig, dualS.blockchain)

	// Initialization for consensus.
	block := dualS.blockchain.CurrentBlock()
	validatorSet := ctx.Config.DevEnvConfig.GetValidatorSet(ctx.Config.DualChainConfig.NumValidators)
	state := state.LastestBlockState{
		ChainID:                     "kaigroupcon",
		LastBlockHeight:             cmn.NewBigInt(int64(block.Height())),
		LastBlockID:                 block.BlockID(),
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt(-1),
	}
	consensusState := consensus.NewConsensusState(
		dualS.logger,
		configs.DefaultConsensusConfig(),
		state,
		dualS.blockchain,
		dualS.txPool,
		ctx.Config.DevEnvConfig.VotingStrategy,
	)
	dualS.csManager = consensus.NewConsensusManager(consensusState)
	// Set private validator for consensus manager.
	privValidator := types.NewPrivValidator(ctx.Config.NodeKey())
	dualS.csManager.SetPrivValidator(privValidator)

	// Initialize protocol manager.
	if dualS.protocolManager, err = NewProtocolManager(dualS.logger, config.NetworkId, dualS.blockchain, dualS.chainConfig, dualS.txPool, dualS.csManager); err != nil {
		return nil, err
	}
	dualS.protocolManager.acceptTxs = config.AcceptTxs
	dualS.csManager.SetProtocol(dualS.protocolManager)

	return dualS, nil
}

// Implements ServiceConstructor, return a dual service from node service context.
func NewDualService(ctx *node.ServiceContext) (node.Service, error) {
	chainConfig := ctx.Config.DualChainConfig
	kai, err := newDualService(ctx, &Config{
		NetworkId: DualNetworkID,
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

func (s *DualService) TxPool() *blockchain.TxPool         { return s.txPool }
func (s *DualService) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *DualService) ChainConfig() *configs.ChainConfig  { return s.chainConfig }
