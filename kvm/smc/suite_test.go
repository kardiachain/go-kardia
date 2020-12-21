// Package kvm
package kvm

import (
	"github.com/kardiachain/go-kardia/configs"
)

func init() {
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
}
