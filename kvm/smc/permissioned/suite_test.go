// Package kvm
package kvm

import (
	"github.com/kardiachain/go-kardiamain/configs"
)

func init() {
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
}
