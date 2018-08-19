package node

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/storage"
)

const (
	datadirPrivateKey      = "nodekey"  // Path within the datadir to the node's private key
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

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
	// made against the server with a malicious host domain.
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

	// ======== DEV ENVIRONMENT CONFIG =========
	// Additional config of this node when running in dev environment.
	DevNodeConfig *dev.DevNodeConfig
	// Additional config of this environment when running as dev.
	DevEnvConfig *dev.DevEnvironmentConfig
	// Number of validators.
	NumValidators int

	// ChainData is directory that stores levelDB data
	ChainData string

	// DbCache is a param used to start levelDB
	DbCache int

	// DbHandles is a param used to start levelDB
	DbHandles int

	// Genesis is genesis block which contain initial Block and accounts
	Genesis *blockchain.Genesis
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
	if c.DevNodeConfig != nil {
		// Load dev node key if running in dev environment.
		key = c.DevNodeConfig.PrivKey
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
