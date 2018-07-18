package node

import (
	"github.com/kardiachain/go-kardia/crypto"
	"github.com/kardiachain/go-kardia/p2p"
	"testing"
)

// Signatures of a simple test service.
type TrivialService struct {
	Started bool
}

func (s *TrivialService) Protocols() []p2p.Protocol {
	return nil
}
func (s *TrivialService) Start(*p2p.Server) error {
	s.Started = true
	return nil
}
func (s *TrivialService) Stop() error {
	s.Started = false
	return nil
}
func newTrivialService(ctx *ServiceContext) (Service, error) { return new(TrivialService), nil }

var (
	testNodeKey, _ = crypto.GenerateKey()
)

func testNodeConfig() *NodeConfig {
	return &NodeConfig{
		Name: "test node",
		P2P:  p2p.Config{PrivateKey: testNodeKey},
	}
}

// Tests that an empty node without service can be started, restarted and stopped.
func TestNodeLifeCycle(t *testing.T) {
	node, err := NewNode(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	// Tests stopping node that not running.
	if err := node.Stop(); err != ErrNodeStopped {
		t.Fatalf("unexpected stop error: %v instead of %v", err, ErrNodeStopped)
	}

	// Tests starting node 2 times
	if err := node.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	if err := node.Start(); err != ErrNodeRunning {
		t.Fatalf("unexpected start error: %v instead of %v ", err, ErrNodeRunning)
	}
	// Tests stopping node 2 times
	if err := node.Stop(); err != nil {
		t.Fatalf("failed to stop node: %v", err)
	}
	if err := node.Stop(); err != ErrNodeStopped {
		t.Fatalf("unexpected stop error: %v instead of %v ", err, ErrNodeStopped)
	}
}

func TestNodeRegisteringService(t *testing.T) {
	node, err := NewNode(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := node.RegisterService(newTrivialService); err != nil {
		t.Fatalf("failed to register TrivialService: %v", err)
	}

	if err := node.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}

	var service *TrivialService

	if err := node.Service(&service); err != nil {
		t.Fatalf("TrivialService is not in service list of node: %v", err)
	}
	if !service.Started {
		t.Fatalf("TrivialService didn't run Start()")
	}
	if err := node.Stop(); err != nil {
		t.Fatalf("failed to stop node: %v", err)
	}
	if service.Started {
		t.Fatalf("TrivialService didn't run Stop()")
	}
}
