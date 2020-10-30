/*
 *  Copyright 2020 KardiaChain
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

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type flags struct {
	genesis string
	kardia  string
	dual    string
	network string
}

const (
	Mainnet = "mainnet"
	Testnet = "testnet"
	Devnet  = "devnet"
)

var (
	defaultFlags = map[string]flags{
		Mainnet: flags{
			genesis: "../../cmd/cfg/genesis.yaml",
			kardia:  "../../cmd/cfg/kai_config.yaml",
			dual:    "",
		},
		Testnet: flags{
			genesis: "../../cmd/cfg/genesis_testnet.yaml",
			kardia:  "../../cmd/cfg/kai_config_testnet.yaml",
			dual:    "",
		},
		Devnet: flags{
			genesis: "../../cmd/cfg/genesis_devnet.yaml",
			kardia:  "../../cmd/cfg/kai_config_devnet.yaml",
			dual:    "",
		},
	}
)

func initFlag(args *flags) {
	flag.StringVar(&args.genesis, "genesis", "", "Path to genesis config file. Default: ${wd}../../cmd/cfg/genesis.yaml")
	flag.StringVar(&args.kardia, "node", "", "Path to Kardia node config file. Default: ${wd}../../cmd/cfg/kai_config.yaml")
	flag.StringVar(&args.dual, "dual", "", "Path to dual node config file. Default: \"\"")
	flag.StringVar(&args.network, "network", "mainnet", "Target network, choose one [mainnet, testnet, devnet]. Default: \"mainnet\"")
}

func init() {
	initFlag(&args)
}

// finalizeConfigParams fills missing config options with default values, based on target network
func finalizeConfigParams(args *flags) {
	if args.network != Mainnet && args.network != Testnet && args.network != Devnet {
		panic("unknown target network")
	}
	if args.genesis == "" {
		args.genesis = defaultFlags[args.network].genesis
	}
	if args.kardia == "" {
		args.kardia = defaultFlags[args.network].kardia
	}
	if args.dual == "" {
		args.dual = defaultFlags[args.network].dual
	}
}

// Load attempts to load the config from given path and filename.
func LoadConfig(args flags) (*Config, error) {
	finalizeConfigParams(&args)
	var (
		wd  string
		err error
	)
	wd, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	config := Config{}
	genesisCfgFile := filepath.Join(wd, args.genesis)
	kaiCfgFile := filepath.Join(wd, args.kardia)

	kaiCfg, err := ioutil.ReadFile(kaiCfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read kai config")
	}
	err = yaml.Unmarshal(kaiCfg, &config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal kai config")
	}

	genesisCfg, err := ioutil.ReadFile(genesisCfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read node config")
	}
	err = yaml.Unmarshal(genesisCfg, &config.MainChain)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal node config")
	}
	config.Genesis = config.MainChain.Genesis

	if args.dual != "" {
		chainCfgFile := filepath.Join(wd, args.dual)
		chainCfg, err := ioutil.ReadFile(chainCfgFile)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read dual node config")
		}
		err = yaml.Unmarshal(chainCfg, &config.DualChain)
		if err != nil {
			return nil, errors.Wrap(err, "cannot unmarshal dual node config")
		}
	}

	return &config, nil
}
