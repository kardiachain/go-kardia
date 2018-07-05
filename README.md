# Concept Chain

Golang implementation of concepts in a blockchain

# Installs
### Solve current build problem
Downloads missing C files in full source to vendor directory  
```
cd $GOPATH/src
go get github.com/ethereum/go-ethereum
cp -r github.com/ethereum/go-ethereum/crypto/secp256k1/ conceptchain/vendor/github.com/ethereum/go-ethereum/crypto/
```

# Run
```
cd $GOPATH/src/conceptchain
go install
cd $GOPATH/bin
./conceptchain
```

# Test p2p connection
Run two nodes in different ports and use enode url to connect.  
First terminal
```
./conceptchain --addr :3000 --name node1
```
Second terminal, set peer args as the enode url displayed in first terminal
```
./conceptchain --addr :30001 --name node2 --peer enode://4b7f6c7274881a6c7fd3068c1a147d3e9d003a964c3e3490814942dd7cbb975e0424db335881962239dd8170a9cc5b09a9f4c81babd57ac10df0d6465a58dd67@[::]:3000
```
