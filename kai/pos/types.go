package pos

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"math/big"
)

type ConsensusInfo struct {
	FetchNewValidators   uint64
	BlockReward          *big.Int
	MaxValidators        uint64
	ConsensusPeriod      uint64
	MinimumStakes        *big.Int
	Master               MasterSmartContract
	Nodes                Nodes
	Stakers              Stakers
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
	Host    string
	Port    string
	Reward  uint16
}

type GenesisStakeInfo struct {
	Address common.Address
	Owner   common.Address
	StakedNode common.Address
	LockedPeriod uint64
	StakeAmount *big.Int
}

