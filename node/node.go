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

package node

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/p2p/discover"
	"github.com/kardiachain/go-kardia/rpc"
)

var PeerProxyURL = "0.0.0.0:9001"

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

	// Finish init startup
	n.services = newServices
	n.server = newServer
	return nil
}

func (n *Node) StartServiceRPC() error {
	// TODO: starts RPC services.
	services := n.services
	if err := n.startRPC(services); err != nil {
		for _, service := range services {
			service.Stop()
		}
		n.server.Stop()
		return err
	}
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
func (n *Node) ConfirmAddPeer(node *discover.Node) error {
	server := n.Server()
	if err := server.AddPeer(node); err != nil {
		return err
	}
	return nil
}

type Request struct {
	Method     string
	ReqNode    *proxyNode
	TargetNode *proxyNode
}

type proxyNode struct {
	ID  string
	IP  string
	TCP uint16
	RPC uint16
}

func (n *Node) AddPeer(url string) (bool, error) {
	//Create Node from URL
	server := n.Server()
	reqNode := server.Self()

	if server == nil {
		return false, ErrNodeStopped
	}
	targetNode, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	if targetNode.Incomplete() {
		return false, errors.New("peer node is incomplete")
	}

	if err := n.CallProxy("AddPeer", reqNode, targetNode); err != nil {
		return false, err
	}
	return true, nil
	//Send an "AddPeer" request to proxy.
	//Proxy will send a confirmed add peer request
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

func (n *Node) BootNode(url string) (bool, error) {
	server := n.Server()

	if server == nil {
		return false, ErrNodeStopped
	}
	reqNode := server.Self()

	BootNode, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	if BootNode.Incomplete() {
		return false, errors.New("boot node is incomplete")
	}
	//Above is vetting

	if err := n.CallProxy("BootNode", reqNode, BootNode); err != nil {
		return false, err
	}
	return true, nil

}

func (n *Node) CallProxy(method string, reqNode, targetNode *discover.Node) error {
	//Make connection with proxy
	ReqNodeID := strings.SplitAfter(reqNode.String(), "@")[0]
	conn, err := net.Dial("tcp", PeerProxyURL)
	if err != nil {
		return err
	}
	var request Request
	request.Method = method
	request.ReqNode = &proxyNode{
		ID:  ReqNodeID[:len(ReqNodeID)-1],
		IP:  reqNode.IP.String(),
		TCP: reqNode.TCP,
		RPC: uint16(n.config.HTTPPort),
	}

	request.TargetNode = &proxyNode{}
	if targetNode != nil {
		TargetNodeID := strings.SplitAfter(targetNode.String(), "@")[0]

		request.TargetNode = &proxyNode{
			ID:  TargetNodeID[:len(TargetNodeID)-1],
			IP:  targetNode.IP.String(),
			TCP: targetNode.TCP,
		}
	}

	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff) // Will write to network.
	if err := enc.Encode(request); err != nil {
		return err
	}
	if _, err = conn.Write(buff.Bytes()); err != nil {
		return err
	}
	err = conn.Close()
	if err != nil {
		return err
	}
	return nil
}
