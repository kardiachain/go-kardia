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
Runs two nodes in different ports and use enode url to connect.  
Uses `txn` flag in one node to create a sample transaction and sees the node sync the transaction in debug logging.  
First terminal. Note: you would need to customize the number of validators via `numValid`.
```
./go-kardia --dev --numValid 2 --addr :3000 --name node1 --txn --clearDataDir
```
Second terminal. Note that the peer node is fixed when running with --dev setting.
```
./go-kardia --dev --numValid 2 --addr :30001 --name node2 --clearDataDir --peer enode://724fbdc7067814bdd60315d836f28175ff9c72e4e1d86513a2b578f9cd769e688d6337550778b89e4861a42580613f1f1dec23f17f7a1627aa99104cc4204eb1@[::]:3000
```
# Test dual node
Runs node with `dual` flag to start dual mode, acting as a full node syncing on both Kardia network and Ethereum Rinkeby testnet.  
Dual node may use 10GB+ storage.
```
./go-kardia --dual --name node1
```
