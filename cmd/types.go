/*
 *  Copyright 2019 KardiaChain
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
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
)

type (
	Config struct {
		Node      `yaml:"Node"`
		MainChain *Chain `yaml:"MainChain"`
		DualChain *Chain `yaml:"DualChain,omitempty"`
	}
	Node struct {
		P2P struct {
			ListenAddress string `yaml:"ListenAddress"`
			PrivateKey    string `yaml:"PrivateKey"`
		} `yaml:"P2P"`
		LogLevel         string   `yaml:"LogLevel"`
		Name             string   `yaml:"Name"`
		DataDir          string   `yaml:"DataDir"`
		HTTPHost         string   `yaml:"HTTPHost"`
		HTTPPort         int      `yaml:"HTTPPort"`
		HTTPModules      []string `yaml:"HTTPModules"`
		HTTPVirtualHosts []string `yaml:"HTTPVirtualHosts"`
		HTTPCors         []string `yaml:"HTTPCors"`
		Metrics          uint     `yaml:"Metrics"`
	}
	Chain struct {
		ServiceName        string      `yaml:"ServiceName"`
		Protocol           *string     `yaml:"Protocol,omitempty"`
		ChainID            uint64      `yaml:"ChainID"`
		NetworkID          uint64      `yaml:"NetworkID"`
		AcceptTxs          uint32      `yaml:"AcceptTxs"`
		ZeroFee            uint        `yaml:"ZeroFee"`
		IsDual             uint        `yaml:"IsDual"`
		Genesis            *Genesis    `yaml:"Genesis,omitempty"`
		TxPool             *Pool       `yaml:"TxPool,omitempty"`
		EventPool          *Pool       `yaml:"EventPool,omitempty"`
		Database           *Database   `yaml:"Database,omitempty"`
		Seeds              []string    `yaml:"Seeds"`
		Events             []Event     `yaml:"Events"`
		PublishedEndpoint  *string     `yaml:"PublishedEndpoint,omitempty"`
		SubscribedEndpoint *string     `yaml:"SubscribedEndpoint,omitempty"`
		Validators         []int       `yaml:"Validators"`
		BaseAccount        BaseAccount `yaml:"BaseAccount"`
	}
	Genesis struct {
		Addresses     []string                    `yaml:"Addresses"`
		GenesisAmount string                      `yaml:"GenesisAmount"`
		Contracts     []Contract                  `yaml:"Contracts"`
		Validators    []*genesis.GenesisValidator `yaml:"Validators"`
	}
	Contract struct {
		Address  string `yaml:"Address"`
		ByteCode string `yaml:"ByteCode"`
		ABI      string `yaml:"ABI,omitempty"`
	}
	Pool struct {
		GlobalSlots uint64 `yaml:"GlobalSlots"`
		GlobalQueue uint64 `yaml:"GlobalQueue"`
		BlockSize   int    `yaml:"BlockSize"`
	}
	Database struct {
		Type    uint   `yaml:"Type"`
		Dir     string `yaml:"Dir"`
		Caches  int    `yaml:"Caches"`
		Handles int    `yaml:"Handles"`
		Drop    int    `yaml:"Drop"`
	}
	Event struct {
		MasterSmartContract string    `yaml:"MasterSmartContract"`
		ContractAddress     string    `yaml:"ContractAddress"`
		MasterABI           *string   `yaml:"MasterABI"`
		ABI                 *string   `yaml:"ABI,omitempty"`
		Watchers            []Watcher `yaml:"Watchers"`
	}
	Watcher struct {
		Method         string   `yaml:"Method"`
		WatcherActions []string `yaml:"WatcherActions,omitempty"`
		DualActions    []string `yaml:"DualActions"`
	}
	BaseAccount struct {
		Address    string `yaml:"Address"`
		PrivateKey string `yaml:"PrivateKey"`
	}
)
