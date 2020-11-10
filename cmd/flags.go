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
		Mainnet: {
			genesis: "./cfg/genesis.yaml",
			kardia:  "./cfg/kai_config.yaml",
			dual:    "",
		},
		Testnet: {
			genesis: "./cfg/genesis_testnet.yaml",
			kardia:  "./cfg/kai_config_testnet.yaml",
			dual:    "",
		},
		Devnet: {
			genesis: "./cfg/genesis_devnet.yaml",
			kardia:  "./cfg/kai_config_devnet.yaml",
			dual:    "",
		},
	}
)

func initFlag(args *flags) {
	flag.StringVar(&args.genesis, "genesis", "", "Path to genesis config file. Default: ${wd}/cfg/genesis.yaml")
	flag.StringVar(&args.kardia, "node", "", "Path to Kardia node config file. Default: ${wd}/cfg/kai_config.yaml")
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

func getDefaultFlag(network string) flags {
	return defaultFlags[network]
}
