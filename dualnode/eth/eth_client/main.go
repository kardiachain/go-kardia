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

package main

import (
	"context"
	"flag"
	"time"

	"github.com/kardiachain/go-kardia/lib/log"
)

// args
type flagArgs struct {
	path         string
	name         string
	ethNetworkId int
	ethStat      bool
	ethStatName  string

	publishedEndpoint  string
	subscribedEndpoint string
}

var args flagArgs

func init() {
	flag.StringVar(&args.path, "path", "./", "path to config file")
	flag.StringVar(&args.name, "name", "config", "config file name")
	flag.IntVar(&args.ethNetworkId, "ethNetworkId", 4, "run Eth network id, 4: rinkeby, 3: ropsten, 1: mainnet")
	flag.BoolVar(&args.ethStat, "ethstat", true, "report eth stats to network")
	flag.StringVar(&args.ethStatName, "ethstatname", "", "name to use when reporting eth stats")
	flag.StringVar(&args.publishedEndpoint, "publishedEndpoint", "", "0MQ Endpoint that message will be published to")
	flag.StringVar(&args.subscribedEndpoint, "subscribedEndpoint", "", "0MQ Endpoint that dual node subscribes to get dual message.")
}

func main() {
	flag.Parse()

	// Setups config.
	config, err := Load(args.path, args.name)
	if err != nil {
		panic(err)
	}

	config.NetworkId = args.ethNetworkId

	if args.ethStatName != "" {
		config.StatName = args.ethStatName
	}

	if args.publishedEndpoint != "" {
		config.PublishedEndpoint = args.publishedEndpoint
	}

	if args.subscribedEndpoint != "" {
		config.SubscribedEndpoint = args.subscribedEndpoint
	}

	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(config.LogLvl), log.StdoutHandler))
	config.Logger = log.New()
	ethNode, err := NewEth(config)
	if err != nil {
		log.Error("Fail to create Eth sub node", "err", err)
		return
	}
	if err := ethNode.Start(); err != nil {
		log.Error("Fail to start Eth sub node", "err", err)
		return
	}
	go displaySyncStatus(ethNode)
	waitForever()
}

func waitForever() {
	select {}
}

func displaySyncStatus(eth *Eth) {
	for {
		client, _, err := eth.Client()
		if err != nil {
			log.Error("Fail on getting ETH's client", "err", err)
		}
		status, err := client.SyncProgress(context.Background())
		if err != nil {
			log.Error("Fail to check sync status of EthKarida", "err", err)
		} else {
			log.Info("Sync status", "sync", status)
		}
		time.Sleep(20 * time.Second)
	}
}
