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
Kardia executable is in Go binary directory. If $PATH is setup as above, go-kardia can be run on any paths.
```
cd $GOPATH/bin
./go-kardia
```
# Test consensus with multiple nodes
Important:
  - Always include `dev` flag in test p2p. Peer address are fixed when running in dev settings.
  - Node name starts from `node1`, `node2`, etc.
  - Port number starts from `3000` for `node1`, `3001` for `node2`, and so on.
`numValid` required flag for number of validators, set to the number of nodes you plan to run.   
`genNewTxs` optional flag to routinely adds transfer transactions between genesis accounts.  
`txn` optional flag instead of `genNewTxs` to add one transfer transaction when node starts.
  
Example, 3-nodes network:  
First terminal:
```
./go-kardia --dev --numValid 3 --addr :3000 --name node1 --txn --clearDataDir
```
Second terminal:
```
./go-kardia --dev --numValid 3 --addr :3001 --name node2 --clearDataDir
```
Third terminal:
```
./go-kardia --dev --numValid 3 --addr :3002 --name node3 --clearDataDir
```

# Test JSON-RPC API request
The default address of the rpc server is http://localhost:8545

Runs the node with `--rpc` flag:
```
./go-kardia --dev --numValid 2 --addr :3000 --name node1 --txn --clearDataDir --rpc
```

Send a json-rpc request to the node using curl:
```
curl -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"kai_blockNumber","params":[],"id":1}' localhost:8545
```
The response will be in the following form:
```
{"jsonrpc":"2.0","id":1,"result":100}
```

Runs node with `dual` flag to start dual mode, acting as a full node syncing on both Kardia network and Ethereum Rinkeby testnet.  
Dual node may use 10GB+ storage and needs SSD storage.
```
./go-kardia --dual --name node1
```
