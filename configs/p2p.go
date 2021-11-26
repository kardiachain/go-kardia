package configs

import (
	"crypto/ecdsa"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay
)

var (
	defaultAddrBookName = "addrbook.json"
)

//-----------------------------------------------------------------------------
// P2PConfig

// P2PConfig defines the configuration options for the peer-to-peer networking layer
type P2PConfig struct { //nolint: maligned
	// This field must be set to a valid secp256k1 private key.
	PrivateKey *ecdsa.PrivateKey `toml:"-"`
	RootDir    string            `mapstructure:"home"`

	// Address to listen for incoming connections
	ListenAddress string `mapstructure:"laddr"`

	// Address to advertise to peers for them to dial
	ExternalAddress string `mapstructure:"external_address"`

	// Comma separated list of seed nodes to connect to
	// We only use these if we can’t connect to peers in the addrbook
	Seeds []string `mapstructure:"seeds"`

	// Comma separated list of nodes to keep persistent connections to
	PersistentPeers string `mapstructure:"persistent_peers"`

	// UPNP port forwarding
	UPNP bool `mapstructure:"upnp"`

	// Path to address book
	AddrBook string `mapstructure:"addr_book_file"`

	// Set true for strict address routability rules
	// Set false for private or local networks
	AddrBookStrict bool `mapstructure:"addr_book_strict"`

	// Maximum number of inbound peers
	MaxNumInboundPeers int `mapstructure:"max_num_inbound_peers"`

	// Maximum number of outbound peers to connect to, excluding persistent peers
	MaxNumOutboundPeers int `mapstructure:"max_num_outbound_peers"`

	// List of node IDs, to which a connection will be (re)established ignoring any existing limits
	UnconditionalPeerIDs string `mapstructure:"unconditional_peer_ids"`

	// Maximum pause when redialing a persistent peer (if zero, exponential backoff is used)
	PersistentPeersMaxDialPeriod time.Duration `mapstructure:"persistent_peers_max_dial_period"`

	// Time to wait before flushing messages out on the connection
	FlushThrottleTimeout time.Duration `mapstructure:"flush_throttle_timeout"`

	// Maximum size of a message packet payload, in bytes
	MaxPacketMsgPayloadSize int `mapstructure:"max_packet_msg_payload_size"`

	// Rate at which packets can be sent, in bytes/second
	SendRate int64 `mapstructure:"send_rate"`

	// Rate at which packets can be received, in bytes/second
	RecvRate int64 `mapstructure:"recv_rate"`

	// Set true to enable the peer-exchange reactor
	PexReactor bool `mapstructure:"pex"`

	// Seed mode, in which node constantly crawls the network and looks for
	// peers. If another node asks it for addresses, it responds and disconnects.
	//
	// Does not work if the peer-exchange reactor is disabled.
	SeedMode bool `mapstructure:"seed_mode"`

	// Comma separated list of peer IDs to keep private (will not be gossiped to
	// other peers)
	PrivatePeerIDs string `mapstructure:"private_peer_ids"`

	// Toggle to disable guard against peers connecting from the same ip.
	AllowDuplicateIP bool `mapstructure:"allow_duplicate_ip"`

	// Peer connection configuration.
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	DialTimeout      time.Duration `mapstructure:"dial_timeout"`

	// Testing params.
	// Force dial to fail
	TestDialFail bool `mapstructure:"test_dial_fail"`
	// FUzz connection
	TestFuzz       bool            `mapstructure:"test_fuzz"`
	TestFuzzConfig *FuzzConnConfig `mapstructure:"test_fuzz_config"`
}

// DefaultP2PConfig returns a default configuration for the peer-to-peer layer
func DefaultP2PConfig() *P2PConfig {
	return &P2PConfig{
		ListenAddress:                "tcp://0.0.0.0:26656",
		ExternalAddress:              "",
		UPNP:                         false,
		AddrBook:                     filepath.Join(DefaultDataDir(), defaultAddrBookName),
		AddrBookStrict:               true,
		MaxNumInboundPeers:           40,
		MaxNumOutboundPeers:          15,
		PersistentPeersMaxDialPeriod: 0 * time.Second,
		FlushThrottleTimeout:         100 * time.Millisecond,
		// MTU (Maximum Transmission Unit) for Etherenet is 1500 bytes
		// IP header = 20 bytes, TCP header = 20 bytes
		// If RFC1323 timestamp is applied it could take 12 bytes more (https://www.ietf.org/rfc/rfc1323.txt)
		// Some other Etherenet using Jumbo (8kb) frames and token ring (4kb)
		// (https://en.wikipedia.org/wiki/Jumbo_frame, https://en.wikipedia.org/wiki/Token_Ring)
		// Some other technicals can reduce the payload such as tunneling or via gateway
		// So safe option should be around 1400 (correct me @lewtran)
		MaxPacketMsgPayloadSize: 1400,
		SendRate:                5120000, // 5 mB/s
		RecvRate:                5120000, // 5 mB/s
		PexReactor:              true,
		SeedMode:                false,
		AllowDuplicateIP:        false,
		HandshakeTimeout:        20 * time.Second,
		DialTimeout:             3 * time.Second,
		TestDialFail:            false,
		TestFuzz:                false,
		TestFuzzConfig:          DefaultFuzzConnConfig(),
	}
}

// AddrBookFile returns the full path to the address book
func (cfg *P2PConfig) AddrBookFile() string {
	return rootify(cfg.AddrBook, cfg.RootDir)
}

// Address return the main address
func (cfg *P2PConfig) Address() *common.Address {
	if address := crypto.PubkeyToAddress(cfg.PrivateKey.PublicKey); !address.Equal(common.Address{}) {
		return &address
	}
	return nil
}

// PrivKey return the main address's private key
func (cfg *P2PConfig) PrivKey() *ecdsa.PrivateKey {
	return cfg.PrivateKey
}

// FuzzConnConfig is a FuzzedConnection configuration.
type FuzzConnConfig struct {
	Mode         int
	MaxDelay     time.Duration
	ProbDropRW   float64
	ProbDropConn float64
	ProbSleep    float64
}

// DefaultFuzzConnConfig returns the default config.
func DefaultFuzzConnConfig() *FuzzConnConfig {
	return &FuzzConnConfig{
		Mode:         FuzzModeDrop,
		MaxDelay:     3 * time.Second,
		ProbDropRW:   0.2,
		ProbDropConn: 0.00,
		ProbSleep:    0.00,
	}
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "KardiaChain")
		case "windows":
			// We used to put everything in %HOME%\AppData\Roaming, but this caused
			// problems with non-typical setups. If this fallback location exists and
			// is non-empty, use it, otherwise DTRT and check %LOCALAPPDATA%.
			fallback := filepath.Join(home, "AppData", "Roaming", "KardiaChain")
			appdata := windowsAppData()
			if appdata == "" || isNonEmptyDir(fallback) {
				return fallback
			}
			return filepath.Join(appdata, "KardiaChain")
		default:
			return filepath.Join(home, ".kardia")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func windowsAppData() string {
	v := os.Getenv("LOCALAPPDATA")
	if v == "" {
		// Windows XP and below don't have LocalAppData. Crash here because
		// we don't support Windows XP and undefining the variable will cause
		// other issues.
		panic("environment variable LocalAppData is undefined")
	}
	return v
}

func isNonEmptyDir(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	names, _ := f.Readdir(1)
	f.Close()
	return len(names) > 0
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

//-----------------------------------------------------------------------------
// InstrumentationConfig

// InstrumentationConfig defines the configuration for metrics reporting.
type InstrumentationConfig struct {
	// When true, Prometheus metrics are served under /metrics on
	// PrometheusListenAddr.
	// Check out the documentation for the list of available metrics.
	Prometheus bool `mapstructure:"prometheus"`

	// Address to listen for Prometheus collector(s) connections.
	PrometheusListenAddr string `mapstructure:"prometheus_listen_addr"`

	// Maximum number of simultaneous connections.
	// If you want to accept a larger number than the default, make sure
	// you increase your OS limits.
	// 0 - unlimited.
	MaxOpenConnections int `mapstructure:"max_open_connections"`

	// Instrumentation namespace.
	Namespace string `mapstructure:"namespace"`
}

// DefaultInstrumentationConfig returns a default configuration for metrics
// reporting.
func DefaultInstrumentationConfig() *InstrumentationConfig {
	return &InstrumentationConfig{
		Prometheus:           false,
		PrometheusListenAddr: ":26660",
		MaxOpenConnections:   3,
		Namespace:            "kardia",
	}
}

// TestInstrumentationConfig returns a default configuration for metrics
// reporting.
func TestInstrumentationConfig() *InstrumentationConfig {
	return DefaultInstrumentationConfig()
}

// ValidateBasic performs basic validation (checking param bounds, etc.) and
// returns an error if any check fails.
func (cfg *InstrumentationConfig) ValidateBasic() error {
	if cfg.MaxOpenConnections < 0 {
		return errors.New("max_open_connections can't be negative")
	}
	return nil
}

//-----------------------------------------------------------------------------
// Utils

// helper function to make config creation independent of root dir
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
