// Package configs
// Config use priority and override/extend configuration strategy. Load configuration following by below steps:
// 1. Read all hardcode configuration for default, user must able to run network without any config files.
// 2. Try to detect target network, and load by all configuration by priority. Default is default
// 3. Config will load all configuration of higher priority network and override if those config exist in lower priority
// Network priority
//    Mainnet : p0
//    Testnet : p1
//    Devnet : p3
// For example:
// 1. Run default by following commands
//    ./cmd
// Will load all hard configuration in config/default folder
// Then find config files with input flag args and override if exist
// 2. Run devnet by following commands
//    ./cmd --network devnet
// Will load all hard configuration in config/default folder
// Then find default config files for mainnet (p0) and override config if exist
// Then find default config files for testnet (p1) and override config if exist
// Then find configs files with input flag args and override if exist
// Folder structure
// ./default contain all mainnet default configuration, which allow network run-able with minimum config (HTTPPort, P2PPrivateKey, P2PPort or Seeds)
package configs
