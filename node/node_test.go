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
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/rpc"
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

func (s *TrivialService) APIs() []rpc.API {
	return nil
}

func newTrivialService(ctx *ServiceContext) (Service, error) { return new(TrivialService), nil }

var (
	testNodeKey, _ = crypto.GenerateKey()
)

func testNodeConfig() *NodeConfig {
	return &NodeConfig{
		Name:          "test node",
		P2P:           p2p.Config{PrivateKey: testNodeKey},
		NumValidators: 1,
		DevEnvConfig:  dev.CreateDevEnvironmentConfig(),
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
