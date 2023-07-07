// Copyright 2014 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

// Kaigo is the official command-line client for Ethereum.
package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/kardiachain/go-kardia/cmd/utils"
	"github.com/kardiachain/go-kardia/internal/debug"
	"github.com/kardiachain/go-kardia/internal/flags"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"

	"github.com/urfave/cli/v2"
)

const (
	clientIdentifier = "kaigo" // Client identifier to advertise over the network
)

var (
	// flags that configure the node
	nodeFlags = flags.Merge([]cli.Flag{
		utils.GenesisFlag,
		utils.KeyStoreDirFlag,
		utils.GCModeFlag,
		utils.SnapshotFlag,
		utils.TxLookupLimitFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheTrieFlag,
		utils.CacheTrieJournalFlag,
		utils.CacheTrieRejournalFlag,
		utils.CacheGCFlag,
		utils.CacheSnapshotFlag,
		utils.CacheNoPrefetchFlag,
		utils.CachePreimagesFlag,
		utils.NetworkIdFlag,
		configFileFlag,
	}, utils.NetworkFlags, utils.DatabasePathFlags)

	rpcFlags = []cli.Flag{
		utils.HTTPEnabledFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPPortFlag,
		utils.HTTPCORSDomainFlag,
		utils.HTTPVirtualHostsFlag,
		utils.HTTPApiFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
	}

	metricsFlags = []cli.Flag{
		utils.MetricsEnabledFlag,
		utils.MetricsEnabledExpensiveFlag,
		utils.MetricsHTTPFlag,
		utils.MetricsPortFlag,
	}
)

var app = flags.NewApp("the go-kardia command line interface")

func init() {
	// Initialize the CLI app and start Kaigo
	app.Action = kaigo
	app.Copyright = "Copyright 2013-2023 The go-kardia Authors"
	app.Commands = []*cli.Command{
		// See snapshot.go
		snapshotCommand,
		dumpConfigCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = flags.Merge(
		nodeFlags,
		rpcFlags,
		debug.Flags,
		metricsFlags,
	)

	app.Before = func(ctx *cli.Context) error {
		flags.MigrateGlobalFlags(ctx)
		return debug.Setup(ctx)
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		prompt.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prepare manipulates memory cache allowance and setups metric system.
// This function should be called before launching devp2p stack.
func prepare(ctx *cli.Context) {
	// If we're running a known preset, log it for convenience.
	switch {
	case ctx.IsSet(utils.TestnetFlag.Name):
		log.Info("Starting Kaigo on testnet...")

	case !ctx.IsSet(utils.NetworkIdFlag.Name):
		log.Info("Starting Kaigo on Kardiachain mainnet...")
	}

	// Start metrics export if enabled
	utils.SetupMetrics(ctx)

	// Start system runtime metrics collection
	go metrics.CollectProcessMetrics(3 * time.Second)
}

// kaigo is the main entry point into the system if no special subcommand is run.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func kaigo(ctx *cli.Context) error {
	if args := ctx.Args().Slice(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	prepare(ctx)
	stack, _ := makeFullNode(ctx)

	utils.StartNode(ctx, stack, false)
	stack.Wait()
	return nil
}
