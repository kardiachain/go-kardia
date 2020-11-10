// Package _default
package _default

import (
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

var (
	// Chain configuration
	serviceName        = "KARDIA"
	chainID     uint64 = 1
	networkID   uint64 = 100
	acceptTxs   uint32 = 1
	zeroFee     uint   = 0

	period uint64 = 15
	epoch  uint64 = 30000

	chainDBDir     = "chaindata"
	chainDBCache   = 16
	chainDBHandles = 32
	chainDBDrop    = 1

	seeds = []string{
		"c1fe56e3f58d3244f606306611a5d10c8333f1f6@127.0.0.1:3000",
		"7cefc13b6e2aedeedfb7cb6c32457240746baee5@127.0.0.1:3001",
		"ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd@127.0.0.1:3002",
	}
)

var kaiConn = &typesCfg.KaiconConfig{
	Period: period,
	Epoch:  epoch,
}

var database = &typesCfg.Database{
	Dir:     chainDBDir,
	Caches:  chainDBCache,
	Handles: chainDBHandles,
	Drop:    chainDBDrop,
}

func Chain() *typesCfg.Chain {
	return &typesCfg.Chain{
		ServiceName: serviceName,
		ChainID:     chainID,
		NetworkID:   networkID,
		AcceptTxs:   acceptTxs,
		ZeroFee:     zeroFee,
		Database:    database,
		Seeds:       seeds,
		Genesis:     Genesis(),
	}
}

func ChainDB() *typesCfg.Database {
	return database
}

func ChainConfig() *typesCfg.ChainConfig {
	return &typesCfg.ChainConfig{
		Kaicon: kaiConn,
	}
}

func KaiConn() *typesCfg.KaiconConfig {
	return kaiConn
}
