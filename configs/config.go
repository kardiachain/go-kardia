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
	"strings"

	_default "github.com/kardiachain/go-kardiamain/configs/default"
	"github.com/kardiachain/go-kardiamain/configs/types"
	"github.com/kardiachain/go-kardiamain/lib/common"
)

// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	TestnetGenesisHash = common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d")

	GenesisDeployerAddr    = common.BytesToAddress([]byte{0x1})
	StakingContractAddress common.Address
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &typesCfg.ChainConfig{
		Kaicon: &typesCfg.KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &typesCfg.ChainConfig{
		Kaicon: &typesCfg.KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestChainConfig contains the chain parameters to run unit test.
	TestChainConfig = &typesCfg.ChainConfig{
		Kaicon: &typesCfg.KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}
)

// -------- Consensus Config ---------
// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *typesCfg.ConsensusConfig {
	return _default.ConsensusConfig()
}

var contracts = make(map[string]typesCfg.Contract)

func LoadGenesisContract(contractType string, address string, bytecode string, abi string) {
	if contractType == "Staking" {
		StakingContractAddress = common.HexToAddress(address)
	}
	contracts[contractType] = typesCfg.Contract{
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
