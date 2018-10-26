# Go-Kardia

[![version](https://img.shields.io/github/release/qubyte/rubidium.svg)](https://github.com/kardiachain/go-kardia/releases/latest)
[![Go version](https://img.shields.io/badge/go-1.10.4-blue.svg)](https://github.com/moovweb/gvm)
[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)
[![CircleCI](https://circleci.com/gh/kardiachain/go-kardia.svg?style=shield&circle-token=3163b86cadff994c8e322dc3aedf57c61f541c42)](https://circleci.com/gh/kardiachain/go-kardia)
[![codecov](https://codecov.io/gh/kardiachain/go-kardia/branch/master/graph/badge.svg?token=9HzVclw3dp)](https://codecov.io/gh/kardiachain/go-kardia)

Official Golang implementation of Kardia chain following the specs in [Technical Paper](http://dl.kardiachain.io/paper.pdf)

# Kardia private testnet
- Block explorer UI: [Kardiascan](http://scan.kardiachain.io/)
- Release: [kardia-milestone3-20181001](https://github.com/kardiachain/go-kardia/releases/tag/kardia-milestone3-20181001)


# Quickstart
### One-command-deployment to supported cloud providers
Copy and execute the below script to create a cloud VM and start Kardia testnet without checking out the source code. See [deployment](https://github.com/kardiachain/go-kardia/tree/master/deployment) for more details.  
- **Google GCP**  
[`./gce_deploy_testnet.sh`](https://github.com/kardiachain/go-kardia/blob/master/deployment/gce_deploy_testnet.sh)   
- **Amazon AWS**  
[`./aws_deploy_testnet.sh`](https://github.com/kardiachain/go-kardia/blob/master/deployment/aws_deploy_testnet.sh)

### Run local testnet with docker
- See [docker](https://github.com/kardiachain/go-kardia/tree/master/docker) for more details.

### Monitor blocks with Kardiascan
- Setup [JSON-RPC](https://github.com/kardiachain/go-kardia/tree/master/rpc) request
- Update config to [Kardiascan config](https://github.com/kardiachain/KardiaScan#update-node-config)
- Launch [Kardiascan](https://github.com/kardiachain/KardiaScan#run-development-mode)

# Development
### Go environment setup
Install [Go](https://golang.org/doc/install) v1.10 to $HOME directory. Sets environment vars:  
> export GOPATH=$HOME/go  
> export PATH=$PATH:$GOPATH/bin

### Build
Install [dep](https://github.com/golang/dep) v0.5 tool for dependency management.  
Download library dependency and build Kardia binary.
```
cd $GOPATH/src/github.com/kardiachain/go-kardia
dep ensure
go install
```

### Unit tests
```
cd $GOPATH/src/github.com/kardiachain/go-kardia
go test ./...
```

### Start Kardia node
```
./go-kardia --dualchain --dev --mainChainValIndex  1 --addr :3000 --name node1 --rpc --rpcport 8545 --clearDataDir
```
