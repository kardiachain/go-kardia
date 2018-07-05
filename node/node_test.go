package node

import (
	"github.com/kardiachain/go-kardia/crypto"
	"github.com/kardiachain/go-kardia/p2p"
	"testing"
)

var (
	testNodeKey, _ = crypto.GenerateKey()
)

func testNodeConfig() *NodeConfig {
	return &NodeConfig{
		Name: "test node",
		P2P:  p2p.Config{PrivateKey: testNodeKey},
	}
}

// Tests that an empty protocol stack can be started, restarted and stopped.
func TestNodeLifeCycle(t *testing.T) {
	node, err := NewNode(testNodeConfig())
	if err != nil {
		t.Fatalf("fail when create node: %v", err)
	}
	// Tests stopping node that not running.
	if err := node.Stop(); err != ErrNodeStopped {
		t.Fatalf("unexpected stop error: %v instead of %v", err, ErrNodeStopped)
	}

	// Tests starting node 2 times
	if err := node.Start(); err != nil {
		t.Fatalf("fail when start node: %v", err)
	}
	if err := node.Start(); err != ErrNodeRunning {
		t.Fatalf("unexpected start error: %v instead of %v ", err, ErrNodeRunning)
	}
	// Tests stopping node 2 times
	if err := node.Stop(); err != nil {
		t.Fatalf("fail when stop node: %v", err)
	}
	if err := node.Stop(); err != ErrNodeStopped {
		t.Fatalf("unexpected stop error: %v instead of %v ", err, ErrNodeStopped)
	}
}
