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
	"bufio"
	"crypto/ecdsa"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/permissioned"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

const (
	datadirPrivateKey      = "nodekey"  // Path within the datadir to the node's private key
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

type MainChainConfig struct {
	// Mainchain

	// Index of validators
	ValidatorIndexes []int

	// ChainDataDir is directory that stores levelDB data
	ChainDataDir string

	// DbCache is size in MB of allocated levelDB cache, minimum 16
	DbCache int

	// DbHandles is number of allocated levelDB file handlers, minium 16
	DbHandles int

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

	// ======== DEV ENVIRONMENT CONFIG =========
	// Configuration of this environment when running in dev environment.
	EnvConfig *EnvironmentConfig
}

type DualChainConfig struct {
	// Dualchain

	ChainId uint64 // ID of dual chain unique to a dualnode group, such as for dual eth.

	// Index of validators
	ValidatorIndexes []int

	// ChainDataDir is directory that stores levelDB data
	ChainDataDir string

	// DbCache is size in MB of allocated levelDB cache, minimum 16
	DbCache int

	// DbHandles is number of allocated levelDB file handlers, minium 16
	DbHandles int

	// Genesis is genesis block which contain initial Block and accounts
	DualGenesis *genesis.Genesis

	// Dual's event pool options
	DualEventPool event_pool.EventPoolConfig

	// IsPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	// ======== DEV ENVIRONMENT CONFIG =========
	// Configuration of this environment when running in dev environment.
	EnvConfig *EnvironmentConfig

	// Dual Network ID
	DualNetworkID uint64
}

type NodeConfig struct {
	// Name sets the instance name of the node. It must not contain the / character and is
	// used in the devp2p node identifier. If no
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
	P2P p2p.Config

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
	// made against the  server with a malicious host domain.
	// Requests using ip address directly are not affected
	HTTPVirtualHosts []string `toml:",omitempty"`

	// HTTPModules is a list of API modules to expose via the HTTP RPC interface.
	// If the module list is empty, all RPC API endpoints designated public will be
	// exposed.
	HTTPModules []string `toml:",omitempty"`

	// KeyStoreDir is the file system folder that contains private keys. The directory can
	// be specified as a relative path, in which case it is resolved relative to the
	// current directory.
	//
	// If KeyStoreDir is empty, the default location is the "keystore" subdirectory of
	// DataDir. If DataDir is unspecified and KeyStoreDir is empty, an ephemeral directory
	// is created by New and destroyed when the node is stopped.
	KeyStoreDir string `toml:",omitempty"`

	// Configuration of the Kardia's blockchain (or main chain).
	MainChainConfig MainChainConfig

	// Configuration of the dual's blockchain.
	DualChainConfig DualChainConfig

	// PeerProxyIP is IP of the network peer proxy, when participates in network with peer proxy for discovery.
	PeerProxyIP string

	// ======== DEV ENVIRONMENT CONFIG =========
	// Configuration of this node when running in dev environment.
	NodeMetadata *NodeMetadata
}

// NodeMetadata contains privateKey and votingPower and function that get coinbase
type NodeMetadata struct {
	PrivKey     *ecdsa.PrivateKey
	PublicKey   *ecdsa.PublicKey
	VotingPower int64
	ListenAddr  string
}

// EnvironmentConfig contains a list of NodeVotingPower, proposalIndex and votingStrategy
type EnvironmentConfig struct {
	NodeSet        []NodeMetadata
	proposalIndex  int
	VotingStrategy map[consensus.VoteTurn]int
}

// NodeName returns the devp2p node identifier.
func (c *NodeConfig) NodeName() string {
	// TODO: add version & OS to name
	return c.name()
}

// NodeKey retrieves the configured private key of the node,
// first any manually set key, falling back to the one found in the configured
// data folder. If no key can be found, a new one is generated.
func (c *NodeConfig) NodeKey() *ecdsa.PrivateKey {
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

	keyfile := c.resolvePath(datadirPrivateKey)
	if key, err := crypto.LoadECDSA(keyfile); err == nil {
		return key
	}

	// No persistent key found, generate and store a new one.
	var key *ecdsa.PrivateKey
	if c.NodeMetadata != nil {
		// Load dev node key if running in dev environment.
		key = c.NodeMetadata.PrivKey
	} else {
		k, err := crypto.GenerateKey()
		if err != nil {
			log.Crit(fmt.Sprintf("Failed to generate node key: %v", err))
		}
		key = k
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

// Database starts a new or existed database in the node data directory, or in-memory database.
func (c *NodeConfig) StartDatabase(name string, cache int, handles int) (storage.Database, error) {
	if c.DataDir == "" {
		return storage.NewMemStore(), nil
	}
	return storage.NewLDBStore(c.resolvePath(name), cache, handles)
}

// Return saved name or executable file name.
func (c *NodeConfig) name() string {
	if c.Name == "" {
		progname := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if progname == "" {
			panic("empty executable name, set Config.Name")
		}
		return progname
	}
	return c.Name
}

// and port parameters.
func (c *NodeConfig) HTTPEndpoint() string {
	if c.HTTPHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.HTTPHost, c.HTTPPort)
}

// DefaultHTTPEndpoint returns the HTTP endpoint used by default.
func DefaultHTTPEndpoint() string {
	config := &NodeConfig{HTTPHost: DefaultHTTPHost, HTTPPort: DefaultHTTPPort}
	return config.HTTPEndpoint()
}

func (c *NodeConfig) instanceDir() string {
	if c.DataDir == "" {
		return ""
	}
	return filepath.Join(c.DataDir, c.name())
}

// Resolves path in the instance directory.
func (c *NodeConfig) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if c.DataDir == "" {
		return ""
	}
	return filepath.Join(c.instanceDir(), path)
}

// GetNodeIndex returns the index of node based on last digits in string
func GetNodeIndex(nodeName string) (int, error) {
	reg, _ := regexp.Compile("[0-9]+\\z")
	return strconv.Atoi(reg.FindString(nodeName))
}

// NewNodeMetadata init new NodeMetadata
func NewNodeMetadata(privateKey *string, publicKey *string, votingPower int64, listenAddr string) (*NodeMetadata, error) {

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

// NodeID returns enodeId
func (n *NodeMetadata) NodeID() string {
	return fmt.Sprintf(
		"enode://%s@%s",
		hex.EncodeToString(n.PublicKey.X.Bytes())+hex.EncodeToString(n.PublicKey.Y.Bytes()),
		n.ListenAddr)
}

// Coinbase returns address of a node
func (n *NodeMetadata) Coinbase() common.Address {
	return crypto.PubkeyToAddress(n.PrivKey.PublicKey)
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
		n, err := NewNodeMetadata(nil, &pubString, votingPower.Int64(), listenAddr)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	return nodes, nil
}

// NewEnvironmentConfig returns new EnvironmentConfig instance
func NewEnvironmentConfig() *EnvironmentConfig {
	var env EnvironmentConfig
	env.proposalIndex = 0 // Default to 0-th node as the proposer.
	env.NodeSet = make([]NodeMetadata, 0)
	return &env
}

// GetNodeSize returns size of NodeSet
func (env *EnvironmentConfig) GetNodeSize() int {
	return len(env.NodeSet)
}

// SetVotingStrategy is used for testing voting
func (env *EnvironmentConfig) SetVotingStrategy(votingStrategy string) {
	if strings.HasSuffix(votingStrategy, "csv") {
		env.VotingStrategy = map[consensus.VoteTurn]int{}
		csvFile, _ := os.Open(votingStrategy)
		reader := csv.NewReader(bufio.NewReader(csvFile))

		for {
			line, err := reader.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Error("error", err)
			}
			var height, _ = strconv.Atoi(line[0])
			var round, _ = strconv.Atoi(line[1])
			var voteType, _ = strconv.Atoi(line[2])
			var result, _ = strconv.Atoi(line[3])

			var _, ok = env.GetScriptedVote(height, round, voteType)
			if ok {
				log.Error(fmt.Sprintf("VoteTurn already exists with height = %v, round = %v, voteType = %v", height, round, voteType))
			} else {
				env.VotingStrategy[consensus.VoteTurn{height, round, voteType}] = result
			}
		}
	}
}

func (env *EnvironmentConfig) GetScriptedVote(height int, round int, voteType int) (int, bool) {
	if val, ok := env.VotingStrategy[consensus.VoteTurn{height, round, voteType}]; ok {
		return val, ok
	}
	return 0, false
}

func (env *EnvironmentConfig) SetProposerIndex(index, limit int) {
	if index < 0 || index >= limit {
		log.Error(fmt.Sprintf("Proposer index must be within %v and %v", 0, env.GetNodeSize()))
	}
	env.proposalIndex = index
}

func (env *EnvironmentConfig) GetNodeMetadata(index int) *NodeMetadata {
	return &env.NodeSet[index]
}

// GetValidatorSetByIndices takes an array of indexes of validators and returns an array of validators with the order respectively to index of input
func (env *EnvironmentConfig) GetValidatorSetByIndices(bc base.BaseBlockChain, valIndexes []int) (*types.ValidatorSet, error) {
	// If NodeSet is empty then get nodes from smc
	if env.GetNodeSize() == 0 {
		nodes, err := GetNodeMetadataFromSmc(&bc, valIndexes)
		if err != nil {
			return nil, err
		}
		env.NodeSet = nodes
	}
	if len(valIndexes) > env.GetNodeSize() {
		return nil, fmt.Errorf("number of validators must be within %v and %v", 1, env.GetNodeSize())
	}
	validators := make([]*types.Validator, 0)
	for i := 0; i < len(valIndexes); i++ {
		if valIndexes[i] < 0 {
			return nil, fmt.Errorf("value of validator must be greater than 0")
		}
		node := env.NodeSet[i]
		validators = append(validators, types.NewValidator(*node.PublicKey, node.VotingPower))
	}

	validatorSet := types.NewValidatorSet(validators)
	validatorSet.TurnOnKeepSameProposer()
	validatorSet.SetProposer(validators[env.proposalIndex])
	return validatorSet, nil
}
