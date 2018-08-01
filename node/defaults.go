package node

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/kardiachain/go-kardia/kai"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/nat"
)

const (
	DefaultHTTPHost = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 8545        // Default TCP port for the HTTP RPC server
	DefaultWSHost   = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort   = 8546        // Default TCP port for the websocket RPC server
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = kai.NodeConfig{
	DataDir: DefaultDataDir(),
	//HTTPPort: DefaultHTTPPort,
	//HTTPModules:      []string{"net", "web3"},
	//HTTPVirtualHosts: []string{"localhost"},
	//WSPort:           DefaultWSPort,
	//WSModules:        []string{"net", "web3"},
	P2P: p2p.Config{
		ListenAddr: ":30303",
		MaxPeers:   5,
		NAT:        nat.Any(),
	},
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		return filepath.Join(home, ".kardia")

		// TODO: may need to handle non-unix OS.
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
