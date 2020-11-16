# Go-Kardia

[![version](https://img.shields.io/github/release/qubyte/rubidium.svg)](https://github.com/kardiachain/go-kardiamain/releases/latest)
[![Go version](https://img.shields.io/badge/go-1.14-blue.svg)](https://github.com/moovweb/gvm)
[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)
[![CircleCI](https://circleci.com/gh/kardiachain/go-kardiamain.svg?style=shield&circle-token=b35bd6e6d67b307a6bb5966efbfa0297820d6846)](https://circleci.com/gh/kardiachain/go-kardiamain)
[![codecov](https://codecov.io/gh/kardiachain/go-kardiamain/branch/master/graph/badge.svg?token=VuisziC3mg)](https://codecov.io/gh/kardiachain/go-kardiamain)

Official Golang implementation of Kardia chain following the specs in [Technical Paper](http://dl.kardiachain.io/paper.pdf)

- Compatible tested `go build` version: 1.13.0, 1.13.9, 1.13.15, 1.14.12
- Compatible tested `go test ./...` version: 1.14.12

# Kardia private testnet
- Block explorer UI: [Kardiascan](http://explorer.kardiachain.io/)
- Release: [kardia-v0.10.4](https://github.com/kardiachain/go-kardiamain/releases/tag/v0.10.4)


# Quickstart
### Run local testnet with docker
- See [docker](https://github.com/kardiachain/go-kardiamain/tree/master/docker) for more details.

# Development
### Go environment setup
Install [Go](https://golang.org/doc/install) v1.14 to $HOME directory. Sets environment vars:  
> export GOPATH=$HOME/go  
> export PATH=$PATH:$GOPATH/bin

### Installation Prerequisites
* Install [libzmq](https://github.com/zeromq/libzmq) 

### Build
```
cd $GOPATH/src/github.com/kardiachain/go-kardiamain/cmd
go install
```

### Directory structure
Most of the top-level directories are self-explanatory. Here are the core directories:
* consensus - consensus engine
* config - default network configs
* dualchain - dual node's blockchain and service
* dualnode - interface layer to external blockchains, e.g. Ethererum, Neo, etc.
* kai - shared libraries specific to KardiaChain
* kvm - Kardia virtual machine
* lib - third-party libraries
* node - Node service
* rpc - RPC server
* mainchain - Kardia blockchain core and service
* proto - protobuf messages definition
* types - Kardia objects types


### Unit tests
```
cd $GOPATH/src/github.com/kardiachain/go-kardiamain
go test ./...
```

### Start Kardia network

####Mainnet
```
./cmd --network mainnet --node <path/to/kai_config.yaml>
```
####Testnet
```
./cmd --network testnet --node <path/to/kai_config_testnet.yaml>
```
###Devnet
```
./cmd --network devnet --node <path/to/kai_config_devnet_node1.yaml>
./cmd --network devnet --node <path/to/kai_config_devnet_node2.yaml>
./cmd --network devnet --node <path/to/kai_config_devnet_node3.yaml>
```

### Monitor blocks with Kardiascan
- Setup [JSON-RPC](https://github.com/kardiachain/go-kardiamain/tree/master/rpc) request
- Launch [Explorer backend](https://github.com/kardiachain/explorer-backend)
