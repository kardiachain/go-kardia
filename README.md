# Go-Kardia

[![version](https://img.shields.io/github/release/qubyte/rubidium.svg)](https://github.com/kardiachain/go-kardia/releases/latest)
[![Go version](https://img.shields.io/badge/go-1.12.9-blue.svg)](https://github.com/moovweb/gvm)
[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)
[![CircleCI](https://circleci.com/gh/kardiachain/go-kardia.svg?style=shield&circle-token=3163b86cadff994c8e322dc3aedf57c61f541c42)](https://circleci.com/gh/kardiachain/go-kardia)
[![codecov](https://codecov.io/gh/kardiachain/go-kardia/branch/master/graph/badge.svg?token=9HzVclw3dp)](https://codecov.io/gh/kardiachain/go-kardia)

Official Golang implementation of Kardia chain following the specs in [Technical Paper](http://dl.kardiachain.io/paper.pdf)

# Kardia private testnet
- Block explorer UI: [Kardiascan](http://scan.kardiachain.io/)
- Release: [kardia-v0.8.0](https://github.com/kardiachain/go-kardia/releases/tag/v0.8.0)


# Quickstart
## 1. One-command-deployment to join Kardia private testnet:
Copy and execute below script to create a VM running latest Kardia node and join our Kardia testnet:
- **Google GCP**  
[`./gce_join_testnet.sh`](https://github.com/kardiachain/go-kardia/blob/master/deployment/gce_join_testnet.sh)  

## 2. One-command-deployment to create Kardia private testnet:
Copy and execute below script to start Kardia testnet without checking out the source code. (Note: Kardiascan UI is included in deployment script)
### Supported cloud providers:
- **Google GCP**  
[`./gce_deploy_testnet.sh`](https://github.com/kardiachain/go-kardia/blob/master/deployment/gce_deploy_testnet.sh)   
- **Amazon AWS**  
[`./aws_deploy_single_machine_testnet.sh`](https://github.com/kardiachain/go-kardia/blob/master/deployment/aws_deploy_single_machine_testnet.sh)   

See [deployment](https://github.com/kardiachain/go-kardia/tree/master/deployment) for more details.  

### Run local testnet with docker
- See [docker](https://github.com/kardiachain/go-kardia/tree/master/docker) for more details.

# Development
### Go environment setup
Install [Go](https://golang.org/doc/install) v1.12 to $HOME directory. Sets environment vars:  
> export GOPATH=$HOME/go  
> export PATH=$PATH:$GOPATH/bin

### Installation Prerequisites
* Install [libzmq](https://github.com/zeromq/libzmq) 

### Build
Install [dep](https://github.com/golang/dep) v0.5 tool for dependency management.  
Download library dependency and build Kardia binary.
```
cd $GOPATH/src/github.com/kardiachain/go-kardia
dep ensure
go install
```

### Directory structure
Most of the top-level directories are self-explanatory. Here are the core directories:
* consensus - consensus engine
* dev - configs that can be enabled in dev environment/runtime to mock different behaviors seen in real decentralized nodes, such as: malicious nodes, crashed nodes, etc. It can even mock block generation of external chain to speed up development.
* dualchain - dual node's blockchain and service
* dualnode - interface layer to external blockchains, e.g. Ethererum, Neo, etc.
* kai - shared libraries specific to Kardia
* kvm - Kardia's virtual machine
* lib - 3rd party libraries
* mainchain - main Kardia's blockchain and service

### Unit tests
```
cd $GOPATH/src/github.com/kardiachain/go-kardia
go test ./...
```

### Start Kardia node
```
./go-kardia --dev --mainChainValIndexes 1,2,3 --addr :3000 --name node1 --rpc --rpcport 8545 --clearDataDir --peer enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@127.0.0.1:3000,enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@127.0.0.1:3001,enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@127.0.0.1:3002
./go-kardia --dev --mainChainValIndexes 1,2,3 --addr :3001 --name node2 --rpc --rpcport 8546 --clearDataDir --peer enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@127.0.0.1:3000,enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@127.0.0.1:3001,enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@127.0.0.1:3002
./go-kardia --dev --mainChainValIndexes 1,2,3 --addr :3002 --name node3 --rpc --rpcport 8547 --clearDataDir --peer enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@127.0.0.1:3000,enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@127.0.0.1:3001,enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@127.0.0.1:3002
```

### Monitor blocks with Kardiascan
- Setup [JSON-RPC](https://github.com/kardiachain/go-kardia/tree/master/rpc) request
- Update config to [Kardiascan config](https://github.com/kardiachain/KardiaScan#update-node-config)
- Launch [Kardiascan](https://github.com/kardiachain/KardiaScan#run-development-mode)
