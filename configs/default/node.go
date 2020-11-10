// Package _default
package _default

import (
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

// Node configuration
var (
	name        = "default"
	dataDir     = "/tmp/.kardia"
	httpHost    = "0.0.0.0"
	httpPort    = 8545
	httpModules = []string{
		"node",
		"kai",
		"tx",
		"account",
		"dual",
		"neo",
	}
	httpVirtualHost = []string{
		"0.0.0.0",
		"localhost",
	}
	httpCors = []string{
		"*",
	}
	logLevel      = "info"
	metrics  uint = 0
)

// P2P config
var (
	p2pPrivateKey    = "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06"
	p2pListenAddress = "tcp://0.0.0.0:3000"
	//p2pMaxPeers      uint = 25
)

var p2p = typesCfg.P2P{
	ListenAddress: p2pListenAddress,
	PrivateKey:    p2pPrivateKey,
}

var node = typesCfg.Node{
	P2P:              p2p,
	LogLevel:         logLevel,
	Name:             name,
	DataDir:          dataDir,
	HTTPHost:         httpHost,
	HTTPPort:         httpPort,
	HTTPModules:      httpModules,
	HTTPVirtualHosts: httpVirtualHost,
	HTTPCors:         httpCors,
	Metrics:          metrics,
	Genesis:          Genesis(),
}

func Node() typesCfg.Node {
	return node
}

func P2P() typesCfg.P2P {
	return p2p
}
