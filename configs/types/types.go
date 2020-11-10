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

package typesCfg

import (
	"crypto/ecdsa"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
)

type (
	Node struct {
		P2P              P2P      `yaml:"P2P" validate:"required"`
		LogLevel         string   `yaml:"LogLevel"`
		Name             string   `yaml:"Name"`
		DataDir          string   `yaml:"DataDir"`
		HTTPHost         string   `yaml:"HTTPHost"`
		HTTPPort         int      `yaml:"HTTPPort"`
		HTTPModules      []string `yaml:"HTTPModules"`
		HTTPVirtualHosts []string `yaml:"HTTPVirtualHosts"`
		HTTPCors         []string `yaml:"HTTPCors"`
		Metrics          uint     `yaml:"Metrics"`
		Genesis          *Genesis `yaml:"Genesis,omitempty" validate:"required"`
		Debug            *Debug   `yaml:"Debug,omitempty" `
	}
	Chain struct {
		ServiceName        string     `yaml:"ServiceName"`
		Protocol           *string    `yaml:"Protocol,omitempty"`
		ChainID            uint64     `yaml:"ChainID"`
		NetworkID          uint64     `yaml:"NetworkID"`
		AcceptTxs          uint32     `yaml:"AcceptTxs"`
		ZeroFee            uint       `yaml:"ZeroFee"`
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
	P2P struct {
		ListenAddress string `yaml:"ListenAddress"`
		PrivateKey    string `yaml:"PrivateKey"`
	}
	Debug struct {
		Port string `yaml:"Port"`
	}
	BaseAccount struct {
		Address    common.Address `json:"address"`
		PrivateKey ecdsa.PrivateKey
	}
)

//region Genesis
type Genesis struct {
	Addresses       []string            `yaml:"Addresses"`
	GenesisAmount   string              `yaml:"GenesisAmount"`
	Contracts       map[string]Contract `yaml:"Contracts" `
	Validators      []*Validator        `yaml:"Validators"`
	ConsensusParams *ConsensusParams    `yaml:"ConsensusParams"`
	Consensus       *Consensus          `yaml:"Consensus"`
	ChainConfig     *ChainConfig        `yaml:"ChainConfig"`
	TxPool          *Pool               `yaml:"TxPool,omitempty"`
}

type (
	Pool struct {
		AccountSlots  uint64 `yaml:"AccountSlots"`
		AccountQueue  uint64 `yaml:"AccountQueue"`
		GlobalSlots   uint64 `yaml:"GlobalSlots"`
		GlobalQueue   uint64 `yaml:"GlobalQueue"`
		BlockSize     int    `yaml:"BlockSize,omitempty"`
		Broadcast     bool   `yaml:"Broadcast"`
		MaxBatchBytes int    `yaml:"MaxBatchBytes"`
	}
	Contract struct {
		Address  string `yaml:"Address"`
		ByteCode string `yaml:"ByteCode"`
		ABI      string `yaml:"ABI,omitempty"`
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
	Validator struct {
		Address string `json:"address" yaml:"Address"`
		Power   int64  `json:"power" yaml:"Power"`
		Name    string `json:"name" yaml:"Name"`
	}
)

// ConsensusConfig defines the configuration for the Kardia consensus service,
// including timeouts and details about the block structure.
// All loader should convert to this object despite of which source
type ConsensusConfig struct {
	// All timeouts are in milliseconds
	TimeoutPropose        time.Duration `mapstructure:"timeout_propose"`
	TimeoutProposeDelta   time.Duration `mapstructure:"timeout_propose_delta"`
	TimeoutPrevote        time.Duration `mapstructure:"timeout_prevote"`
	TimeoutPrevoteDelta   time.Duration `mapstructure:"timeout_prevote_delta"`
	TimeoutPrecommit      time.Duration `mapstructure:"timeout_precommit"`
	TimeoutPrecommitDelta time.Duration `mapstructure:"timeout_precommit_delta"`
	TimeoutCommit         time.Duration `mapstructure:"timeout_commit"`

	// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
	IsSkipTimeoutCommit bool `mapstructure:"is_skip_timeout_commit"`

	// EmptyBlocks mode and possible interval between empty blocks in seconds
	IsCreateEmptyBlocks       bool          `mapstructure:"is_create_empty_blocks"`
	CreateEmptyBlocksInterval time.Duration `mapstructure:"create_empty_blocks_interval"`

	// Reactor sleep duration parameters are in milliseconds
	PeerGossipSleepDuration     time.Duration `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration time.Duration `mapstructure:"peer_query_maj23_sleep_duration"`
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.IsCreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(cfg.TimeoutCommit)
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round uint32) time.Duration {
	return time.Duration(
		cfg.TimeoutPropose.Nanoseconds()+cfg.TimeoutProposeDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round uint32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrevote.Nanoseconds()+cfg.TimeoutPrevoteDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round uint32) time.Duration {
	return time.Duration(
		cfg.TimeoutPrecommit.Nanoseconds()+cfg.TimeoutPrecommitDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// PeerGossipSleep returns the amount of time to sleep if there is nothing to send from the ConsensusReactor
func (cfg *ConsensusConfig) PeerGossipSleep() time.Duration {
	return cfg.PeerGossipSleepDuration
}

// PeerQueryMaj23Sleep returns the amount of time to sleep after each VoteSetMaj23Message is sent in the ConsensusReactor
func (cfg *ConsensusConfig) PeerQueryMaj23Sleep() time.Duration {
	return cfg.PeerQueryMaj23SleepDuration
}

// ======================= Genesis Utils Functions =======================
//endregion Genesis
