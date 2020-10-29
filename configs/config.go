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
	"time"

	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

// TODO(huny): Get the proper genesis hash for Kardia when ready
// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	TestnetGenesisHash = common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d")

	GenesisDeployerAddr    = common.BytesToAddress([]byte{0x1})
	StakingContractAddress = common.HexToAddress("0x0000000000000000000000000000000000001337")
)

var (
	DefaultChainID  = uint64(1)
	EthDualChainID  = uint64(2)
	NeoDualChainID  = uint64(3)
	TronDualChainID = uint64(4)
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestChainConfig contains the chain parameters to run unit test.
	TestChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}
)

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

type Config struct {
	Consensus *ConsensusConfig
}

// -------- Consensus Params ---------

// DefaultConsensusParams returns default param values for the consensus service
func DefaultConsensusParams() *kaiproto.ConsensusParams {
	return &kaiproto.ConsensusParams{
		Block: kaiproto.BlockParams{
			MaxBytes:   104857600,
			MaxGas:     20000000,
			TimeIotaMs: 1000,
		},
		Evidence: kaiproto.EvidenceParams{
			MaxAgeNumBlocks: 100000, // 27.8 hrs at 1block/s
			MaxAgeDuration:  48 * time.Hour,
			MaxBytes:        1048576, // 1MB
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
		MaxAgeNumBlocks: 100000, // 27.8 hrs at 1block/s
		MaxAgeDuration:  48 * time.Hour,
		MaxBytes:        1048576, // 1MB
	}
	return csParams
}

// -------- Consensus Config ---------

// ConsensusConfig defines the configuration for the Kardia consensus service,
// including timeouts and details about the block structure.
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
	SkipTimeoutCommit bool `mapstructure:"skip_timeout_commit"`

	// EmptyBlocks mode and possible interval between empty blocks in seconds
	CreateEmptyBlocks         bool          `mapstructure:"create_empty_blocks"`
	CreateEmptyBlocksInterval time.Duration `mapstructure:"create_empty_blocks_interval"`

	// Reactor sleep duration parameters are in milliseconds
	PeerGossipSleepDuration     time.Duration `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration time.Duration `mapstructure:"peer_query_maj23_sleep_duration"`
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		TimeoutPropose:              3000 * time.Millisecond,
		TimeoutProposeDelta:         500 * time.Millisecond,
		TimeoutPrevote:              1000 * time.Millisecond,
		TimeoutPrevoteDelta:         500 * time.Millisecond,
		TimeoutPrecommit:            1000 * time.Millisecond,
		TimeoutPrecommitDelta:       500 * time.Millisecond,
		TimeoutCommit:               1000 * time.Millisecond,
		SkipTimeoutCommit:           false,
		CreateEmptyBlocks:           true,
		CreateEmptyBlocksInterval:   1 * time.Second,
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
	cfg.SkipTimeoutCommit = true
	cfg.CreateEmptyBlocksInterval = 0
	cfg.PeerGossipSleepDuration = 5 * time.Millisecond
	cfg.PeerQueryMaj23SleepDuration = 250 * time.Millisecond
	//cfg.DoubleSignCheckHeight = int64(0)
	return cfg
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool {
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
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

var InitValue = big.NewInt(int64(math.Pow10(10))) // Update Genesis Account Values
var InitValueInCell = InitValue.Mul(InitValue, big.NewInt(int64(math.Pow10(18))))

// GenesisAccounts are used to initialized accounts in genesis block
var GenesisAccounts = map[string]*big.Int{
	// TODO(kiendn): These addresses are same of node address. Change to another set.
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": InitValueInCell,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": InitValueInCell,
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": InitValueInCell,
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": InitValueInCell,
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": InitValueInCell,
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": InitValueInCell,
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": InitValueInCell,
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": InitValueInCell,
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": InitValueInCell,
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": InitValueInCell,
	"0x66BAB3F68Ff0822B7bA568a58A5CB619C4825Ce5": InitValueInCell,
	"0x88e1B4289b639C3b7b97899Be32627DCd3e81b7e": InitValueInCell,
	"0xCE61E95666737E46B2453717Fe1ba0d9A85B9d3E": InitValueInCell,
	"0x1A5193E85ffa06fde42b2A2A6da7535BA510aE8C": InitValueInCell,
	"0xb19BC4477ff32EC13872a2A827782DeA8b6E92C0": InitValueInCell,
	"0x0fFFA18f6c90ce3f02691dc5eC954495EA483046": InitValueInCell,
	"0x8C10639F908FED884a04C5A49A2735AB726DDaB4": InitValueInCell,
	"0x2BB7316884C7568F2C6A6aDf2908667C0d241A66": InitValueInCell,

	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"0x36BE7365e6037bD0FDa455DC4d197B07A2002547": 100000000,
}

//  GenesisAddrKeys maps genesis account addresses to private keys.
var GenesisAddrKeys = map[string]string{
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": "77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": "98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": "32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": "68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": "049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": "9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": "b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": "0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e689",
	"0x66BAB3F68Ff0822B7bA568a58A5CB619C4825Ce5": "2003be66077b0873f5bedb32a596530eb8a0c908c32dda7771f169ee137c1f82",
	"0x88e1B4289b639C3b7b97899Be32627DCd3e81b7e": "9dce5ec0b40e363e898f296c01345c12a0961f1cccad098964c73ed85fef5850",
	"0xCE61E95666737E46B2453717Fe1ba0d9A85B9d3E": "f0b2f6f24b70481a51712639badf0e5587545080dc53e0664770adb9881823fb",
	"0x1A5193E85ffa06fde42b2A2A6da7535BA510aE8C": "83731e17afb0da61c0026eaf780364eee367c50a5225ece89a63ad94a4a1f088",
	"0xb19BC4477ff32EC13872a2A827782DeA8b6E92C0": "fc09d3f004b1ee430fee60568aa29748e277e76f1f372eea9d2b9ff1e27bdfdb",
	"0x0fFFA18f6c90ce3f02691dc5eC954495EA483046": "5605dd5f4db003c396956b4b80c093c472ccef4021181aa910125d7c57324152",
	"0x8C10639F908FED884a04C5A49A2735AB726DDaB4": "9813a1dffe303131d1fe80b6fe872206267abd8ff84a52c907b0d32df582b1eb",
	"0x2BB7316884C7568F2C6A6aDf2908667C0d241A66": "4561f7d91a4f95ef0a72550fa423febaad3594f91611f9a2b10a7af4d3deb9ed",
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d": "0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
}

type Contract struct {
	address  string
	bytecode string
	abi      string
}

var contracts []Contract

func LoadGenesisContract(address string, bytecode string, abi string) {
	contracts = append(contracts, Contract{
		address:  address,
		bytecode: bytecode,
		abi:      abi,
	})
}

func GetContractABIByAddress(address string) string {
	for _, contract := range contracts {
		if strings.EqualFold(address, contract.address) {
			return contract.abi
		}
	}
	panic("abi not found")
}

func GetContractByteCodeByAddress(address string) string {
	for _, contract := range contracts {
		if strings.EqualFold(address, contract.address) {
			return contract.bytecode
		}
	}
	panic("bytecode not found")
}
