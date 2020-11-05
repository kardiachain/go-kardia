// Package configs
package configs

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	chainCfg := loadDefaultMainnet()
	fmt.Printf("Default chain config: %+v \n", chainCfg)
}
