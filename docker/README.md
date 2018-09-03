# Prerequisites
Install Docker following the installation guide for your platform: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)

# Test docker environment 

Build docker image: 

```
docker build -t kardiachain/go-kardia ../
```

Example, 3-nodes network: 

```
./start_kardia_network.sh {NUMBER_OF_NODES}
```

or run in terminal

```
docker run --rm -d --name node1 --net="host" kardiachain/go-kardia --dev --numValid 3 --addr :3000 --name node1 --rpc --rpcport 8545 --txn --clearDataDir
docker run --rm -d --name node2 --net="host" kardiachain/go-kardia --dev --numValid 3 --addr :3001 --name node2 --rpc --rpcport 8546 --txn --clearDataDir
docker run --rm -d --name node3 --net="host" kardiachain/go-kardia --dev --numValid 3 --addr :3002 --name node3 --rpc --rpcport 8547 --txn --clearDataDir
```

