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
First terminal
```
./go-kardia --addr :3000 --name node1 --txn
```
Second terminal, set peer args as the enode url displayed in first terminal
```
./go-kardia --addr :30001 --name node2 --peer enode://4b7f6c7274881a6c7fd3068c1a147d3e9d003a964c3e3490814942dd7cbb975e0424db335881962239dd8170a9cc5b09a9f4c81babd57ac10df0d6465a58dd67@[::]:3000
```
# Test dual node
Runs node with `dual` flag to start dual mode, acting as a full node syncing on both Kardia network and Ethereum Rinkeby testnet.  
Dual node may use 10GB+ storage.
```
./go-kardia --dual --name node1
```
