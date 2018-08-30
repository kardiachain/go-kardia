package node

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"github.com/kardiachain/go-kardia/rpc"
)

// Node is the highest level container for a full Kardia node.
// It keeps all config data and services.
type Node struct {
	config       *NodeConfig
	serverConfig p2p.Config
	server       *p2p.Server

	services            map[string]Service // Map of type names to running services
	serviceConstructors []ServiceConstructor

	rpcAPIs       []rpc.API    // List of APIs currently provided by the node
	httpEndpoint  string       // HTTP endpoint (interface + port) to listen at (empty = HTTP disabled)
	httpWhitelist []string     // HTTP RPC modules to allow through this endpoint
	httpListener  net.Listener // HTTP RPC listener socket to server API requests
	httpHandler   *rpc.Server  // HTTP RPC request handler to process the API requests

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

	// RPC Endpoint
	n.httpEndpoint = n.config.HTTPEndpoint()

	// Generate node PrivKey
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
	newServices := make(map[string]Service)
	for _, serviceConstructor := range n.serviceConstructors {
		// Creates context as parameter for constructor
		ctx := &ServiceContext{
			Config:   n.config,
			Services: make(map[string]Service),
		}
		for serviceType, s := range newServices { // full map copy in each ServiceContext, for concurrent access
			ctx.Services[serviceType] = s
		}
		service, err := serviceConstructor(ctx)
		if err != nil {
			return err
		}
		serviceTypeName := reflect.TypeOf(service).Elem().Name()
		if _, exists := newServices[serviceTypeName]; exists {
			return fmt.Errorf("duplicated service of type %s", serviceTypeName)
		}
		newServices[serviceTypeName] = service
	}
	// Gather the protocols and start the freshly assembled P2P server
	for _, service := range newServices {
		newServer.Protocols = append(newServer.Protocols, service.Protocols()...)
	}

	if err := newServer.Start(); err != nil {
		return err
	}

	// Start each of the services
	var startedServices []Service
	for _, service := range newServices {
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
	if err := n.startRPC(newServices); err != nil {
		for _, service := range newServices {
			service.Stop()
		}
		newServer.Stop()
		return err
	}

	// Finish init startup
	n.services = newServices
	n.server = newServer
	return nil
}

func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server == nil {
		return ErrNodeStopped
	}

	sFailures := make(map[string]error)

	for typeName, service := range n.services {
		if err := service.Stop(); err != nil {
			sFailures[typeName] = err
		}
	}

	n.server.Stop()
	n.services = nil
	n.server = nil

	if len(sFailures) > 0 {
		n.log.Error("Failed to stop node services: %v", sFailures)
		return ErrNodeStopFailure
	}

	return nil
}

// startRPC: start all the various RPC endpoint during node startup ONLY.
// DO NOT CALL AFTERWARDS
func (n *Node) startRPC(services map[string]Service) error {
	apis := n.apis()
	n.log.Info("StartRPC")
	n.log.Debug("Add Services APIs to node")
	for name, service := range services {
		n.log.Debug(fmt.Sprintf("Add APIs from services: %s, len: %v", name, len(service.APIs())))
		apis = append(apis, service.APIs()...)
	}

	if err := n.startHTTP(n.httpEndpoint, apis, n.config.HTTPModules, n.config.HTTPCors, n.config.HTTPVirtualHosts); err != nil {
		return err
	}

	n.rpcAPIs = apis
	return nil
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (n *Node) startHTTP(endpoint string, apis []rpc.API, modules []string, cors []string, vhosts []string) error {
	if endpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartHTTPEndpoint(endpoint, apis, modules, cors, vhosts)
	if err != nil {
		return err
	}
	n.log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))

	n.httpEndpoint = endpoint
	n.httpListener = listener
	n.httpHandler = handler

	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (n *Node) stopHTTP() {
	if n.httpListener != nil {
		n.httpListener.Close()
		n.httpListener = nil

		n.log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", n.httpEndpoint))
	}
	if n.httpHandler != nil {
		n.httpHandler.Stop()
		n.httpHandler = nil
	}
}

// Server returns p2p server of node.
func (n *Node) Server() *p2p.Server {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.server
}

// Service adds a new service to node.
func (n *Node) RegisterService(constructor ServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return ErrNodeRunning
	}
	n.serviceConstructors = append(n.serviceConstructors, constructor)
	return nil
}

// AddPeer adds a remote node as static peer, maintaining the new
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

// Service returns running service with given type.
// returnedService should be **Service pointer.
func (n *Node) Service(returnedService interface{}) error {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if n.server == nil {
		return ErrNodeStopped
	}

	// Get pointer with *Service type.
	pointer := reflect.ValueOf(returnedService).Elem()
	serviceName := pointer.Type().Elem().Name()

	if registeredS, ok := n.services[serviceName]; ok {
		pointer.Set(reflect.ValueOf(registeredS))

		return nil
	}
	return ErrServiceUnknown
}

// ServiceMap returns map of all running services.
func (n *Node) ServiceMap() map[string]Service {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.services
}

// All endpoints of the node.
// TODO: Add more APIs
func (n *Node) apis() []rpc.API {
	return []rpc.API{
		{
			Namespace: "node",
			Version:   "1.0",
			Service:   NewPublicNodeAPI(n),
			Public:    true,
		},
	}
}
