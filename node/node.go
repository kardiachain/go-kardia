package node

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kai"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/types"
)

// Node is the highest level container for a full Kardia node.
// It keeps all config data and services.
type Node struct {
	config       *kai.NodeConfig
	serverConfig p2p.Config
	server       *p2p.Server

	services            map[string]kai.Service // Map of type names to running services
	serviceConstructors []kai.ServiceConstructor

	csReactor     *consensus.ConsensusReactor
	privValidator *types.PrivValidator

	lock sync.RWMutex
	log  log.Logger
}

func NewNode(config *kai.NodeConfig) (*Node, error) {
	node := new(Node)
	node.config = config
	node.log = log.New()
	// TODO: input check on the config

	// Initialization for consensus.
	startTime, _ := time.Parse(time.UnixDate, "Monday July 30 00:00:00 PST 2018")
	validatorSet := config.DevEnvConfig.GetValidatorSet(config.NumValidators)
	state := state.LastestBlockState{
		ChainID:                     "kaicon",
		LastBlockHeight:             cmn.NewBigInt(0),
		LastBlockID:                 types.BlockID{},
		LastBlockTime:               startTime,
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt(1),
	}
	// Consensus config is imported from:
	// http://tendermint.readthedocs.io/en/master/specification/configuration.html
	// TODO(namdoh): Move this to config loader.
	csConfig := configs.ConsensusConfig{
		TimeoutPropose:            3000,
		TimeoutProposeDelta:       500,
		TimeoutPrevote:            1000,
		TimeoutPrevoteDelta:       500,
		TimeoutPrecommit:          1000,
		TimeoutPrecommitDelta:     500,
		TimeoutCommit:             1000,
		SkipTimeoutCommit:         false,
		CreateEmptyBlocks:         true,
		CreateEmptyBlocksInterval: 0,
	}
	consensusState := consensus.NewConsensusState(
		&csConfig,
		state,
	)

	node.csReactor = consensus.NewConsensusReactor(consensusState)

	return node, nil
}

func (n *Node) Start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Starts p2p server.
	if n.server != nil {
		return kai.ErrNodeRunning
	}
	n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)

	// Generate node PrivKey
	nodeKey := n.config.NodeKey()
	n.serverConfig = n.config.P2P
	n.serverConfig.Logger = n.log
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.PrivateKey = nodeKey
	n.privValidator = types.NewPrivValidator(nodeKey)
	n.csReactor.SetPrivValidator(n.privValidator)

	// TODO: use json file in datadir to load the node list
	// n.serverConfig.StaticNodes = []
	// n.serverConfig.TrustedNodes = ...
	// n.serverConfig.NodeDatabase = ...

	newServer := &p2p.Server{Config: n.serverConfig}

	// Starts protocol services.
	newServices := make(map[string]kai.Service)
	for _, serviceConstructor := range n.serviceConstructors {
		// Creates context as parameter for constructor
		ctx := &kai.ServiceContext{
			Config:   n.config,
			Services: make(map[string]kai.Service),
		}
		for serviceType, s := range newServices { // full map copy in each ServiceContext, for concurrent access
			ctx.Services[serviceType] = s
		}
		service, err := serviceConstructor(ctx)
		service.ConnectReactor(n.csReactor)
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
	var startedServices []kai.Service
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

	n.services = newServices
	n.server = newServer
	n.csReactor.SetNodeID(n.server.Self().ID)
	return nil
}

func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server == nil {
		return kai.ErrNodeStopped
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
		return kai.ErrNodeStopFailure
	}

	return nil
}

// Server returns p2p server of node.
func (n *Node) Server() *p2p.Server {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.server
}

// Service adds a new service to node.
func (n *Node) RegisterService(constructor kai.ServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return kai.ErrNodeRunning
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
		return false, kai.ErrNodeStopped
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
		return kai.ErrNodeStopped
	}

	// Get pointer with *Service type.
	pointer := reflect.ValueOf(returnedService).Elem()
	serviceName := pointer.Type().Elem().Name()

	if registeredS, ok := n.services[serviceName]; ok {
		pointer.Set(reflect.ValueOf(registeredS))

		return nil
	}
	return kai.ErrServiceUnknown
}

// ServiceMap returns map of all running services.
func (n *Node) ServiceMap() map[string]kai.Service {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.services
}
