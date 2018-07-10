// Package kai implements the Kardia protocol.
package kai

import (
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/node"
)

const DefaultNetworkID = 100

type KaiServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// Kardia implements the Kardia full node service.
type Kardia struct {
	config *Config

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Ethereum

	// Handlers
	protocolManager *ProtocolManager
	kaiServer       KaiServer

	networkID uint64
}

func (s *Kardia) AddKaiServer(ks KaiServer) {
	s.kaiServer = ks
}

// New creates a new Kardia object (including the
// initialisation of the common Kardia object)
func newKardia(config *Config) (*Kardia, error) {

	kai := &Kardia{
		config: config,

		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
	}

	log.Info("Initialising Kardia protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	var err error
	if kai.protocolManager, err = NewProtocolManager(config.NetworkId); err != nil {
		return nil, err
	}

	return kai, nil
}

// Implements node.ServiceConstructor, return a Kardia node service from node service context.
func NewKardiaService(ctx *node.ServiceContext) (node.Service, error) {
	kai, err := newKardia(&Config{NetworkId: DefaultNetworkID})
	if err != nil {
		return nil, err
	}
	return kai, nil
}

func (s *Kardia) IsListening() bool  { return true } // Always listening
func (s *Kardia) KaiVersion() int    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Kardia) NetVersion() uint64 { return s.networkID }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Kardia) Protocols() []p2p.Protocol {
	if s.kaiServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.kaiServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Kardia protocol implementation.
func (s *Kardia) Start(srvr *p2p.Server) error {
	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers

	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.kaiServer != nil {
		s.kaiServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Kardia protocol.
func (s *Kardia) Stop() error {
	s.protocolManager.Stop()
	if s.kaiServer != nil {
		s.kaiServer.Stop()
	}

	close(s.shutdownChan)

	return nil
}
