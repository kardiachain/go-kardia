// Package configs
package configs

import (
	"fmt"
	"testing"
)

func TestBootstrap_LoadConfig(t *testing.T) {
	cfg := LoadConfig()
	fmt.Printf("Default config: %+v \n", cfg)
	fmt.Println()
	fmt.Println()
	fmt.Printf("Default genesis config: %+v \n", cfg.Genesis)
	fmt.Println()
	fmt.Println()
	fmt.Printf("Default chain config: %+v \n", cfg.MainChain)
	//fmt.Printf("Default genesis config: %+v \n", cfg.Genesis)
}

func TestLoadConfig(t *testing.T) {
	chainCfg := loadDefaultMainnet()
	fmt.Printf("Default chain config: %+v \n", chainCfg)
}
