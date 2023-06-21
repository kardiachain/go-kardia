// Copyright 2015 The go-kardia Authors
// This file is part of go-kardia.
//
// go-kardia is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-kardia is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-kardia. If not, see <http://www.gnu.org/licenses/>.

// Package utils contains internal helper functions for go-kardia commands.
package utils

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path/filepath"
	godebug "runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/internal/flags"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"
	"github.com/kardiachain/go-kardia/lib/metrics/exp"
	kai "github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/node"
	gopsutil "github.com/shirou/gopsutil/mem"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.

var (
	// General settings
	GenesisFlag = &cli.StringFlag{
		Name:     "genesis",
		Usage:    "Custom genesis file for dev network",
		Category: flags.KaiCategory,
	}
	DataDirFlag = &flags.DirectoryFlag{
		Name:     "datadir",
		Usage:    "Data directory for the databases and keystore",
		Value:    flags.DirectoryString(configs.DefaultDataDir()),
		Category: flags.KaiCategory,
	}
	KeyStoreDirFlag = &flags.DirectoryFlag{
		Name:     "keystore",
		Usage:    "Directory for the keystore (default = inside the datadir)",
		Category: flags.AccountCategory,
	}
	NetworkIdFlag = &cli.Uint64Flag{
		Name:     "networkid",
		Usage:    "Explicitly set network id (integer)(default = 24)",
		Value:    kai.Defaults.NetworkId,
		Category: flags.KaiCategory,
	}
	MainnetFlag = &cli.BoolFlag{
		Name:     "mainnet",
		Usage:    "Kardiachain mainnet",
		Category: flags.KaiCategory,
	}
	TestnetFlag = &cli.BoolFlag{
		Name:     "testnet",
		Usage:    "Test network: pre-configured proof-of-stake test network",
		Category: flags.KaiCategory,
	}

	GCModeFlag = &cli.StringFlag{
		Name:     "gcmode",
		Usage:    `Blockchain garbage collection mode ("full", "archive")`,
		Value:    "full",
		Category: flags.KaiCategory,
	}
	SnapshotFlag = &cli.BoolFlag{
		Name:     "snapshot",
		Usage:    `Enables snapshot-database mode (default = enable)`,
		Value:    true,
		Category: flags.KaiCategory,
	}
	BloomFilterSizeFlag = &cli.Uint64Flag{
		Name:     "bloomfilter.size",
		Usage:    "Megabytes of memory allocated to bloom-filter for pruning",
		Value:    2048,
		Category: flags.KaiCategory,
	}
	TxLookupLimitFlag = &cli.Uint64Flag{
		Name:     "txlookuplimit",
		Usage:    "Number of recent blocks to maintain transactions index for (default = about one year, 0 = entire chain)",
		Value:    kai.Defaults.TxLookupLimit,
		Category: flags.KaiCategory,
	}

	// Performance tuning settings
	CacheFlag = &cli.IntFlag{
		Name:     "cache",
		Usage:    "Megabytes of memory allocated to internal caching (default = 4096 mainnet full node, 128 light mode)",
		Value:    1024,
		Category: flags.PerfCategory,
	}
	CacheDatabaseFlag = &cli.IntFlag{
		Name:     "cache.database",
		Usage:    "Percentage of cache memory allowance to use for database io",
		Value:    50,
		Category: flags.PerfCategory,
	}
	CacheTrieFlag = &cli.IntFlag{
		Name:     "cache.trie",
		Usage:    "Percentage of cache memory allowance to use for trie caching (default = 15% full mode, 30% archive mode)",
		Value:    15,
		Category: flags.PerfCategory,
	}
	CacheTrieJournalFlag = &cli.StringFlag{
		Name:     "cache.trie.journal",
		Usage:    "Disk journal directory for trie cache to survive node restarts",
		Value:    kai.Defaults.TrieCleanCacheJournal,
		Category: flags.PerfCategory,
	}
	CacheTrieRejournalFlag = &cli.DurationFlag{
		Name:     "cache.trie.rejournal",
		Usage:    "Time interval to regenerate the trie cache journal",
		Value:    kai.Defaults.TrieCleanCacheRejournal,
		Category: flags.PerfCategory,
	}
	CacheGCFlag = &cli.IntFlag{
		Name:     "cache.gc",
		Usage:    "Percentage of cache memory allowance to use for trie pruning (default = 25% full mode, 0% archive mode)",
		Value:    25,
		Category: flags.PerfCategory,
	}
	CacheSnapshotFlag = &cli.IntFlag{
		Name:     "cache.snapshot",
		Usage:    "Percentage of cache memory allowance to use for snapshot caching (default = 10% full mode, 20% archive mode)",
		Value:    10,
		Category: flags.PerfCategory,
	}
	CacheNoPrefetchFlag = &cli.BoolFlag{
		Name:     "cache.noprefetch",
		Usage:    "Disable heuristic state prefetch during block import (less CPU and disk IO, more time waiting for data)",
		Category: flags.PerfCategory,
	}
	CachePreimagesFlag = &cli.BoolFlag{
		Name:     "cache.preimages",
		Usage:    "Enable recording the SHA3/keccak preimages of trie keys",
		Category: flags.PerfCategory,
	}

	// RPC settings
	IPCDisabledFlag = &cli.BoolFlag{
		Name:     "ipcdisable",
		Usage:    "Disable the IPC-RPC server",
		Category: flags.APICategory,
	}
	IPCPathFlag = &flags.DirectoryFlag{
		Name:     "ipcpath",
		Usage:    "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
		Category: flags.APICategory,
	}
	HTTPEnabledFlag = &cli.BoolFlag{
		Name:     "http",
		Usage:    "Enable the HTTP-RPC server",
		Category: flags.APICategory,
	}
	HTTPListenAddrFlag = &cli.StringFlag{
		Name:     "http.addr",
		Usage:    "HTTP-RPC server listening interface",
		Value:    node.DefaultHTTPHost,
		Category: flags.APICategory,
	}
	HTTPPortFlag = &cli.IntFlag{
		Name:     "http.port",
		Usage:    "HTTP-RPC server listening port",
		Value:    node.DefaultHTTPPort,
		Category: flags.APICategory,
	}
	HTTPCORSDomainFlag = &cli.StringFlag{
		Name:     "http.corsdomain",
		Usage:    "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value:    "",
		Category: flags.APICategory,
	}
	HTTPVirtualHostsFlag = &cli.StringFlag{
		Name:     "http.vhosts",
		Usage:    "Comma separated list of virtual hostnames from which to accept requests (server enforced). Accepts '*' wildcard.",
		Value:    strings.Join(node.DefaultConfig.HTTPVirtualHosts, ","),
		Category: flags.APICategory,
	}
	HTTPApiFlag = &cli.StringFlag{
		Name:     "http.api",
		Usage:    "API's offered over the HTTP-RPC interface",
		Value:    "",
		Category: flags.APICategory,
	}
	WSEnabledFlag = &cli.BoolFlag{
		Name:     "ws",
		Usage:    "Enable the WS-RPC server",
		Category: flags.APICategory,
	}
	WSListenAddrFlag = &cli.StringFlag{
		Name:     "ws.addr",
		Usage:    "WS-RPC server listening interface",
		Value:    node.DefaultWSHost,
		Category: flags.APICategory,
	}
	WSPortFlag = &cli.IntFlag{
		Name:     "ws.port",
		Usage:    "WS-RPC server listening port",
		Value:    node.DefaultWSPort,
		Category: flags.APICategory,
	}
	WSApiFlag = &cli.StringFlag{
		Name:     "ws.api",
		Usage:    "API's offered over the WS-RPC interface",
		Value:    "",
		Category: flags.APICategory,
	}
	WSAllowedOriginsFlag = &cli.StringFlag{
		Name:     "ws.origins",
		Usage:    "Origins from which to accept websockets requests",
		Value:    "",
		Category: flags.APICategory,
	}

	// Metrics flags
	MetricsEnabledFlag = &cli.BoolFlag{
		Name:     "metrics",
		Usage:    "Enable metrics collection and reporting",
		Category: flags.MetricsCategory,
	}
	MetricsEnabledExpensiveFlag = &cli.BoolFlag{
		Name:     "metrics.expensive",
		Usage:    "Enable expensive metrics collection and reporting",
		Category: flags.MetricsCategory,
	}
	// MetricsHTTPFlag defines the endpoint for a stand-alone metrics HTTP endpoint.
	// Since the pprof service enables sensitive/vulnerable behavior, this allows a user
	// to enable a public-OK metrics endpoint without having to worry about ALSO exposing
	// other profiling behavior or information.
	MetricsHTTPFlag = &cli.StringFlag{
		Name:     "metrics.addr",
		Usage:    `Enable stand-alone metrics HTTP server listening interface.`,
		Category: flags.MetricsCategory,
	}
	MetricsPortFlag = &cli.IntFlag{
		Name: "metrics.port",
		Usage: `Metrics HTTP server listening port.
Please note that --` + MetricsHTTPFlag.Name + ` must be set to start the server.`,
		Value:    metrics.DefaultConfig.Port,
		Category: flags.MetricsCategory,
	}
)

var (
	// NetworkFlags is the flag group of all built-in supported networks.
	NetworkFlags = []cli.Flag{MainnetFlag, TestnetFlag}

	// DatabasePathFlags is the flag group of all database path flags.
	DatabasePathFlags = []cli.Flag{
		DataDirFlag,
	}
)

// MakeDataDir retrieves the currently requested data directory, terminating
// if none (or the empty string) is specified. If the node is starting a testnet,
// then a subdirectory of the specified datadir will be used.
func MakeDataDir(ctx *cli.Context) string {
	if path := ctx.String(DataDirFlag.Name); path != "" {
		if ctx.Bool(TestnetFlag.Name) {
			return filepath.Join(path, "testnet")
		}
		return path
	}
	Fatalf("Cannot determine default data directory, please set manually (--datadir)")
	return ""
}

// SplitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func SplitAndTrim(input string) (ret []string) {
	l := strings.Split(input, ",")
	for _, r := range l {
		if r = strings.TrimSpace(r); r != "" {
			ret = append(ret, r)
		}
	}
	return ret
}

// setHTTP creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setHTTP(ctx *cli.Context, cfg *node.Config) {
	if ctx.Bool(HTTPEnabledFlag.Name) && cfg.HTTPHost == "" {
		cfg.HTTPHost = "127.0.0.1"
		if ctx.IsSet(HTTPListenAddrFlag.Name) {
			cfg.HTTPHost = ctx.String(HTTPListenAddrFlag.Name)
		}
	}

	if ctx.IsSet(HTTPPortFlag.Name) {
		cfg.HTTPPort = ctx.Int(HTTPPortFlag.Name)
	}

	if ctx.IsSet(HTTPCORSDomainFlag.Name) {
		cfg.HTTPCors = SplitAndTrim(ctx.String(HTTPCORSDomainFlag.Name))
	}

	if ctx.IsSet(HTTPApiFlag.Name) {
		cfg.HTTPModules = SplitAndTrim(ctx.String(HTTPApiFlag.Name))
	}

	if ctx.IsSet(HTTPVirtualHostsFlag.Name) {
		cfg.HTTPVirtualHosts = SplitAndTrim(ctx.String(HTTPVirtualHostsFlag.Name))
	}
}

// setWS creates the WebSocket RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setWS(ctx *cli.Context, cfg *node.Config) {
	if ctx.Bool(WSEnabledFlag.Name) && cfg.WSHost == "" {
		cfg.WSHost = "127.0.0.1"
		if ctx.IsSet(WSListenAddrFlag.Name) {
			cfg.WSHost = ctx.String(WSListenAddrFlag.Name)
		}
	}
	if ctx.IsSet(WSPortFlag.Name) {
		cfg.WSPort = ctx.Int(WSPortFlag.Name)
	}

	if ctx.IsSet(WSAllowedOriginsFlag.Name) {
		cfg.WSOrigins = SplitAndTrim(ctx.String(WSAllowedOriginsFlag.Name))
	}

	if ctx.IsSet(WSApiFlag.Name) {
		cfg.WSModules = SplitAndTrim(ctx.String(WSApiFlag.Name))
	}
}

// setIPC creates an IPC path configuration from the set command line flags,
// returning an empty string if IPC was explicitly disabled, or the set path.
func setIPC(ctx *cli.Context, cfg *node.Config) {
	CheckExclusive(ctx, IPCDisabledFlag, IPCPathFlag)
	switch {
	case ctx.Bool(IPCDisabledFlag.Name):
		cfg.IPCPath = ""
	case ctx.IsSet(IPCPathFlag.Name):
		cfg.IPCPath = ctx.String(IPCPathFlag.Name)
	}
}

func setGenesis(ctx *cli.Context, cfg *kai.Config) {
	if !ctx.IsSet(GenesisFlag.Name) {
		Fatalf(`Genesis file must be specified.`)
	}

	wd, err := os.Getwd()
	if err != nil {
		Fatalf(`Cannot open working directory {}`, err)
	}

	genesisCfgFile := filepath.Join(wd, ctx.String(GenesisFlag.Name))
	genesisCfgRaw, err := ioutil.ReadFile(genesisCfgFile)
	if err != nil {
		Fatalf(`Cannot read genesis file`, err)
	}
	var ch = &Chain{}
	err = yaml.Unmarshal(genesisCfgRaw, ch)
	if err != nil {
		Fatalf(`Cannot unmarshal node config`, err)
	}

	var g = ch.Genesis
	var ga genesis.GenesisAlloc

	genesisAccounts := make(map[string]*big.Int)
	genesisContracts := make(map[string]string)

	for _, account := range g.Accounts {
		amount, ok := new(big.Int).SetString(account.Amount, 10)
		if !ok {
			Fatalf("Cannot convert genesis amount")
		}
		genesisAccounts[account.Address] = amount
	}

	for key, contract := range g.Contracts {
		configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
		if key != configs.StakingContractKey && contract.Address != "" {
			genesisContracts[contract.Address] = contract.ByteCode
		}
	}
	ga, err = genesis.GenesisAllocFromAccountAndContract(genesisAccounts, genesisContracts)
	if err != nil {
		Fatalf("{}", err)
	}

	var chainConfig *configs.ChainConfig
	switch {
	case ctx.IsSet(MainnetFlag.Name):
		chainConfig = configs.MainnetChainConfig
	case ctx.IsSet(TestnetFlag.Name):
		chainConfig = configs.TestnetChainConfig
	}

	cfg.Genesis = &genesis.Genesis{
		ChainID:         ctx.String(NetworkIdFlag.Name),
		Config:          chainConfig,
		InitialHeight:   1,
		Alloc:           ga,
		Validators:      g.Validators,
		ConsensusParams: configs.DefaultConsensusParams(),
		Consensus:       configs.DefaultConsensusConfig(),
		Timestamp:       time.Unix(g.Timestamp, 0),
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func SetNodeConfig(ctx *cli.Context, cfg *node.Config) {
	setIPC(ctx, cfg)
	setHTTP(ctx, cfg)
	setWS(ctx, cfg)
	SetDataDir(ctx, cfg)

	if ctx.IsSet(KeyStoreDirFlag.Name) {
		cfg.KeyStoreDir = ctx.String(KeyStoreDirFlag.Name)
	}
}

func SetDataDir(ctx *cli.Context, cfg *node.Config) {
	switch {
	case ctx.IsSet(DataDirFlag.Name):
		cfg.DataDir = ctx.String(DataDirFlag.Name)
	case ctx.Bool(TestnetFlag.Name) && cfg.DataDir == configs.DefaultDataDir():
		cfg.DataDir = filepath.Join(configs.DefaultDataDir(), "testnet")
	}
}

// CheckExclusive verifies that only a single instance of the provided flags was
// set by the user. Each flag might optionally be followed by a string type to
// specialize it further.
func CheckExclusive(ctx *cli.Context, args ...interface{}) {
	set := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		// Make sure the next argument is a flag and skip if not set
		flag, ok := args[i].(cli.Flag)
		if !ok {
			panic(fmt.Sprintf("invalid argument, not cli.Flag type: %T", args[i]))
		}
		// Check if next arg extends current and expand its name if so
		name := flag.Names()[0]

		if i+1 < len(args) {
			switch option := args[i+1].(type) {
			case string:
				// Extended flag check, make sure value set doesn't conflict with passed in option
				if ctx.String(flag.Names()[0]) == option {
					name += "=" + option
					set = append(set, "--"+name)
				}
				// shift arguments and continue
				i++
				continue

			case cli.Flag:
			default:
				panic(fmt.Sprintf("invalid argument, not cli.Flag or string extension: %T", args[i+1]))
			}
		}
		// Mark the flag if it's set
		if ctx.IsSet(flag.Names()[0]) {
			set = append(set, "--"+name)
		}
	}
	if len(set) > 1 {
		Fatalf("Flags %v can't be used at the same time", strings.Join(set, ", "))
	}
}

// SetKaiConfig applies kai-related command line flags to the config.
func SetKaiConfig(ctx *cli.Context, stack *node.Node, cfg *kai.Config) {
	// Set genesis
	setGenesis(ctx, cfg)

	// Avoid conflicting network flags
	CheckExclusive(ctx, MainnetFlag, TestnetFlag)

	// Disable transaction unindexing for archive node
	if ctx.String(GCModeFlag.Name) == "archive" && ctx.Uint64(TxLookupLimitFlag.Name) != 0 {
		ctx.Set(TxLookupLimitFlag.Name, "0")
		log.Warn("Disable transaction unindexing for archive node")
	}

	// Cap the cache allowance and tune the garbage collector
	mem, err := gopsutil.VirtualMemory()
	if err == nil {
		if 32<<(^uintptr(0)>>63) == 32 && mem.Total > 2*1024*1024*1024 {
			log.Warn("Lowering memory allowance on 32bit arch", "available", mem.Total/1024/1024, "addressable", 2*1024)
			mem.Total = 2 * 1024 * 1024 * 1024
		}
		allowance := int(mem.Total / 1024 / 1024 / 3)
		if cache := ctx.Int(CacheFlag.Name); cache > allowance {
			log.Warn("Sanitizing cache to Go's GC limits", "provided", cache, "updated", allowance)
			ctx.Set(CacheFlag.Name, strconv.Itoa(allowance))
		}
	}
	// Ensure Go's GC ignores the database cache for trigger percentage
	cache := ctx.Int(CacheFlag.Name)
	gogc := math.Max(20, math.Min(100, 100/(float64(cache)/1024)))

	log.Debug("Sanitizing Go's GC trigger", "percent", int(gogc))
	godebug.SetGCPercent(int(gogc))

	if ctx.IsSet(NetworkIdFlag.Name) {
		cfg.NetworkId = ctx.Uint64(NetworkIdFlag.Name)
	}
	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheDatabaseFlag.Name) {
		cfg.DatabaseCache = ctx.Int(CacheFlag.Name) * ctx.Int(CacheDatabaseFlag.Name) / 100
	}

	if gcmode := ctx.String(GCModeFlag.Name); gcmode != "full" && gcmode != "archive" {
		Fatalf("--%s must be either 'full' or 'archive'", GCModeFlag.Name)
	}
	if ctx.IsSet(GCModeFlag.Name) {
		cfg.NoPruning = ctx.String(GCModeFlag.Name) == "archive"
	}
	if ctx.IsSet(CacheNoPrefetchFlag.Name) {
		cfg.NoPrefetch = ctx.Bool(CacheNoPrefetchFlag.Name)
	}
	// Read the value from the flag no matter if it's set or not.
	cfg.Preimages = ctx.Bool(CachePreimagesFlag.Name)
	if cfg.NoPruning && !cfg.Preimages {
		cfg.Preimages = true
		log.Info("Enabling recording of key preimages since archive mode is used")
	}
	if ctx.IsSet(TxLookupLimitFlag.Name) {
		cfg.TxLookupLimit = ctx.Uint64(TxLookupLimitFlag.Name)
	}
	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheTrieFlag.Name) {
		cfg.TrieCleanCache = ctx.Int(CacheFlag.Name) * ctx.Int(CacheTrieFlag.Name) / 100
	}
	if ctx.IsSet(CacheTrieJournalFlag.Name) {
		cfg.TrieCleanCacheJournal = ctx.String(CacheTrieJournalFlag.Name)
	}
	if ctx.IsSet(CacheTrieRejournalFlag.Name) {
		cfg.TrieCleanCacheRejournal = ctx.Duration(CacheTrieRejournalFlag.Name)
	}
	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheGCFlag.Name) {
		cfg.TrieDirtyCache = ctx.Int(CacheFlag.Name) * ctx.Int(CacheGCFlag.Name) / 100
	}
	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheSnapshotFlag.Name) {
		cfg.SnapshotCache = ctx.Int(CacheFlag.Name) * ctx.Int(CacheSnapshotFlag.Name) / 100
	}

	switch {
	case ctx.IsSet(MainnetFlag.Name):
		cfg.FastSync = configs.DefaultFastSyncConfig()
		cfg.Consensus = configs.DefaultConsensusConfig()
	case ctx.IsSet(TestnetFlag.Name):
		cfg.FastSync = configs.TestFastSyncConfig()
		cfg.Consensus = configs.TestConsensusConfig()
	}

	// Gas oracle
	cfg.GasOracle = oracles.DefaultOracleConfig()
}

// RegisterKaiService adds an Kardiachain client to the stack.
// The second return value is the full node instance, which may be nil if the
// node is running as a light client.
func RegisterKaiService(stack *node.Node, cfg *kai.Config) (kaiapi.Backend, *kai.Kardiachain) {
	backend, err := kai.New(stack, cfg)
	if err != nil {
		Fatalf("Failed to register the Kardiachain service: %v", err)
	}

	return backend.APIBackend, backend
}

func SetupMetrics(ctx *cli.Context) {
	if metrics.Enabled {
		log.Info("Enabling metrics collection")

		if ctx.IsSet(MetricsHTTPFlag.Name) {
			address := fmt.Sprintf("%s:%d", ctx.String(MetricsHTTPFlag.Name), ctx.Int(MetricsPortFlag.Name))
			log.Info("Enabling stand-alone metrics HTTP endpoint", "address", address)
			exp.Setup(address)
		} else if ctx.IsSet(MetricsPortFlag.Name) {
			log.Warn(fmt.Sprintf("--%s specified without --%s, metrics server will not start.", MetricsPortFlag.Name, MetricsHTTPFlag.Name))
		}
	}
}

func SplitTagsFlag(tagsFlag string) map[string]string {
	tags := strings.Split(tagsFlag, ",")
	tagsMap := map[string]string{}

	for _, t := range tags {
		if t != "" {
			kv := strings.Split(t, "=")

			if len(kv) == 2 {
				tagsMap[kv[0]] = kv[1]
			}
		}
	}

	return tagsMap
}

// MakeChainDatabase open an LevelDB using the flags passed to the client and will hard crash if it fails.
func MakeChainDatabase(ctx *cli.Context, stack *node.Node, readonly bool) kaidb.Database {
	storeDB, err := stack.OpenDatabase("chaindata", 16, 32, "chaindata")
	if err != nil {
		Fatalf("Could not open database: %v", err)
	}
	return storeDB.DB()
}

func IsNetworkPreset(ctx *cli.Context) bool {
	for _, flag := range NetworkFlags {
		bFlag, _ := flag.(*cli.BoolFlag)
		if ctx.IsSet(bFlag.Name) {
			return true
		}
	}
	return false
}

func MakeGenesis(ctx *cli.Context) *genesis.Genesis {
	var gs *genesis.Genesis
	switch {
	case ctx.Bool(MainnetFlag.Name):
		gs = genesis.DefaultGenesisBlock()
	default:
		Fatalf("Developer chains are ephemeral")
	}
	return gs
}

// MakeChain creates a chain manager from set command line flags.
func MakeChain(ctx *cli.Context, stack *node.Node, readonly bool) (*blockchain.BlockChain, kaidb.Database) {
	var (
		gspec   = MakeGenesis(ctx)
		chainDb = MakeChainDatabase(ctx, stack, readonly)
	)
	if gcmode := ctx.String(GCModeFlag.Name); gcmode != "full" && gcmode != "archive" {
		Fatalf("--%s must be either 'full' or 'archive'", GCModeFlag.Name)
	}
	cache := &blockchain.CacheConfig{
		TrieCleanLimit:      kai.Defaults.TrieCleanCache,
		TrieCleanNoPrefetch: ctx.Bool(CacheNoPrefetchFlag.Name),
		TrieDirtyLimit:      kai.Defaults.TrieDirtyCache,
		TrieDirtyDisabled:   ctx.String(GCModeFlag.Name) == "archive",
		TrieTimeLimit:       kai.Defaults.TrieTimeout,
		SnapshotLimit:       kai.Defaults.SnapshotCache,
		Preimages:           ctx.Bool(CachePreimagesFlag.Name),
	}
	if cache.TrieDirtyDisabled && !cache.Preimages {
		cache.Preimages = true
		log.Info("Enabling recording of key preimages since archive mode is used")
	}
	if !ctx.Bool(SnapshotFlag.Name) {
		cache.SnapshotLimit = 0 // Disabled
	}
	// If we're in readonly, do not bother generating snapshot data.
	if readonly {
		cache.SnapshotNoBuild = true
	}

	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheTrieFlag.Name) {
		cache.TrieCleanLimit = ctx.Int(CacheFlag.Name) * ctx.Int(CacheTrieFlag.Name) / 100
	}
	if ctx.IsSet(CacheFlag.Name) || ctx.IsSet(CacheGCFlag.Name) {
		cache.TrieDirtyLimit = ctx.Int(CacheFlag.Name) * ctx.Int(CacheGCFlag.Name) / 100
	}

	chain, err := blockchain.NewBlockChain(chainDb, cache, gspec)
	if err != nil {
		Fatalf("Can't create BlockChain: %v", err)
	}
	return chain, chainDb
}
