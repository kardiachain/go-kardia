package node

import (
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/p2p"
	"errors"
	"sync"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"fmt"
)

var (
	ErrNodeStopped = errors.New("node not started")
	ErrNodeRunning = errors.New("node already running")
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

	if n.server != nil {
		return ErrNodeRunning
	}

	n.serverConfig = n.config.P2P
	n.serverConfig.Logger = n.log
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.PrivateKey = n.config.NodeKey()

	// TODO: use json file in datadir to load the node list
	// n.serverConfig.StaticNodes = []
	// n.serverConfig.TrustedNodes = ...
	// n.serverConfig.NodeDatabase = ...

	running := &p2p.Server{Config: n.serverConfig}
	n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)

	if err := running.Start(); err != nil {
		return err
	}

	// Next is to start all the API services for this node (talk with user and others)
	// if any error when starting, call the running.Stop()

	n.server = running
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
