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
./conceptchain --addr :3000
```
Second terminal
```
 ./conceptchain --addr :30001 --peer enode://1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439@127.0.0.1:3000
```
