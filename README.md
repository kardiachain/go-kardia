# Go-Kardia

[![version](https://img.shields.io/github/release/qubyte/rubidium.svg)](https://github.com/kardiachain/go-kardiamain/releases/latest)
[![Go version](https://img.shields.io/badge/go-1.13-blue.svg)](https://github.com/moovweb/gvm)
[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)
[![CircleCI](https://circleci.com/gh/kardiachain/go-kardiamain.svg?style=shield&circle-token=b35bd6e6d67b307a6bb5966efbfa0297820d6846)](https://circleci.com/gh/kardiachain/go-kardiamain)
[![codecov](https://codecov.io/gh/kardiachain/go-kardiamain/branch/master/graph/badge.svg?token=VuisziC3mg)](https://codecov.io/gh/kardiachain/go-kardiamain)

Official Golang implementation of Kardia chain following the specs in [Technical Paper](http://dl.kardiachain.io/paper.pdf)

Compatible tested Go version: 1.13.0, 1.13.9, 1.13.15

# Kardia private testnet
- Block explorer UI: [Kardiascan](http://explorer.kardiachain.io/)
- Release: [kardia-v0.9.0](https://github.com/kardiachain/go-kardiamain/releases/tag/v0.9.0)


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
Install [Go](https://golang.org/doc/install) v1.13 to $HOME directory. Sets environment vars:  
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
* dev - configs that can be enabled in dev environment/runtime to mock different behaviors seen in real decentralized nodes, such as: malicious nodes, crashed nodes, etc. It can even mock block generation of external chain to speed up development.
* dualchain - dual node's blockchain and service
* dualnode - interface layer to external blockchains, e.g. Ethererum, Neo, etc.
* kai - shared libraries specific to Kardia
* kvm - Kardia's virtual machine
* lib - 3rd party libraries
* mainchain - main Kardia's blockchain and service

### Unit tests
```
cd $GOPATH/src/github.com/kardiachain/go-kardiamain
go test ./...
```

### Start Kardia node
```
./cmd --config <path/to/kai_eth_config_1.yaml>
./cmd --config <path/to/kai_eth_config_2.yaml>
./cmd --config <path/to/kai_eth_config_3.yaml>
```

### Monitor blocks with Kardiascan
- Setup [JSON-RPC](https://github.com/kardiachain/go-kardia/tree/master/rpc) request
- Update config to [Kardiascan config](https://github.com/kardiachain/KardiaScan#update-node-config)
- Launch [Kardiascan](https://github.com/kardiachain/KardiaScan#run-development-mode)
