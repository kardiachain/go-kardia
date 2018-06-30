package main

import (
	"sync"
	"errors"
	"github.com/ethereum/go-ethereum/p2p"
	// TODO: Should have been conceptchain/log but not compatible with serverConfig
	"github.com/ethereum/go-ethereum/log"
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

	serverConfig p2p.Config
	server *p2p.Server

	lock sync.RWMutex
	log log.Logger
}

func NewNode(name string) (*Node, error) {
	node := new(Node)

	// node.serverConfig.PrivateKey = the private key type
	node.serverConfig.Name = name
	node.serverConfig.Logger = node.log

	return node, nil
}

func (n *Node) start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return errors.New("server already exists")
	}

	// n.serverConfig.StaticNodes = ...
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

func (n *Node) stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server == nil {
		return errors.New("try to stop but server is not running")
	}

	n.server.Stop()
	n.server = nil

	return nil
}