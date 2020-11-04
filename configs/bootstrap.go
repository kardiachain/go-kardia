// Package configs
package configs

import (
	_default "github.com/kardiachain/go-kardiamain/configs/default"
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
)

func LoadConfig() *typesCfg.Config {
	cfg := loadDefaultMainnet()
	return cfg
}

//region mainnet loader
func loadMainnetConfig(cfgData []byte) {

}

func loadMainConfigFromPath() {

}

func loadMainConfigFromFiles() {

}

func loadDefaultMainnet() *typesCfg.Config {
	defaultGenesisCfg := &typesCfg.Genesis{
		Addresses:     _default.Addresses,
		GenesisAmount: _default.GenesisAmount,
		Contracts:     _default.Contracts,
		Validators:    _default.Validators,
	}
	chainCfg := &typesCfg.Chain{
		Protocol:           nil,
		ChainID:            _default.ChainID,
		NetworkID:          _default.NetworkID,
		AcceptTxs:          _default.AcceptTxs,
		ZeroFee:            _default.ZeroFee,
		Genesis:            defaultGenesisCfg,
		Database:           _default.Database,
		Seeds:              nil,
		Events:             nil,
		PublishedEndpoint:  nil,
		SubscribedEndpoint: nil,
		BaseAccount:        typesCfg.BaseAccount{},
		Consensus:          nil,
	}
	cfg := &typesCfg.Config{
		MainChain: chainCfg,
	}
	cfg.Node = typesCfg.Node{
		P2P: typesCfg.P2P{
			ListenAddress: _default.P2PListenAddress,
			PrivateKey:    _default.P2PPrivateKey,
			MaxPeers:      _default.P2PMaxPeers,
		},
		Name:             _default.NodeName,
		LogLevel:         _default.LogLevel,
		DataDir:          _default.DataDir,
		HTTPHost:         _default.HTTPHost,
		HTTPPort:         _default.HTTPPort,
		HTTPModules:      _default.HTTPModules,
		HTTPVirtualHosts: _default.HTTPVirtualHost,
		HTTPCors:         _default.HTTPCors,
		Metrics:          _default.Metrics,
		Genesis:          defaultGenesisCfg,
	}
	return cfg
}

//endregion mainnet loader

//region testnet loader
//endregion testnet loader

//region devnet loader
//endregion devnet loader
