// Package kai implements the Kardia protocol.
package kai

import (
	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/configs"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	kaidb "github.com/kardiachain/go-kardia/storage"
)

const DefaultNetworkID = 100

// TODO: evaluates using this subservice as dual mode or light subprotocol.
type KardiaSubService interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// Kardia implements Service for running full Kardia full protocol.
type Kardia struct {
	config      *Config
	chainConfig *configs.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Ethereum

	// DB interfaces
	chainDb kaidb.Database // Block chain database

	// Handlers
	txPool          *blockchain.TxPool
	protocolManager *ProtocolManager
	blockchain      *blockchain.BlockChain

	subService KardiaSubService

	networkID uint64
}

func (s *Kardia) AddKaiServer(ks KardiaSubService) {
	s.subService = ks
}

// New creates a new Kardia object (including the
// initialisation of the common Kardia object)
func newKardia(ctx *ServiceContext, config *Config) (*Kardia, error) {
	// TODO(thientn): Uses config for database parameters
	chainDb, err := ctx.Config.StartDatabase("chaindata", 16, 16)
	if err != nil {
		return nil, err
	}

	chainConfig, _, genesisErr := blockchain.SetupGenesisBlock(chainDb, config.Genesis)
	if genesisErr != nil {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	kai := &Kardia{
		config:       config,
		chainDb:      chainDb,
		chainConfig:  chainConfig,
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}

	log.Info("Initialising Kardia protocol", "versions", kcmn.ProtocolVersions, "network", config.NetworkId)

	// TODO(huny@): Do we need to check for blockchain version mismatch ?

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(chainDb, kai.chainConfig)
	if err != nil {
		return nil, err
	}

	kai.txPool = blockchain.NewTxPool(config.TxPool, kai.chainConfig, kai.blockchain)

	if kai.protocolManager, err = NewProtocolManager(config.NetworkId, kai.blockchain, kai.chainConfig, kai.txPool); err != nil {
		return nil, err
	}

	return kai, nil
}

// Implements ServiceConstructor, return a Kardia node service from node service context.
// TODO: move this outside of kai package to customize kai.Config
func NewKardiaService(ctx *ServiceContext) (Service, error) {
	kai, err := newKardia(ctx, &Config{NetworkId: DefaultNetworkID})
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

	// Starts optional subservice.
	if s.subService != nil {
		s.subService.Start(srvr)
	}
	return nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *Kardia) Stop() error {
	s.protocolManager.Stop()
	if s.subService != nil {
		s.subService.Stop()
	}

	close(s.shutdownChan)

	return nil
}

func (s *Kardia) ConnectReactor(reactor Reactor) {
	s.protocolManager.ConnectReactor(reactor)
	reactor.SetProtocolManager(s.protocolManager)
}

func (s *Kardia) TxPool() *blockchain.TxPool         { return s.txPool }
func (s *Kardia) BlockChain() *blockchain.BlockChain { return s.blockchain }
func (s *Kardia) ChainConfig() *configs.ChainConfig  { return s.chainConfig }
