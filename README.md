# Go Kardia

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
Run two nodes in different ports and use enode url to connect.  
First terminal
```
./go-kardia --addr :3000 --name node1
```
Second terminal, set peer args as the enode url displayed in first terminal
```
./go-kardia --addr :30001 --name node2 --peer enode://4b7f6c7274881a6c7fd3068c1a147d3e9d003a964c3e3490814942dd7cbb975e0424db335881962239dd8170a9cc5b09a9f4c81babd57ac10df0d6465a58dd67@[::]:3000
```
