// Package _default
package _default

import (
	"github.com/kardiachain/go-kardiamain/configs/contracts"
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

func StakingContracts() typesCfg.Contract {
	return contracts.Contracts[contracts.StakingSMCKey]
}

func Contracts() map[string]typesCfg.Contract {
	return contracts.Contracts
}
