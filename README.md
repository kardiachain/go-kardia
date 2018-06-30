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