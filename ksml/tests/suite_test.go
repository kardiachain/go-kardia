// Package tests
package tests

import (
	"github.com/kardiachain/go-kardiamain/configs"
)

func init() {
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
}
