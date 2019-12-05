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

package pos

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"math/big"
)

type ConsensusInfo struct {
	MaxViolatePercentageAllowed uint64
	FetchNewValidatorsTime      uint64
	BlockReward                 *big.Int
	MaxValidators               uint64
	ConsensusPeriodInBlock      uint64
	MinimumStakes               *big.Int
	LockedPeriod                uint64
	Master                      MasterSmartContract
	Nodes                       Nodes
	Stakers                     Stakers
}

type MasterSmartContract struct {
	Address  common.Address
	ByteCode []byte
	ABI      string
	GenesisAmount *big.Int
}

type Nodes struct {
	ABI             string
	ByteCode        []byte
	GenesisInfo     []GenesisNodeInfo
}

type Stakers struct {
	ABI         string
	ByteCode    []byte
	GenesisInfo []GenesisStakeInfo
}

type GenesisNodeInfo struct {
	Address common.Address
	Owner   common.Address
	PubKey  string
	Name    string
	RewardPercentage  uint16
}

type GenesisStakeInfo struct {
	Address common.Address
	Owner   common.Address
	StakedNode common.Address
	LockedPeriod uint64
	StakeAmount *big.Int
}

