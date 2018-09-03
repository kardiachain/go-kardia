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
See the [consensus](https://github.com/kardiachain/go-kardia/tree/master/consensus) for more information.

# Test JSON-RPC API request
See the [rpc](https://github.com/kardiachain/go-kardia/tree/master/rpc) for more information.

# Test dual node
See the [dual](https://github.com/kardiachain/go-kardia/tree/master/dual) for more information.

# Test docker environment
See the [docker](https://github.com/kardiachain/go-kardia/tree/master/docker) for more information.

# Test Kardia Virtual Machine
See the [vm/sample_kvm](https://github.com/kardiachain/go-kardia/tree/master/vm/sample_kvm) for more information. 