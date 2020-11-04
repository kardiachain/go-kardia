// Package _default
package _default

import (
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

// Node configuration
var (
	NodeName    = "defaultNodeName"
	DataDir     = "/tmp/.kardia"
	HTTPHost    = "0.0.0.0"
	HTTPPort    = 8545
	HTTPModules = []string{
		"node",
		"kai",
		"tx",
		"account",
		"dual",
		"neo",
	}
	HTTPVirtualHost = []string{
		"0.0.0.0",
		"localhost",
	}
	HTTPCors = []string{
		"*",
	}
	LogLevel      = "info"
	Metrics  uint = 0

	// Chain configuration
	ChainID   uint64 = 1
	NetworkID uint64 = 100
	AcceptTxs uint32 = 1
	ZeroFee   uint   = 0
	Database         = &typesCfg.Database{
		Dir:     "chaindata",
		Caches:  16,
		Handles: 32,
		Drop:    1,
	}
)

// P2P config
var (
	P2PPrivateKey         = "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2ddefault"
	P2PListenAddress      = "tcp://0.0.0.0:3000"
	P2PMaxPeers      uint = 25
)
