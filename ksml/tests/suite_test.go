// Package tests
package tests

import (
	"github.com/kardiachain/go-kardia/configs"
)

func init() {
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
}
