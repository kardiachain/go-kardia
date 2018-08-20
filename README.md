# Go-Kardia

Golang implementation of concepts for Kardia chain

# Setup & build
### Go environment setup
Installs [Go](https://golang.org/doc/install) to $HOME directory. Sets environment vars:  
> export GOPATH=$HOME/go  
> export PATH=$PATH:$GOPATH/bin

### Build
Installs [dep](https://github.com/golang/dep) tool for dependency management.  
Downloads library dependency and builds Kardia binary.
```
cd $GOPATH/src/github.com/kardiachain/go-kardia
dep ensure
go install
```
# Run
Kardia executable is in Go binary directory. If $PATH is setup as above, go-karida can be run on any paths.
```
cd $GOPATH/bin
./go-kardia
```
# Test p2p connection
Important:
  - Always include `dev` flag in test p2p.
  - Node name starts from `node1`, `node2`, etc.
  - Port number starts from `3000` for `node1`, `3001` for `node2`, and so on.

Runs two nodes in different ports and use enode url to connect.  
Uses `txn` flag in one node to create a sample transaction and sees the node sync the transaction in debug logging.  
First terminal. Note: you would need to customize the number of validators via `numValid`.
```
./go-kardia --dev --numValid 2 --addr :3000 --name node1 --txn --clearDataDir
```
Second terminal. Note that the peer node is fixed when running with --dev setting.
```
./go-kardia --dev --numValid 2 --addr :30001 --name node2 --clearDataDir
```
# Test multiple nodes
`numValid` flag must be set to the total number of nodes you plan to run. For example, in a 3-node scenario:
First terminal:
```
./go-kardia --dev --numValid 3 --addr :3000 --name node1 --txn --clearDataDir
```
Second terminal:
```
./go-kardia --dev --numValid 3 --addr :3001 --name node2 --txn --clearDataDir
```
Third terminal:
```
./go-kardia --dev --numValid 3 --addr :3002 --name node3 --txn --clearDataDir
```

# Test dual node
Runs node with `dual` flag to start dual mode, acting as a full node syncing on both Kardia network and Ethereum Rinkeby testnet.  
Dual node may use 10GB+ storage.
```
./go-kardia --dual --name node1
```
