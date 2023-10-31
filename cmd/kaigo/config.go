// Copyright 2017 The go-ethereum Authors
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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unicode"

	"github.com/kardiachain/go-kardia/cmd/utils"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/accounts"
	"github.com/kardiachain/go-kardia/kai/accounts/keystore"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"
	kai "github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/node"
	"github.com/urfave/cli/v2"

	"github.com/kardiachain/go-kardia/internal/flags"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/internal/version"
	"github.com/naoina/toml"
)

var (
	dumpConfigCommand = &cli.Command{
		Action:      dumpConfig,
		Name:        "dumpconfig",
		Usage:       "Export configuration values in a TOML format",
		ArgsUsage:   "<dumpfile (optional)>",
		Flags:       flags.Merge(nodeFlags, rpcFlags),
		Description: `Export configuration values in TOML format (to stdout by default).`,
	}

	configFileFlag = &cli.StringFlag{
		Name:     "config",
		Usage:    "TOML configuration file",
		Category: flags.KaiCategory,
	}
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		id := fmt.Sprintf("%s.%s", rt.String(), field)
		if deprecated(id) {
			log.Warn("Config field is deprecated and won't have an effect", "name", id)
			return nil
		}
		var link string
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

type kaigoConfig struct {
	Kai     kai.Config
	Node    node.Config
	Metrics metrics.Config
}

func loadConfig(file string, cfg *kaigoConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	git, _ := version.VCS()
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = configs.VersionWithCommit(git.Commit, git.Date)
	cfg.IPCPath = "kaigo.ipc"
	return cfg
}

// loadBaseConfig loads the gethConfig based on the given command line
// parameters and config file.
func loadBaseConfig(ctx *cli.Context) kaigoConfig {
	// Load defaults.
	cfg := kaigoConfig{
		Kai:     kai.Defaults,
		Node:    defaultNodeConfig(),
		Metrics: metrics.DefaultConfig,
	}

	// Load config file.
	if file := ctx.String(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	if cfg.Node.P2P.PrivateKeyRaw != "" {
		cfg.Node.P2P.PrivateKey, _ = crypto.HexToECDSA(cfg.Node.P2P.PrivateKeyRaw)
	} else {
		cfg.Node.P2P.PrivateKey, _ = crypto.GenerateKey()
	}

	if cfg.Node.TimeOutForStaticCall > 0 {
		configs.TimeOutForStaticCall = cfg.Node.TimeOutForStaticCall
	} else {
		configs.TimeOutForStaticCall = configs.DefaultTimeOutForStaticCall
	}

	// Copy duplicate configs
	cfg.Node.Genesis = cfg.Kai.Genesis
	cfg.Node.FastSync = cfg.Kai.FastSync
	cfg.Node.GasOracle = cfg.Kai.GasOracle

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	return cfg
}

// makeConfigNode loads kai configuration and creates a blank node instance.
func makeConfigNode(ctx *cli.Context) (*node.Node, kaigoConfig) {
	cfg := loadBaseConfig(ctx)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	// Node doesn't by default populate account manager backends
	if err := setAccountManagerBackends(stack.Config(), stack.AccountManager(), stack.KeyStoreDir()); err != nil {
		utils.Fatalf("Failed to set account manager backends: %v", err)
	}

	utils.SetKaiConfig(ctx, stack, &cfg.Kai)

	// Update datadir changes
	if cfg.Node.DataDir != configs.DefaultDataDir() {
		cfg.Node.P2P.RootDir = cfg.Node.DataDir
		cfg.Kai.Consensus.RootDir = cfg.Node.DataDir
	}

	applyMetricConfig(ctx, &cfg)

	return stack, cfg
}

// makeFullNode loads kai configuration and creates the Ethereum backend.
func makeFullNode(ctx *cli.Context) (*node.Node, kaiapi.Backend) {
	stack, cfg := makeConfigNode(ctx)
	backend, _ := utils.RegisterKaiService(stack, &cfg.Kai)

	return stack, backend
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	if cfg.Kai.Genesis != nil {
		cfg.Kai.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}

	dump := os.Stdout
	if ctx.NArg() > 0 {
		dump, err = os.OpenFile(ctx.Args().Get(0), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dump.Close()
	}
	dump.WriteString(comment)
	dump.Write(out)

	return nil
}

func applyMetricConfig(ctx *cli.Context, cfg *kaigoConfig) {
	if ctx.IsSet(utils.MetricsEnabledFlag.Name) {
		cfg.Metrics.Enabled = ctx.Bool(utils.MetricsEnabledFlag.Name)
	}
	if ctx.IsSet(utils.MetricsEnabledExpensiveFlag.Name) {
		cfg.Metrics.EnabledExpensive = ctx.Bool(utils.MetricsEnabledExpensiveFlag.Name)
	}
	if ctx.IsSet(utils.MetricsHTTPFlag.Name) {
		cfg.Metrics.HTTP = ctx.String(utils.MetricsHTTPFlag.Name)
	}
	if ctx.IsSet(utils.MetricsPortFlag.Name) {
		cfg.Metrics.Port = ctx.Int(utils.MetricsPortFlag.Name)
	}
}

func deprecated(field string) bool {
	switch field {
	case "ethconfig.Config.EVMInterpreter":
		return true
	case "ethconfig.Config.EWASMInterpreter":
		return true
	default:
		return false
	}
}

func setAccountManagerBackends(conf *node.Config, am *accounts.Manager, keydir string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	if conf.UseLightweightKDF {
		scryptN = keystore.LightScryptN
		scryptP = keystore.LightScryptP
	}

	// Assemble the supported backends
	am.AddBackend(keystore.NewKeyStore(keydir, scryptN, scryptP))
	return nil
}
