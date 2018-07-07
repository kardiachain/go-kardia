package node

import (
	"errors"
	"fmt"
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"reflect"
	"sync"
)

var (
	ErrNodeStopped    = errors.New("node not started")
	ErrNodeRunning    = errors.New("node already running")
	ErrServiceUnknown = errors.New("service unknown")
)

// TODO: move to a blockstore.
type Block struct {
	Index        int
	Hash         string
	PreviousHash string
	Content      string
}

// Wrapper for a running node.
type Node struct {
	blockchain []Block

	config       *NodeConfig
	serverConfig p2p.Config
	server       *p2p.Server

	services            map[reflect.Type]Service // Running services
	serviceConstructors []ServiceConstructor

	lock sync.RWMutex
	log  log.Logger
}

func NewNode(config *NodeConfig) (*Node, error) {
	node := new(Node)
	node.config = config
	node.log = log.New()
	// TODO: input check on the config

	return node, nil
}

func (n *Node) Start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Starts p2p server.
	if n.server != nil {
		return ErrNodeRunning
	}
	n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)

	n.serverConfig = n.config.P2P
	n.serverConfig.Logger = n.log
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.PrivateKey = n.config.NodeKey()

	// TODO: use json file in datadir to load the node list
	// n.serverConfig.StaticNodes = []
	// n.serverConfig.TrustedNodes = ...
	// n.serverConfig.NodeDatabase = ...

	newServer := &p2p.Server{Config: n.serverConfig}

	// Starts protocol services.
	services := make(map[reflect.Type]Service)
	for _, serviceConstructor := range n.serviceConstructors {
		// Creates context as parameter for constructor
		ctx := &ServiceContext{
			config:         n.config,
			services:       make(map[reflect.Type]Service),
		}
		for serviceType, s := range services { // full map copy in each ServiceContext, for concurrent access
			ctx.services[serviceType] = s
		}
		service, err := serviceConstructor(ctx)
		if err != nil {
			return err
		}
		serviceType := reflect.TypeOf(service)
		if _, exists := services[serviceType]; exists {
			return fmt.Errorf("duplicated service of type %s", serviceType)
		}
		services[serviceType] = service
	}
	// Gather the protocols and start the freshly assembled P2P server
	for _, service := range services {
		newServer.Protocols = append(newServer.Protocols, service.Protocols()...)
	}

	if err := newServer.Start(); err != nil {
		return err
	}

	// Start each of the services
	var startedServices []Service
	for _, service := range services {
		// Start the next service, stopping all previous upon failure
		if err := service.Start(newServer); err != nil {
			for _, startedService := range startedServices {
				startedService.Stop()
			}
			newServer.Stop()

			return err
		}
		// Mark the service started for potential cleanup
		startedServices = append(startedServices, service)
	}

	// TODO: starts RPC services.

	n.server = newServer
	return nil
}

func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server == nil {
		return ErrNodeStopped
	}

	n.server.Stop()
	n.server = nil

	return nil
}

// Gets p2p server of node.
func (n *Node) Server() *p2p.Server {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.server
}

func (n *Node) RegisterService(constructor ServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return ErrNodeRunning
	}
	n.serviceConstructors = append(n.serviceConstructors, constructor)
	return nil
}

// Add a remote node as static peer, maintaining the new
// connection at all times, even reconnecting if it is lost.
// Only accepts complete node for now.
func (n *Node) AddPeer(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := n.Server()

	if server == nil {
		return false, ErrNodeStopped
	}
	// Try to add the url as a static peer and return
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	if node.Incomplete() {
		return false, errors.New("peer node is incomplete")
	}

	server.AddPeer(node)

	return true, nil
}
