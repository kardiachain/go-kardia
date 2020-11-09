// Package main
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

var (
	mainnetDefaultCfg        = Config{}
	mainnetCfgWithCustomNode = Config{
		Node:      typesCfg.Node{},
		MainChain: nil,
		DualChain: nil,
	}
	mainnetCfgWithCustomGenesis = Config{}

	testnetDefaultCfg        = Config{}
	testnetCfgWithCustomNode = Config{
		Node:      typesCfg.Node{},
		MainChain: nil,
		DualChain: nil,
	}
	testnetCfgWithCustomGenesis = Config{}

	devnetDefaultCfg        = Config{}
	devnetCfgWithCustomNode = Config{
		Node:      typesCfg.Node{},
		MainChain: nil,
		DualChain: nil,
	}
	devnetCfgWithCustomGenesis = Config{}
)

type (
	loadConfigInput struct {
		Name     string
		Flags    flags
		Expected loadConfigOutput
	}
	loadConfigOutput struct {
		Cfg Config
		Err error
	}
)

func TestLoadConfig_Mainnet(t *testing.T) {
	cases := []loadConfigInput{
		{
			Name: "Flag default",
			Flags: flags{
				genesis: "",
				kardia:  "",
				dual:    "",
				network: "",
			},
		},
		{
			Name:  "Flag mainnet with custom genesis",
			Flags: flags{},
			Expected: loadConfigOutput{
				Cfg: Config{},
				Err: nil,
			},
		},
		{
			Name:  "Flag mainnet with custom & missing genesis file",
			Flags: flags{},
			Expected: loadConfigOutput{
				Cfg: Config{},
				Err: nil,
			},
		},
		{
			Name:  "Flag mainnet with custom & wrong genesis format",
			Flags: flags{},
			Expected: loadConfigOutput{
				Cfg: Config{},
				Err: nil,
			},
		},
		{
			Name: "Flag mainnet with custom node",
		},
		{
			Name: "Flag mainnet with custom & missing node file",
		},
		{
			Name: "Flag mainnet with custom & wrong node",
		},
		{
			Name: "Flag mainnet with custom node & genesis",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			cfg, err := LoadConfig(c.Flags)
			assert.Equal(t, c.Expected.Err, err, "Error not match")
			assert.Equal(t, c.Expected.Cfg, cfg, "")
		})
	}
}

func TestLoadConfig_Testnet(t *testing.T) {

}

func TestLoadConfig_Devnet(t *testing.T) {

}

//region Config unit test

//endregion Config unit test
