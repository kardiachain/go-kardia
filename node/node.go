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
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/tsdb/fileutil"

	"github.com/kardiachain/go-kardia/blockchain"
	cs "github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kai/accounts"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/p2p/pex"
	bs "github.com/kardiachain/go-kardia/lib/service"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence"
)

var (
	nodeVersion = "1.5.1"
)

// Node is a container on which services can be registered.
type Node struct {
	bs.BaseService

	eventmux   *event.Feed // Event multiplexer used between the services of a stack
	config     *Config
	accMan     *accounts.Manager
	logger     log.Logger
	keyDir     string // key store directory
	keyDirTemp bool   // If true, key directory will be removed by Stop

	ephemeralKeystore string            // if non-empty, the key directory that will be removed by Stop
	instanceDirLock   fileutil.Releaser // prevents concurrent use of instance directory

	stop       chan struct{} // Channel to wait for termination notifications
	sw         *p2p.Switch   // p2p connections
	nodeKey    *p2p.NodeKey
	transport  *p2p.MultiplexTransport
	addrBook   pex.AddrBook // known peers
	pexReactor *pex.Reactor

	lock          sync.RWMutex
	lifecycles    []Lifecycle // All registered backends, services, and auxiliary services that have a lifecycle
	rpcAPIs       []rpc.API   // List of APIs currently provided by the node
	http          *httpServer //
	ws            *httpServer //
	ipc           *ipcServer  // Stores information about the ipc http server
	inprocHandler *rpc.Server // In-process RPC request handler to process the API requests
}

// New creates a new P2P node, ready for protocol registration.
func New(conf *Config) (*Node, error) {
	// Copy config and resolve the datadir so future changes to the current
	// working directory don't affect the node.
	confCopy := *conf
	conf = &confCopy
	if conf.DataDir != "" {
		absdatadir, err := filepath.Abs(conf.DataDir)
		if err != nil {
			return nil, err
		}
		conf.DataDir = absdatadir
	}

	// Ensure that the instance name doesn't cause weird conflicts with
	// other files in the data directory.
	if strings.ContainsAny(conf.Name, `/\`) {
		return nil, errors.New(`Config.Name must not contain '/' or '\'`)
	}
	if conf.Name == datadirDefaultKeyStore {
		return nil, errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	}
	if strings.HasSuffix(conf.Name, ".ipc") {
		return nil, errors.New(`Config.Name cannot end in ".ipc"`)
	}

	logger := log.New()

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	node := &Node{
		config:        conf,
		inprocHandler: rpc.NewServer(),
		lifecycles:    []Lifecycle{},
		eventmux:      new(event.Feed),
		logger:        logger,
		stop:          make(chan struct{}),
	}
	node.BaseService = *bs.NewBaseService(logger, "Node", node)

	// Register built-in APIs.
	node.rpcAPIs = append(node.rpcAPIs, node.apis()...)

	// Acquire the instance directory lock.
	if err := node.openDataDir(); err != nil {
		return nil, err
	}

	keyDir, isEphem, err := conf.GetKeyStoreDir()
	if err != nil {
		return nil, err
	}
	node.keyDir = keyDir
	node.keyDirTemp = isEphem
	// Creates an empty AccountManager with no backends. Callers (e.g. cmd/main.go)
	// are required to add the backends later on.
	node.accMan = accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: conf.InsecureUnlockAllowed})

	// Setting up the p2p server
	node.nodeKey = &p2p.NodeKey{PrivKey: conf.NodeKey()}
	nodeInfo, err := makeNodeInfo(node.config, node.nodeKey)
	if err != nil {
		return nil, err
	}
	transport, peerFilters := createTransport(node.config, nodeInfo, node.nodeKey)
	node.transport = transport
	node.sw = createSwitch(
		node.config, node.transport, peerFilters, nodeInfo, node.nodeKey, node.logger,
	)

	// Configure RPC servers.
	node.http = newHTTPServer(node.logger, conf.HTTPTimeouts)
	node.ws = newHTTPServer(node.logger, rpc.DefaultHTTPTimeouts)
	node.ipc = newIPCServer(node.logger, conf.IPCEndpoint())

	return node, nil
}

// RegisterLifecycle registers the given Lifecycle on the node.
func (n *Node) RegisterLifecycle(lifecycle Lifecycle) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.lifecycles = append(n.lifecycles, lifecycle)
}

// Start create a live P2P node and starts running it.
func (n *Node) OnStart() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	err := n.sw.AddPersistentPeers(n.config.P2P.PersistentPeers)
	if err != nil {
		return fmt.Errorf("could not add peers from persistent_peers field: %w", err)
	}

	err = n.sw.AddUnconditionalPeerIDs(n.config.P2P.UnconditionalPeerIDs)
	if err != nil {
		return fmt.Errorf("could not add peer ids from unconditional_peer_ids field: %w", err)
	}

	addrBook, err := createAddrBookAndSetOnSwitch(n.config, n.sw, n.logger, n.nodeKey)
	if err != nil {
		return fmt.Errorf("could not create addrbook: %w", err)
	}

	var pexReactor *pex.Reactor
	if n.config.P2P.PexReactor {
		pexReactor = createPEXReactorAndAddToSwitch(addrBook, n.config, n.sw, n.logger)
	}

	n.addrBook = addrBook
	n.pexReactor = pexReactor

	// Start the transport.
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(n.nodeKey.ID(), n.config.P2P.ListenAddress))
	if err != nil {
		return err
	}
	if err := n.transport.Listen(*addr); err != nil {
		return err
	}

	// Start all registered lifecycles.
	for _, lifecycle := range n.lifecycles {
		if err = lifecycle.Start(); err != nil {
			break
		}
	}

	// Check if any lifecycle failed to start.
	if err != nil {
		n.Stop()
	}

	// start RPC endpoints
	if err := n.openRPCEndpoints(); err != nil {
		if err := n.Stop(); err != nil {
			return err
		}
		return err
	}

	n.stop = make(chan struct{})

	// Start the switch (the P2P server).
	if err := n.sw.Start(); err != nil {
		return err
	}

	// Always connect to persistent peers
	err = n.sw.DialPeersAsync(n.config.P2P.PersistentPeers)
	if err != nil {
		return fmt.Errorf("could not dial peers from persistent_peers field: %w", err)
	}

	return nil
}

// Config returns the configuration of node.
func (n *Node) Config() *Config {
	return n.config
}

func (n *Node) openDataDir() error {
	if n.config.DataDir == "" {
		return nil // ephemeral
	}

	instdir := filepath.Join(n.config.DataDir, n.config.name())
	if err := os.MkdirAll(instdir, 0700); err != nil {
		return err
	}
	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	release, _, err := fileutil.Flock(filepath.Join(instdir, "LOCK"))
	if err != nil {
		return bs.ConvertFileLockError(err)
	}
	n.instanceDirLock = release
	return nil
}

// openRPCEndpoints start RPC or return its error handler
func (n *Node) openRPCEndpoints() error {
	n.logger.Info("Starting RPC endpoints")
	if err := n.startRPC(); err != nil {
		n.logger.Error("Failed to start RPC endpoints", "err", err)
		n.stopRPC()
		n.Stop()
	}
	return nil
}

// startRPC is a helper method to start all the various RPC endpoint during node
// startup. It's not meant to be called at any time afterwards as it makes certain
// assumptions about the state of the node.
func (n *Node) startRPC() error {
	if err := n.startInProc(); err != nil {
		return err
	}

	// Configure IPC.
	if n.ipc.endpoint != "" {
		if err := n.ipc.start(n.rpcAPIs); err != nil {
			return err
		}
	}

	// Configure HTTP.
	if n.config.HTTPHost != "" {
		config := httpConfig{
			CorsAllowedOrigins: n.config.HTTPCors,
			Vhosts:             n.config.HTTPVirtualHosts,
			Modules:            n.config.HTTPModules,
		}
		if err := n.http.setListenAddr(n.config.HTTPHost, n.config.HTTPPort); err != nil {
			return err
		}
		if err := n.http.enableRPC(n.rpcAPIs, config); err != nil {
			return err
		}
	}

	// Configure WebSocket.
	if n.config.WSHost != "" {
		server := n.wsServerForPort(n.config.WSPort)
		config := wsConfig{
			Modules: n.config.WSModules,
			Origins: n.config.WSOrigins,
		}
		if err := server.setListenAddr(n.config.WSHost, n.config.WSPort); err != nil {
			return err
		}
		if err := server.enableWS(n.rpcAPIs, config); err != nil {
			return err
		}
	}

	if err := n.http.start(); err != nil {
		return err
	}
	return n.ws.start()
}

func (n *Node) stopRPC() {
	n.http.stop()
	n.ws.stop()
	n.ipc.stop()
	n.stopInProc()
}

// startInProc registers all RPC APIs on the inproc server.
func (n *Node) startInProc() error {
	for _, api := range n.rpcAPIs {
		if err := n.inprocHandler.RegisterName(api.Namespace, api.Service); err != nil {
			return err
		}
	}
	return nil
}

// stopInProc terminates the in-process RPC endpoint.
func (n *Node) stopInProc() {
	n.inprocHandler.Stop()
}

// Stop terminates a running node along with all it's services. In the node was
// not started, an error is returned.
func (n *Node) OnStop() {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Terminate the API, services and the p2p server.
	n.stopRPC()
	n.rpcAPIs = nil
	failure := &bs.StopError{
		Services: make(map[reflect.Type]error),
	}
	for _, service := range n.lifecycles {
		if err := service.Stop(); err != nil {
			failure.Services[reflect.TypeOf(service)] = err
		}
	}

	// Stop accounts manager
	if err := n.accMan.Close(); err != nil {
		n.Logger.Error("Error closing accounts manager", "err", err)
	}

	if err := n.sw.Stop(); err != nil {
		n.Logger.Error("Error closing switch", "err", err)
	}

	if err := n.transport.Close(); err != nil {
		n.Logger.Error("Error closing transport", "err", err)
	}

	// Release instance directory lock.
	if n.instanceDirLock != nil {
		if err := n.instanceDirLock.Release(); err != nil {
			n.Logger.Error("Can't release datadir lock", "err", err)
		}
		n.instanceDirLock = nil
	}

	// unblock n.Wait
	close(n.stop)

	// Remove the keystore if it was created ephemerally.
	var keystoreErr error
	if n.ephemeralKeystore != "" {
		keystoreErr = os.RemoveAll(n.ephemeralKeystore)
	}

	if len(failure.Services) > 0 {
		n.Logger.Error("failure", "err", failure)
	}
	if keystoreErr != nil {
		n.Logger.Error("keystoreErr", "err", failure)
	}
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (n *Node) Wait() {
	<-n.stop
}

// Restart terminates a running node and boots up a new one in its place. If the
// node isn't running, an error is returned.
func (n *Node) Restart() error {
	if err := n.Stop(); err != nil {
		return err
	}
	if err := n.Start(); err != nil {
		return err
	}
	return nil
}

// RegisterHandler mounts a handler on the given path on the canonical HTTP server.
//
// The name of the handler is shown in a log message when the HTTP server starts
// and should be a descriptive term for the service provided by the handler.
func (n *Node) RegisterHandler(name, path string, handler http.Handler) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.http.mux.Handle(path, handler)
	n.http.handlerNames[path] = name
}

// RegisterAPIs registers the APIs a service provides on the node.
func (n *Node) RegisterAPIs(apis []rpc.API) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.rpcAPIs = append(n.rpcAPIs, apis...)
}

// DataDir retrieves the current datadir used by the protocol stack.
// Deprecated: No files should be stored in this directory, use InstanceDir instead.
func (n *Node) DataDir() string {
	return n.config.DataDir
}

// InstanceDir retrieves the instance directory used by the protocol stack.
func (n *Node) InstanceDir() string {
	return n.config.instanceDir()
}

// IPCEndpoint retrieves the current IPC endpoint used by the protocol stack.
func (n *Node) IPCEndpoint() string {
	return n.ipc.endpoint
}

// HTTPEndpoint returns the URL of the HTTP server.
func (n *Node) HTTPEndpoint() string {
	return "http://" + n.http.listenAddr()
}

// WSEndpoint returns the current JSON-RPC over WebSocket endpoint.
func (n *Node) WSEndpoint() string {
	if n.http.wsAllowed() {
		return "ws://" + n.http.listenAddr()
	}
	return "ws://" + n.ws.listenAddr()
}

func (n *Node) wsServerForPort(port int) *httpServer {
	if n.config.HTTPHost == "" || n.http.port == port {
		return n.http
	}
	return n.ws
}

// EventMux retrieves the event multiplexer used by all the network services in
// the current protocol stack.
func (n *Node) EventMux() *event.Feed {
	return n.eventmux
}

// KeyStoreDir retrieves the key directory
func (n *Node) KeyStoreDir() string {
	return n.keyDir
}

// AccountManager retrieves the account manager used by the protocol stack.
func (n *Node) AccountManager() *accounts.Manager {
	return n.accMan
}

// OpenDatabase opens an existing database with the given name (or creates one if no
// previous can be found) from within the node's instance directory. If the node is
// ephemeral, a memory database is returned.
func (n *Node) OpenDatabase(name string, cache, handles int, namespace string) (types.StoreDB, error) {
	if n.config.DataDir == "" {
		return rawdb.NewMemoryDatabase(), nil
	}
	return rawdb.NewLevelDBDatabase(n.config.ResolvePath(name), cache, handles, namespace)
}

// ResolvePath returns the absolute path of a resource in the instance directory.
func (n *Node) ResolvePath(x string) string {
	return n.config.ResolvePath(x)
}

// P2PSwitch retrieves the currently running P2P network layer. This method is meant
// only to inspect fields of the currently running server. Callers should not
// start or stop the returned p2p switch.
func (n *Node) P2PSwitch() *p2p.Switch {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.sw
}

func createTransport(
	config *Config,
	nodeInfo p2p.NodeInfo,
	nodeKey *p2p.NodeKey,
) (
	*p2p.MultiplexTransport,
	[]p2p.PeerFilterFunc,
) {
	var (
		mConnConfig = p2p.MConnConfig(config.P2P)
		transport   = p2p.NewMultiplexTransport(nodeInfo, *nodeKey, mConnConfig)
		peerFilters = []p2p.PeerFilterFunc{}
	)

	// Limit the number of incoming connections.
	max := config.P2P.MaxNumInboundPeers + len(config.P2P.UnconditionalPeerIDs)
	p2p.MultiplexTransportMaxIncomingConnections(max)(transport)

	return transport, peerFilters
}

func makeNodeInfo(
	config *Config,
	nodeKey *p2p.NodeKey,
) (p2p.NodeInfo, error) {
	txIndexerStatus := "on"

	nodeInfo := p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(
			uint64(1), // global
			uint64(1),
			uint64(1),
		),
		DefaultNodeID: nodeKey.ID(),
		Network:       "", // TRICK! All running nodes have network id of blank ;)
		Version:       nodeVersion,
		Channels: []byte{
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			evidence.EvidenceChannel, tx_pool.TxpoolChannel,
		},
		Moniker: config.Name,
		Other: p2p.DefaultNodeInfoOther{
			TxIndex: txIndexerStatus,
		},
	}

	if config.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}
	if config.FastSync != nil {
		nodeInfo.Channels = append(nodeInfo.Channels, blockchain.BlockchainChannel)
	}

	lAddr := config.P2P.ExternalAddress

	if lAddr == "" {
		lAddr = config.P2P.ListenAddress
	}

	nodeInfo.ListenAddr = lAddr

	err := nodeInfo.Validate()
	return nodeInfo, err
}

func createSwitch(config *Config,
	transport p2p.Transport,
	peerFilters []p2p.PeerFilterFunc,
	nodeInfo p2p.NodeInfo,
	nodeKey *p2p.NodeKey,
	p2pLogger log.Logger) *p2p.Switch {

	sw := p2p.NewSwitch(
		config.P2P,
		transport,
	)
	sw.SetLogger(p2pLogger)

	sw.SetNodeInfo(nodeInfo)
	sw.SetNodeKey(nodeKey)

	return sw
}

func createAddrBookAndSetOnSwitch(config *Config, sw *p2p.Switch,
	p2pLogger log.Logger, nodeKey *p2p.NodeKey) (pex.AddrBook, error) {

	addrBook := pex.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
	addrBook.SetLogger(p2pLogger)

	// Add ourselves to addrbook to prevent dialing ourselves
	if config.P2P.ExternalAddress != "" {
		addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeKey.ID(), config.P2P.ExternalAddress))
		if err != nil {
			return nil, fmt.Errorf("p2p.external_address is incorrect: %w", err)
		}
		addrBook.AddOurAddress(addr)
	}
	if config.P2P.ListenAddress != "" {
		addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeKey.ID(), config.P2P.ListenAddress))
		if err != nil {
			return nil, fmt.Errorf("p2p.laddr is incorrect: %w", err)
		}
		addrBook.AddOurAddress(addr)
	}

	sw.SetAddrBook(addrBook)

	return addrBook, nil
}

func createPEXReactorAndAddToSwitch(addrBook pex.AddrBook, config *Config,
	sw *p2p.Switch, logger log.Logger) *pex.Reactor {

	// TODO persistent peers ? so we can have their DNS addrs saved
	pexReactor := pex.NewReactor(addrBook,
		&pex.ReactorConfig{
			Seeds:    config.P2P.Seeds,
			SeedMode: config.P2P.SeedMode,
			// blocksToContributeToBecomeGoodPeer 10000
			// blocks assuming 5s+ blocks ~ 14 hours.
			SeedDisconnectWaitPeriod:     14 * time.Hour,
			PersistentPeersMaxDialPeriod: config.P2P.PersistentPeersMaxDialPeriod,
		})
	pexReactor.SetLogger(logger)
	sw.AddReactor("PEX", pexReactor)
	return pexReactor
}
