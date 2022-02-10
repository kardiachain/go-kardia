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

package configs

import (
	"math"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	kaiproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

// Genesis hashes to enforce below configs on.
var (
	EthDualChainID  = uint64(2)
	NeoDualChainID  = uint64(3)
	TronDualChainID = uint64(4)

	MainnetGenesisHash = common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	TestnetGenesisHash = common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d")

	GenesisDeployerAddr    = common.BytesToAddress([]byte{0x1})
	StakingContractAddress common.Address

	// PowerReduction calculation MUST sync up with power/token reduction in the staking smart contract
	// This reduction is used for speed up the kvm computing and computing fees
	// power = (amount of kai * 10^18)/ power reduction
	PowerReduction = big.NewInt(int64(math.Pow10(10)))
)

var (
	// ChainID
	MainnetChainID = big.NewInt(24)
	TestnetChainID = big.NewInt(242)
	// Network ID
	MainnetNetworkID = uint64(0)
	TestnetNetworkID = uint64(69)
	// scheduled block for Galaxias Hardfork
	galaxiasForkBlock        = uint64(6039393)
	testnetGalaxiasForkBlock = uint64(4823900)

	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
		GalaxiasBlock: &galaxiasForkBlock,
		ChainID:       MainnetChainID,
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
		GalaxiasBlock: &testnetGalaxiasForkBlock,
		ChainID:       TestnetChainID,
	}

	// TestChainConfig contains the chain parameters to run unit test.
	TestChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
		GalaxiasBlock: &galaxiasForkBlock,
		ChainID:       TestnetChainID,
	}
)

type Config struct {
	Consensus *ConsensusConfig
}

// -------- Consensus Params ---------

// DefaultConsensusParams returns default param values for the consensus service
func DefaultConsensusParams() *kaiproto.ConsensusParams {
	return &kaiproto.ConsensusParams{
		Block: kaiproto.BlockParams{
			MaxBytes:   BlockMaxBytes,
			MaxGas:     BlockGasLimit,
			TimeIotaMs: 1000,
		},
		Evidence: kaiproto.EvidenceParams{
			MaxAgeNumBlocks: 100000,
			MaxAgeDuration:  48 * time.Hour,
			MaxBytes:        1048576,
		},
	}
}

// TestConsensusParams returns a configuration for testing the consensus service
func TestConsensusParams() *kaiproto.ConsensusParams {
	csParams := DefaultConsensusParams()
	csParams.Block = kaiproto.BlockParams{
		MaxBytes:   104857600,
		MaxGas:     20000000,
		TimeIotaMs: 1000,
	}
	csParams.Evidence = kaiproto.EvidenceParams{
		MaxAgeNumBlocks: 100000,
		MaxAgeDuration:  48 * time.Hour,
		MaxBytes:        1048576,
	}
	return csParams
}

// -------- Consensus Config ---------

// ConsensusConfig defines the configuration for the Kardia consensus service,
// including timeouts and details about the block structure.
type ConsensusConfig struct {
	RootDir string `mapstructure:"home"`
	WalPath string `mapstructure:"wal_file"`
	walFile string // overrides WalPath if set

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

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		WalPath:                     filepath.Join(DefaultDataDir(), "cs.wal", "wal"),
		TimeoutPropose:              3000 * time.Millisecond,
		TimeoutProposeDelta:         500 * time.Millisecond,
		TimeoutPrevote:              1000 * time.Millisecond,
		TimeoutPrevoteDelta:         500 * time.Millisecond,
		TimeoutPrecommit:            1000 * time.Millisecond,
		TimeoutPrecommitDelta:       500 * time.Millisecond,
		TimeoutCommit:               1000 * time.Millisecond,
		IsSkipTimeoutCommit:         false,
		IsCreateEmptyBlocks:         true,
		CreateEmptyBlocksInterval:   3500 * time.Millisecond,
		PeerGossipSleepDuration:     100 * time.Millisecond,
		PeerQueryMaj23SleepDuration: 2000 * time.Millisecond,
	}
}

// TestConsensusConfig returns a configuration for testing the consensus service
func TestConsensusConfig() *ConsensusConfig {
	cfg := DefaultConsensusConfig()
	cfg.TimeoutPropose = 40 * time.Millisecond
	cfg.TimeoutProposeDelta = 1 * time.Millisecond
	cfg.TimeoutPrevote = 10 * time.Millisecond
	cfg.TimeoutPrevoteDelta = 1 * time.Millisecond
	cfg.TimeoutPrecommit = 10 * time.Millisecond
	cfg.TimeoutPrecommitDelta = 1 * time.Millisecond
	// NOTE: when modifying, make sure to update time_iota_ms (testGenesisFmt) in toml.go
	cfg.TimeoutCommit = 10 * time.Millisecond
	cfg.IsSkipTimeoutCommit = false
	cfg.CreateEmptyBlocksInterval = 0
	cfg.PeerGossipSleepDuration = 5 * time.Millisecond
	cfg.PeerQueryMaj23SleepDuration = 250 * time.Millisecond
	//cfg.DoubleSignCheckHeight = int64(0)
	return cfg
}

// WalFile returns the full path to the write-ahead log file
func (cfg *ConsensusConfig) WalFile() string {
	if cfg.walFile != "" {
		return cfg.walFile
	}
	return rootify(cfg.WalPath, cfg.RootDir)
}

// SetWalFile sets the path to the write-ahead log file
func (cfg *ConsensusConfig) SetWalFile(walFile string) {
	cfg.walFile = walFile
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

// ------------------------- Consensus Params ----------------------------
type FastSyncConfig struct {
	ServiceName   string        // log tag of blockchain reactor logs
	Enable        bool          // true if this node allow and be able to fastsync, otherwise false.
	MaxPeers      int           // maximum peer is allowed to receive fastsync blocks from this node at a time.
	TargetPending int           // maximum number of blocks in a batch sync.
	PeerTimeout   time.Duration // maximum response time from a peer.
	MinRecvRate   int64         // minimum receive rate from peer, otherwise prune.
}

func DefaultFastSyncConfig() *FastSyncConfig {
	return &FastSyncConfig{
		ServiceName:   DefaultBcReactorServiceName,
		Enable:        true,
		MaxPeers:      10,
		TargetPending: 10,
		PeerTimeout:   15 * time.Second,
		MinRecvRate:   0, // int64(7680)
	}
}

func TestFastSyncConfig() *FastSyncConfig {
	return &FastSyncConfig{
		ServiceName:   DefaultBcReactorServiceName,
		Enable:        true,
		MaxPeers:      2,
		TargetPending: 5,
		PeerTimeout:   2 * time.Second,
		MinRecvRate:   0,
	}
}

// ======================= Genesis Utils Functions =======================

type Contract struct {
	Address  string
	ByteCode string
	ABI      string
}

var contracts = make(map[string]Contract)

func LoadGenesisContract(contractType string, address string, bytecode string, abi string) {
	if contractType == StakingContractKey {
		StakingContractAddress = common.HexToAddress(address)
	}
	contracts[contractType] = Contract{
		Address:  address,
		ByteCode: bytecode,
		ABI:      abi,
	}
}

func GetContractABIByAddress(address string) string {
	for _, contract := range contracts {
		if strings.EqualFold(address, contract.Address) {
			return contract.ABI
		}
	}
	panic("ABI not found")
}

func GetContractByteCodeByAddress(address string) string {
	for _, contract := range contracts {
		if strings.EqualFold(address, contract.Address) {
			return contract.ByteCode
		}
	}
	panic("ByteCode not found")
}

func GetContractABIByType(contractType string) string {
	if contracts[contractType].ABI != "" {
		return contracts[contractType].ABI
	}
	panic("ABI not found")
}

func GetContractByteCodeByType(contractType string) string {
	if contracts[contractType].ByteCode != "" {
		return contracts[contractType].ByteCode
	}
	panic("ABI not found")
}
