package node

import (
	"conceptchain/log"
	"conceptchain/p2p"
	"errors"
	"sync"
	"conceptchain/p2p/discover"
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
	Server       *p2p.Server

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

	if n.Server != nil {
		return ErrNodeRunning
	}

	n.serverConfig = n.config.P2P
	n.serverConfig.Logger = n.log
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.PrivateKey = n.config.NodeKey()

	// TODO: use json file in datadir to load the node list
	var nodes []*discover.Node
	peer, err := discover.ParseNode("enode://1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439@127.0.0.1:3000")

	if err != nil {
		n.log.Error(err.Error())
		panic(err)
	}
	n.log.Info("Discovered Node:", "discover.node", peer, "incomplete", peer.Incomplete())

	nodes = append(nodes, peer)

	n.serverConfig.StaticNodes = nodes
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

	n.Server = running
	return nil
}

func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.Server == nil {
		return ErrNodeStopped
	}

	n.Server.Stop()
	n.Server = nil

	return nil
}
