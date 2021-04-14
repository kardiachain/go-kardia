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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/service"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	kaiproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/rpc"
)

var (
	testNodeKey, _ = crypto.GenerateKey()
)

func testNodeConfig() *Config {
	return &Config{
		Name: "test node",
		P2P:  configs.DefaultP2PConfig(),
		Genesis: &genesis.Genesis{
			ConsensusParams: &kaiproto.ConsensusParams{},
		},
	}
}

// Tests that an empty protocol stack can be closed more than once.
func TestNodeCloseMultipleTimes(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	stack.Stop()

	// Ensure that a stopped node can be stopped again
	for i := 0; i < 3; i++ {
		if err := stack.Stop(); err != service.ErrNodeStopped {
			t.Fatalf("iter %d: stop failure mismatch: have %v, want %v", i, err, service.ErrNodeStopped)
		}
	}
}

// Tests that a node can only started and closed once
func TestNodeStartMultipleTimes(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}

	// Ensure that a node can be successfully started, but only once
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	if err := stack.Start(); err != service.ErrNodeRunning {
		t.Fatalf("start failure mismatch: have %v, want %v ", err, service.ErrNodeRunning)
	}
	// Ensure that a node can be stopped, but only once
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop node: %v", err)
	}
	if err := stack.Stop(); err != service.ErrNodeStopped {
		t.Fatalf("stop failure mismatch: have %v, want %v ", err, service.ErrNodeStopped)
	}
}

// Tests that if the data dir is already in use, an appropriate error is returned.
func TestNodeUsedDataDir(t *testing.T) {
	// Create a temporary folder to use as the data directory
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create temporary data directory: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create a new node based on the data directory
	cfg := testNodeConfig()
	cfg.DataDir = dir
	original, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create original protocol stack: %v", err)
	}
	defer original.Stop()
	if err := original.Start(); err != nil {
		t.Fatalf("failed to start original protocol stack: %v", err)
	}

	// Create a second node based on the same data directory and ensure failure
	_, err = New(cfg)
	if err != service.ErrDatadirUsed {
		t.Fatalf("duplicate datadir failure mismatch: have %v, want %v", err, service.ErrDatadirUsed)
	}
}

// Tests whether services can be registered and duplicates caught.
func TestServiceRegistry(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Register a batch of unique services and ensure they start successfully
	services := []ServiceConstructor{NewNoopServiceA, NewNoopServiceB, NewNoopServiceC}
	for i, constructor := range services {
		if err := stack.Register(constructor); err != nil {
			t.Fatalf("service #%d: registration failed: %v", i, err)
		}
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start original service stack: %v", err)
	}
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop original service stack: %v", err)
	}

}

// Tests that registered services get started and stopped correctly.
func TestServiceLifeCycle(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Register a batch of life-cycle instrumented services
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
		id := id // Closure for the constructor
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Switch) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
	// Start the node and check that all services are running
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	for id := range services {
		if !started[id] {
			t.Fatalf("service %s: freshly started service not running", id)
		}
		if stopped[id] {
			t.Fatalf("service %s: freshly started service already stopped", id)
		}
	}
	// Stop the node and check that all services have been stopped
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop protocol stack: %v", err)
	}
	for id := range services {
		if !stopped[id] {
			t.Fatalf("service %s: freshly terminated service still running", id)
		}
	}
}

// Tests whether a service's api can be registered properly on the node server.
func TestRegisterAPIs(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	fs, err := NewAPIService(stack)
	if err != nil {
		t.Fatalf("could not create full service: %v", err)
	}

	for _, api := range fs.APIs() {
		if !containsAPI(stack.rpcAPIs, api) {
			t.Fatalf("api %v was not successfully registered", api)
		}
	}
}

// Tests that services are restarted cleanly as new instances.
func TestServiceRestarts(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Define a service that does not support restarts
	var (
		running bool
		started int
	)
	constructor := func(*ServiceContext) (Service, error) {
		running = false

		return &InstrumentedService{
			startHook: func(*p2p.Switch) {
				if running {
					panic("already running")
				}
				running = true
				started++
			},
		}, nil
	}
	// Register the service and start the protocol stack
	if err := stack.Register(constructor); err != nil {
		t.Fatalf("failed to register the service: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	defer stack.Stop()

	if !running || started != 1 {
		t.Fatalf("running/started mismatch: have %v/%d, want true/1", running, started)
	}

}

// Tests that if a service fails to initialize itself, none of the other services
// will be allowed to even start.
func TestServiceConstructionAbortion(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Define a batch of good services
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	for id, maker := range services {
		id := id // Closure for the constructor
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Switch) { started[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
	// Register a service that fails to construct itself
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return nil, failure
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}

}

// Tests that if a service fails to start, all others started before it will be
// shut down.
func TestServiceStartupAbortion(t *testing.T) {
	stack, err := New(testNodeConfig())
	assert.Nil(t, err, "failed to create protocol stack")
	defer stack.Stop()

	// Register a batch of good services
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
		id := id // Closure for the constructor
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Switch) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		assert.Nil(t, stack.Register(maker(constructor)), "service %s: registration failed", id)
	}
	// Register a service that fails to start
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return &InstrumentedService{
			start: failure,
		}, nil
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}

}

// Tests that even if a registered service fails to shut down cleanly, it does
// not influece the rest of the shutdown invocations.
func TestServiceTerminationGuarantee(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Register a batch of good services
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
		id := id // Closure for the constructor
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Switch) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
	// Register a service that fails to shot down cleanly
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return &InstrumentedService{
			stop: failure,
		}, nil
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}

}

// TestServiceRetrieval tests that individual services can be retrieved.
func TestServiceRetrieval(t *testing.T) {
	// Create a simple stack and register two service types
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	if err := stack.Register(NewNoopService); err != nil {
		t.Fatalf("noop service registration failed: %v", err)
	}
	if err := stack.Register(NewInstrumentedService); err != nil {
		t.Fatalf("instrumented service registration failed: %v", err)
	}

	// Start the stack and ensure everything is retrievable now
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start stack: %v", err)
	}
	defer stack.Stop()

}

// Tests that all protocols defined by individual services get launched.
func TestProtocolGather(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	defer stack.Stop()

	// Register a batch of services with some configured number of protocols
	services := map[string]struct {
		Count int
		Maker InstrumentingWrapper
	}{
		"zero": {0, InstrumentedServiceMakerA},
		"one":  {1, InstrumentedServiceMakerB},
		"many": {10, InstrumentedServiceMakerC},
	}
	for id, config := range services {

		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{}, nil
		}
		if err := stack.Register(config.Maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
	// Start the services and ensure all protocols start successfully
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	defer stack.Stop()

}

// Tests whether a handler can be successfully mounted on the canonical HTTP server
// on the given prefix
func TestRegisterHandler_Successful(t *testing.T) {
	node := createNode(t, 7878, 7979)

	// create and mount handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success"))
	})
	node.RegisterHandler("test", "/test", handler)

	// start node
	if err := node.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}
	defer node.Stop()

	// create HTTP request
	httpReq, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:7878/test", nil)
	if err != nil {
		t.Error("could not issue new http request ", err)
	}

	// check response
	resp := doHTTPRequest(t, httpReq)
	buf := make([]byte, 7)
	_, err = io.ReadFull(resp.Body, buf)
	if err != nil {
		t.Fatalf("could not read response: %v", err)
	}
	assert.Equal(t, "success", string(buf))
}

// Tests that the given handler will not be successfully mounted since no HTTP server
// is enabled for RPC
func TestRegisterHandler_Unsuccessful(t *testing.T) {
	node, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("could not create new node: %v", err)
	}

	// create and mount handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success"))
	})
	node.RegisterHandler("test", "/test", handler)
}

// Tests whether websocket requests can be handled on the same port as a regular http server.
func TestWebsocketHTTPOnSamePort_WebsocketRequest(t *testing.T) {
	node := startHTTP(t, 0, 0)
	defer node.Stop()

	ws := strings.Replace(node.HTTPEndpoint(), "http://", "ws://", 1)

	if node.WSEndpoint() != ws {
		t.Fatalf("endpoints should be the same")
	}
	if !checkRPC(ws) {
		t.Fatalf("ws request failed")
	}
	if !checkRPC(node.HTTPEndpoint()) {
		t.Fatalf("http request failed")
	}
}

func TestWebsocketHTTPOnSeparatePort_WSRequest(t *testing.T) {
	// try and get a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("can't listen:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	node := startHTTP(t, 0, port)
	defer node.Stop()

	wsOnHTTP := strings.Replace(node.HTTPEndpoint(), "http://", "ws://", 1)
	ws := fmt.Sprintf("ws://127.0.0.1:%d", port)

	if node.WSEndpoint() == wsOnHTTP {
		t.Fatalf("endpoints should not be the same")
	}
	// ensure ws endpoint matches the expected endpoint
	if node.WSEndpoint() != ws {
		t.Fatalf("ws endpoint is incorrect: expected %s, got %s", ws, node.WSEndpoint())
	}

	if !checkRPC(ws) {
		t.Fatalf("ws request failed")
	}
	if !checkRPC(node.HTTPEndpoint()) {
		t.Fatalf("http request failed")
	}
}

// Test helper functions
func createNode(t *testing.T, httpPort, wsPort int) *Node {
	conf := testNodeConfig()
	conf.HTTPHost = "127.0.0.1"
	conf.HTTPPort = httpPort
	conf.WSHost = "127.0.0.1"
	conf.WSPort = wsPort
	node, err := New(conf)
	if err != nil {
		t.Fatalf("could not create a new node: %v", err)
	}
	return node
}

func startHTTP(t *testing.T, httpPort, wsPort int) *Node {
	node := createNode(t, httpPort, wsPort)
	err := node.Start()
	if err != nil {
		t.Fatalf("could not start http service on node: %v", err)
	}

	return node
}

func doHTTPRequest(t *testing.T, req *http.Request) *http.Response {
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("could not issue a GET request to the given endpoint: %v", err)

	}
	return resp
}

func containsAPI(stackAPIs []rpc.API, api rpc.API) bool {
	for _, a := range stackAPIs {
		if reflect.DeepEqual(a, api) {
			return true
		}
	}
	return false
}
