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
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
)

type (
	Config struct {
		Node      `yaml:"Node"`
		MainChain *Chain `yaml:"MainChain"`
		DualChain *Chain `yaml:"DualChain,omitempty"`
		Debug     *Debug `yaml:"Debug"` // todo @longnd: Change this config name to profile
	}
	Node struct {
		P2P struct {
			ListenAddress string `yaml:"ListenAddress"`
			PrivateKey    string `yaml:"PrivateKey"`
		} `yaml:"P2P"`
		LogLevel         string     `yaml:"LogLevel"`
		Name             string     `yaml:"Name"`
		DataDir          string     `yaml:"DataDir"`
		HTTPHost         string     `yaml:"HTTPHost"`
		HTTPPort         int        `yaml:"HTTPPort"`
		HTTPModules      []string   `yaml:"HTTPModules"`
		HTTPVirtualHosts []string   `yaml:"HTTPVirtualHosts"`
		HTTPCors         []string   `yaml:"HTTPCors"`
		WSHost           string     `yaml:"WSHost"`
		WSPort           int        `yaml:"WSPort"`
		Metrics          bool       `yaml:"Metrics"`
		FastSync         *FastSync  `yaml:"FastSync"`
		GasOracle        *GasOracle `yaml:"GasOracle"`
		Genesis          *Genesis   `yaml:"Genesis,omitempty"`
	}
	GasOracle struct {
		Blocks     int    `yaml:"Blocks"`
		Percentile int    `yaml:"Percentile"`
		Default    string `yaml:"Default"`
		MaxPrice   string `yaml:"MaxPrice"`
	}
	FastSync struct {
		ServiceName   string `yaml:"ServiceName"`
		Enable        bool   `yaml:"Enable"`
		MaxPeers      int    `yaml:"MaxPeers"`
		TargetPending int    `yaml:"TargetPending"`
		PeerTimeout   int    `yaml:"PeerTimeout"`
		MinRecvRate   int64  `yaml:"MinRecvRate"`
	}
	Chain struct {
		ServiceName        string     `yaml:"ServiceName"`
		Protocol           *string    `yaml:"Protocol,omitempty"`
		ChainID            uint64     `yaml:"ChainID"`
		NetworkID          uint64     `yaml:"NetworkID"`
		AcceptTxs          uint32     `yaml:"AcceptTxs"`
		IsDual             uint       `yaml:"IsDual"`
		Genesis            *Genesis   `yaml:"Genesis,omitempty"`
		EventPool          *Pool      `yaml:"EventPool,omitempty"`
		Database           *Database  `yaml:"Database,omitempty"`
		Seeds              []string   `yaml:"Seeds"`
		Events             []Event    `yaml:"Events"`
		PublishedEndpoint  *string    `yaml:"PublishedEndpoint,omitempty"`
		SubscribedEndpoint *string    `yaml:"SubscribedEndpoint,omitempty"`
		Consensus          *Consensus `yaml:"Consensus"`
	}
	Genesis struct {
		Accounts        []Account                   `yaml:"Accounts"`
		Contracts       map[string]Contract         `yaml:"Contracts"`
		Validators      []*genesis.GenesisValidator `yaml:"Validators"`
		ConsensusParams *ConsensusParams            `yaml:"ConsensusParams"`
		Consensus       *Consensus                  `yaml:"Consensus"`
		ChainConfig     *configs.ChainConfig        `yaml:"ChainConfig"`
		TxPool          *Pool                       `yaml:"TxPool,omitempty"`
		Timestamp       int64                       `yaml:"Timestamp,omitempty"`
	}
	Account struct {
		Address string `yaml:"Address"`
		Amount  string `yaml:"Amount"`
	}
	Contract struct {
		Address  string `yaml:"Address"`
		ByteCode string `yaml:"ByteCode"`
		ABI      string `yaml:"ABI,omitempty"`
	}
	Pool struct {
		AccountSlots  uint64 `yaml:"AccountSlots"`
		AccountQueue  uint64 `yaml:"AccountQueue"`
		GlobalSlots   uint64 `yaml:"GlobalSlots"`
		GlobalQueue   uint64 `yaml:"GlobalQueue"`
		BlockSize     int    `yaml:"BlockSize,omitempty"`
		Broadcast     bool   `yaml:"Broadcast"`
		MaxBatchBytes int    `yaml:"MaxBatchBytes"`
	}
	Database struct {
		Type    uint   `yaml:"Type"`
		Dir     string `yaml:"Dir"`
		Caches  int    `yaml:"Caches"`
		Handles int    `yaml:"Handles"`
		Drop    int    `yaml:"Drop"`
	}
	Event struct {
		MasterSmartContract string  `yaml:"MasterSmartContract"`
		ContractAddress     string  `yaml:"ContractAddress"`
		MasterABI           *string `yaml:"MasterABI"`
		ABI                 *string `yaml:"ABI,omitempty"`
	}
	Consensus struct {
		// All timeouts are in milliseconds
		TimeoutPropose        int `yaml:"TimeoutPropose"`
		TimeoutProposeDelta   int `yaml:"TimeoutProposeDelta"`
		TimeoutPrevote        int `yaml:"TimeoutPrevote"`
		TimeoutPrevoteDelta   int `yaml:"TimeoutPrevoteDelta"`
		TimeoutPrecommit      int `yaml:"TimeoutPrecommit"`
		TimeoutPrecommitDelta int `yaml:"TimeoutPrecommitDelta"`
		TimeoutCommit         int `yaml:"TimeoutCommit"`

		// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
		IsSkipTimeoutCommit bool `yaml:"IsSkipTimeoutCommit"`

		// EmptyBlocks mode and possible interval between empty blocks in seconds
		IsCreateEmptyBlocks       bool `yaml:"IsCreateEmptyBlocks"`
		CreateEmptyBlocksInterval int  `yaml:"CreateEmptyBlocksInterval"`

		// Reactor sleep duration parameters are in milliseconds
		PeerGossipSleepDuration     int `yaml:"PeerGossipSleepDuration"`
		PeerQueryMaj23SleepDuration int `yaml:"PeerQueryMaj23SleepDuration"`
	}
	ConsensusParams struct {
		Block    BlockParams    `yaml:"Block"`
		Evidence EvidenceParams `yaml:"Evidence"`
	}
	BlockParams struct {
		MaxBytes   int64  `yaml:"MaxBytes"`
		MaxGas     uint64 `yaml:"MaxGas"`
		TimeIotaMs int64  `yaml:"TimeIotaMs"`
	}
	EvidenceParams struct {
		MaxAgeNumBlocks int64 `yaml:"MaxAgeNumBlocks"`
		MaxAgeDuration  int   `yaml:"MaxAgeDuration"`
		MaxBytes        int64 `yaml:"MaxBytes"`
	}
	Debug struct {
		Port string `yaml:"Port"`
	}
)
