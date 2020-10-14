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
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/kardiachain/go-kardiamain/configs"

	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/permissioned"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	datadirPrivateKey      = "nodekey"            // Path within the datadir to the node's private key
	datadirDefaultKeyStore = "keystore"           // Path within the datadir to the keystore
	datadirStaticNodes     = "static-nodes.json"  // Path within the datadir to the static node list
	datadirTrustedNodes    = "trusted-nodes.json" // Path within the datadir to the trusted node list
	datadirNodeDatabase    = "nodes"              // Path within the datadir to store the node infos
)

type MainChainConfig struct {
	// Mainchain

	// DbInfo stores configuration information to setup database
	DBInfo storage.DbInfo

	// Genesis is genesis block which contain initial Block and accounts
	Genesis *genesis.Genesis

	// Transaction pool options
	TxPool tx_pool.TxPoolConfig

	// AcceptTxs accept tx sync process or not (1 is yes and 0 is no)
	AcceptTxs uint32

	// IsZeroFee is true then sender will be refunded all gas spent for a transaction
	IsZeroFee bool

	// IsPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	NetworkId uint64

	ChainId uint64

	// ServiceName is used as log's prefix
	ServiceName string

	// BaseAccount defines account which is used to execute internal smart contracts
	BaseAccount *configs.BaseAccount
}

type DualChainConfig struct {
	// Dualchain

	ChainId uint64 // ID of dual chain unique to a dualnode group, such as for dual eth.

	// DbInfo stores configuration information to setup database
	DBInfo storage.DbInfo

	// Genesis is genesis block which contain initial Block and accounts
	DualGenesis *genesis.Genesis

	// Dual's event pool options
	DualEventPool event_pool.Config

	// IsPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	// Dual protocol name, this name is used if the node is setup as dual node
	DualProtocolName string

	// BaseAccount defines account which is used to execute internal smart contracts
	BaseAccount *configs.BaseAccount

	// Dual Network ID
	DualNetworkID uint64
}

// NodeMetadata contains privateKey and votingPower and function that get coinbase
type NodeMetadata struct {
	PrivKey     *ecdsa.PrivateKey
	PublicKey   *ecdsa.PublicKey
	VotingPower uint64
	ListenAddr  string
}

// EnvironmentConfig contains a list of NodeVotingPower, proposalIndex and votingStrategy
type EnvironmentConfig struct {
	NodeSet        []NodeMetadata
	proposalIndex  int
	VotingStrategy map[consensus.VoteTurn]int
}

// Config represents a small collection of configuration values to fine tune the
// P2P network layer of a protocol stack. These values can be further extended by
// all registered services.
type Config struct {
	// Name sets the instance name of the node. It must not contain the / character and is
	// used in the devp2p node identifier. The instance name of geth is "geth". If no
	// value is specified, the basename of the current executable is used.
	Name string `toml:"-"`

	// UserIdent, if set, is used as an additional component in the devp2p node identifier.
	UserIdent string `toml:",omitempty"`

	// Version should be set to the version number of the program. It is used
	// in the devp2p node identifier.
	Version string `toml:"-"`

	// DataDir is the file system folder the node should use for any data storage
	// requirements. The configured data directory will not be directly shared with
	// registered services, instead those can use utility methods to create/access
	// databases or flat files. This enables ephemeral nodes which can fully reside
	// in memory.
	DataDir string

	// Configuration of peer-to-peer networking.
	P2P *configs.P2PConfig

	// KeyStoreDir is the file system folder that contains private keys. The directory can
	// be specified as a relative path, in which case it is resolved relative to the
	// current directory.
	//
	// If KeyStoreDir is empty, the default location is the "keystore" subdirectory of
	// DataDir. If DataDir is unspecified and KeyStoreDir is empty, an ephemeral directory
	// is created by New and destroyed when the node is stopped.
	KeyStoreDir string `toml:",omitempty"`

	// ExternalSigner specifies an external URI for a clef-type signer
	ExternalSigner string `toml:"omitempty"`

	// UseLightweightKDF lowers the memory and CPU requirements of the key store
	// scrypt KDF at the expense of security.
	UseLightweightKDF bool `toml:",omitempty"`

	// InsecureUnlockAllowed allows user to unlock accounts in unsafe http environment.
	InsecureUnlockAllowed bool `toml:",omitempty"`

	// NoUSB disables hardware wallet monitoring and connectivity.
	NoUSB bool `toml:",omitempty"`

	// SmartCardDaemonPath is the path to the smartcard daemon's socket
	SmartCardDaemonPath string `toml:",omitempty"`

	// IPCPath is the requested location to place the IPC endpoint. If the path is
	// a simple file name, it is placed inside the data directory (or on the root
	// pipe path on Windows), whereas if it's a resolvable path name (absolute or
	// relative), then that specific path is enforced. An empty path disables IPC.
	IPCPath string `toml:",omitempty"`

	// HTTPHost is the host interface on which to start the HTTP RPC server. If this
	// field is empty, no HTTP API endpoint will be started.
	HTTPHost string `toml:",omitempty"`

	// HTTPPort is the TCP port number on which to start the HTTP RPC server. The
	// default zero value is/ valid and will pick a port number randomly (useful
	// for ephemeral nodes).
	HTTPPort int `toml:",omitempty"`

	// HTTPCors is the Cross-Origin Resource Sharing header to send to requesting
	// clients. Please be aware that CORS is a browser enforced security, it's fully
	// useless for custom HTTP clients.
	HTTPCors []string `toml:",omitempty"`

	// HTTPVirtualHosts is the list of virtual hostnames which are allowed on incoming requests.
	// This is by default {'localhost'}. Using this prevents attacks like
	// DNS rebinding, which bypasses SOP by simply masquerading as being within the same
	// origin. These attacks do not utilize CORS, since they are not cross-domain.
	// By explicitly checking the Host-header, the server will not allow requests
	// made against the server with a malicious host domain.
	// Requests using ip address directly are not affected
	HTTPVirtualHosts []string `toml:",omitempty"`

	// HTTPModules is a list of API modules to expose via the HTTP RPC interface.
	// If the module list is empty, all RPC API endpoints designated public will be
	// exposed.
	HTTPModules []string `toml:",omitempty"`

	// HTTPTimeouts allows for customization of the timeout values used by the HTTP RPC
	// interface.
	HTTPTimeouts rpc.HTTPTimeouts

	// WSHost is the host interface on which to start the websocket RPC server. If
	// this field is empty, no websocket API endpoint will be started.
	WSHost string `toml:",omitempty"`

	// WSPort is the TCP port number on which to start the websocket RPC server. The
	// default zero value is/ valid and will pick a port number randomly (useful for
	// ephemeral nodes).
	WSPort int `toml:",omitempty"`

	// WSOrigins is the list of domain to accept websocket requests from. Please be
	// aware that the server can only act upon the HTTP request the client sends and
	// cannot verify the validity of the request header.
	WSOrigins []string `toml:",omitempty"`

	// WSModules is a list of API modules to expose via the websocket RPC interface.
	// If the module list is empty, all RPC API endpoints designated public will be
	// exposed.
	WSModules []string `toml:",omitempty"`

	// WSExposeAll exposes all API modules via the WebSocket RPC interface rather
	// than just the public ones.
	//
	// *WARNING* Only set this if the node is running in a trusted network, exposing
	// private APIs to untrusted users is a major security risk.
	WSExposeAll bool `toml:",omitempty"`

	// GraphQLHost is the host interface on which to start the GraphQL server. If this
	// field is empty, no GraphQL API endpoint will be started.
	GraphQLHost string `toml:",omitempty"`

	// GraphQLPort is the TCP port number on which to start the GraphQL server. The
	// default zero value is/ valid and will pick a port number randomly (useful
	// for ephemeral nodes).
	GraphQLPort int `toml:",omitempty"`

	// GraphQLCors is the Cross-Origin Resource Sharing header to send to requesting
	// clients. Please be aware that CORS is a browser enforced security, it's fully
	// useless for custom HTTP clients.
	GraphQLCors []string `toml:",omitempty"`

	// GraphQLVirtualHosts is the list of virtual hostnames which are allowed on incoming requests.
	// This is by default {'localhost'}. Using this prevents attacks like
	// DNS rebinding, which bypasses SOP by simply masquerading as being within the same
	// origin. These attacks do not utilize CORS, since they are not cross-domain.
	// By explicitly checking the Host-header, the server will not allow requests
	// made against the server with a malicious host domain.
	// Requests using ip address directly are not affected
	GraphQLVirtualHosts []string `toml:",omitempty"`

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`

	staticNodesWarning     bool
	trustedNodesWarning    bool
	oldGethResourceWarning bool

	// Configuration of the Kardia's blockchain (or main chain).
	MainChainConfig MainChainConfig

	// Configuration of the dual's blockchain.
	DualChainConfig DualChainConfig

	// PeerProxyIP is IP of the network peer proxy, when participates in network with peer proxy for discovery.
	PeerProxyIP string

	// BaseAccount defines account which is used to execute internal smart contracts
	BaseAccount *configs.BaseAccount

	// Metrics defines whether we want to collect and expose metrics of the node
	Metrics uint

	// ======== DEV ENVIRONMENT CONFIG =========
	// Configuration of this node when running in dev environment.
	NodeMetadata *NodeMetadata
}

// IPCEndpoint resolves an IPC endpoint based on a configured value, taking into
// account the set data folders as well as the designated platform we're currently
// running on.
func (c *Config) IPCEndpoint() string {
	// Short circuit if IPC has not been enabled
	if c.IPCPath == "" {
		return ""
	}
	// On windows we can only use plain top-level pipes
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(c.IPCPath, `\\.\pipe\`) {
			return c.IPCPath
		}
		return `\\.\pipe\` + c.IPCPath
	}
	// Resolve names into the data directory full paths otherwise
	if filepath.Base(c.IPCPath) == c.IPCPath {
		if c.DataDir == "" {
			return filepath.Join(os.TempDir(), c.IPCPath)
		}
		return filepath.Join(c.DataDir, c.IPCPath)
	}
	return c.IPCPath
}

// NodeDB returns the path to the discovery node database.
func (c *Config) NodeDB() string {
	if c.DataDir == "" {
		return "" // ephemeral
	}
	return c.ResolvePath(datadirNodeDatabase)
}

// DefaultIPCEndpoint returns the IPC path used by default.
func DefaultIPCEndpoint(clientIdentifier string) string {
	if clientIdentifier == "" {
		clientIdentifier = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if clientIdentifier == "" {
			panic("empty executable name")
		}
	}
	config := &Config{DataDir: configs.DefaultDataDir(), IPCPath: clientIdentifier + ".ipc"}
	return config.IPCEndpoint()
}

// HTTPEndpoint resolves an HTTP endpoint based on the configured host interface
// and port parameters.
func (c *Config) HTTPEndpoint() string {
	if c.HTTPHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.HTTPHost, c.HTTPPort)
}

// GraphQLEndpoint resolves a GraphQL endpoint based on the configured host interface
// and port parameters.
func (c *Config) GraphQLEndpoint() string {
	if c.GraphQLHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.GraphQLHost, c.GraphQLPort)
}

// DefaultHTTPEndpoint returns the HTTP endpoint used by default.
func DefaultHTTPEndpoint() string {
	config := &Config{HTTPHost: DefaultHTTPHost, HTTPPort: DefaultHTTPPort}
	return config.HTTPEndpoint()
}

// WSEndpoint resolves a websocket endpoint based on the configured host interface
// and port parameters.
func (c *Config) WSEndpoint() string {
	if c.WSHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.WSHost, c.WSPort)
}

// DefaultWSEndpoint returns the websocket endpoint used by default.
func DefaultWSEndpoint() string {
	config := &Config{WSHost: DefaultWSHost, WSPort: DefaultWSPort}
	return config.WSEndpoint()
}

// ExtRPCEnabled returns the indicator whether node enables the external
// RPC(http, ws or graphql).
func (c *Config) ExtRPCEnabled() bool {
	return c.HTTPHost != "" || c.WSHost != "" || c.GraphQLHost != ""
}

// NodeName returns the devp2p node identifier.
func (c *Config) NodeName() string {
	name := c.name()
	// Backwards compatibility: previous versions used title-cased "Geth", keep that.
	if name == "geth" || name == "geth-testnet" {
		name = "Geth"
	}
	if c.UserIdent != "" {
		name += "/" + c.UserIdent
	}
	if c.Version != "" {
		name += "/v" + c.Version
	}
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

func (c *Config) name() string {
	if c.Name == "" {
		progname := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if progname == "" {
			panic("empty executable name, set Config.Name")
		}
		return progname
	}
	return c.Name
}

// These resources are resolved differently for "geth" instances.
var isOldGethResource = map[string]bool{
	"chaindata":          true,
	"nodes":              true,
	"nodekey":            true,
	"static-nodes.json":  false, // no warning for these because they have their
	"trusted-nodes.json": false, // own separate warning.
}

// ResolvePath resolves path in the instance directory.
func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if c.DataDir == "" {
		return ""
	}
	// Backwards-compatibility: ensure that data directory files created
	// by geth 1.4 are used if they exist.
	if warn, isOld := isOldGethResource[path]; isOld {
		oldpath := ""
		if c.name() == "geth" {
			oldpath = filepath.Join(c.DataDir, path)
		}
		if oldpath != "" && common.FileExist(oldpath) {
			if warn {
				c.warnOnce(&c.oldGethResourceWarning, "Using deprecated resource file %s, please move this file to the 'geth' subdirectory of datadir.", oldpath)
			}
			return oldpath
		}
	}
	return filepath.Join(c.instanceDir(), path)
}

func (c *Config) instanceDir() string {
	if c.DataDir == "" {
		return ""
	}
	return filepath.Join(c.DataDir, c.name())
}

// NodeKey retrieves the currently configured private key of the node, checking
// first any manually set key, falling back to the one found in the configured
// data folder. If no key can be found, a new one is generated.
func (c *Config) NodeKey() *ecdsa.PrivateKey {
	// Use any specifically configured key.
	if c.P2P.PrivateKey != nil {
		return c.P2P.PrivateKey
	}
	// Generate ephemeral key if no datadir is being used.
	if c.DataDir == "" {
		key, err := crypto.GenerateKey()
		if err != nil {
			log.Crit(fmt.Sprintf("Failed to generate ephemeral node key: %v", err))
		}
		return key
	}

	keyfile := c.ResolvePath(datadirPrivateKey)
	if key, err := crypto.LoadECDSA(keyfile); err == nil {
		return key
	}
	// No persistent key found, generate and store a new one.
	key, err := crypto.GenerateKey()
	if err != nil {
		log.Crit(fmt.Sprintf("Failed to generate node key: %v", err))
	}
	instanceDir := filepath.Join(c.DataDir, c.name())
	if err := os.MkdirAll(instanceDir, 0700); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
		return key
	}
	keyfile = filepath.Join(instanceDir, datadirPrivateKey)
	if err := crypto.SaveECDSA(keyfile, key); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
	}
	return key
}

var warnLock sync.Mutex

func (c *Config) warnOnce(w *bool, format string, args ...interface{}) {
	warnLock.Lock()
	defer warnLock.Unlock()

	if *w {
		return
	}
	l := c.Logger
	if l == nil {
		l = log.Root()
	}
	l.Warn(fmt.Sprintf(format, args...))
	*w = true
}

// NewNodeMetadata init new NodeMetadata
func NewNodeMetadata(privateKey *string, publicKey *string, votingPower uint64, listenAddr string) (*NodeMetadata, error) {

	node := &NodeMetadata{
		VotingPower: votingPower,
		ListenAddr:  listenAddr,
	}

	if privateKey == nil && publicKey == nil {
		return nil, fmt.Errorf("PrivateKey or PublicKey is required")
	}
	// Set PrivKey if privateKey is not nil
	if privateKey != nil {
		privKey, err := crypto.StringToPrivateKey(*privateKey)
		if err != nil {
			return nil, err
		}
		node.PrivKey = privKey
		node.PublicKey = &privKey.PublicKey
	}
	// Set PublicKey if publicKey is not nil
	if publicKey != nil {
		pubKey, err := crypto.StringToPublicKey(*publicKey)
		if err != nil {
			return nil, err
		}
		node.PublicKey = pubKey
	}
	return node, nil
}

// GetValidatorSet gets list of validators from permission smc defined in config and a list of indices.
func GetValidatorSet(bc base.BaseBlockChain, valIndexes []int) (*types.ValidatorSet, error) {
	nodes, err := GetNodeMetadataFromSmc(&bc, valIndexes)
	if err != nil {
		return nil, err
	}
	validators := make([]*types.Validator, 0)
	for i := 0; i < len(valIndexes); i++ {
		if valIndexes[i] < 0 {
			return nil, fmt.Errorf("value of validator must be greater than 0")
		}
		node := nodes[i]
		validators = append(validators, types.NewValidator(crypto.PubkeyToAddress(*node.PublicKey), node.VotingPower))
	}
	// TODO(huny@): Pass the start/end block height of the initial set of validator from the
	// genesis here. Default to 0 and 100000000000 for now.
	validatorSet := types.NewValidatorSet(validators)
	return validatorSet, nil
}

// GetNodeMetadataFromSmc gets nodes list from smartcontract
func GetNodeMetadataFromSmc(bc *base.BaseBlockChain, valIndices []int) ([]NodeMetadata, error) {
	util, err := permissioned.NewSmcPermissionUtil(*bc)
	if err != nil {
		return nil, err
	}
	nodes := make([]NodeMetadata, 0)
	for _, idx := range valIndices {
		// Get nodes by list of indices.
		// Note: this is used for dev environement only.
		pubString, _, listenAddr, votingPower, _, err := util.GetAdminNodeByIndex(int64(idx))
		if err != nil {
			return nil, err
		}
		n, err := NewNodeMetadata(nil, &pubString, votingPower.Uint64(), listenAddr)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	return nodes, nil
}

// GetNodeIndex returns the index of node based on last digits in string
func GetNodeIndex(nodeName string) (int, error) {
	reg, _ := regexp.Compile("[0-9]+\\z")
	return strconv.Atoi(reg.FindString(nodeName))
}
