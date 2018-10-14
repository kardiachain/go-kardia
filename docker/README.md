# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

# Test docker environment 

Build docker image: 

```
docker build -t kardiachain/go-kardia ../
```

Example, 6-nodes network with ETH dual node is node1, NEO dual node is node2

```
./start_kardia_network.sh -n {NUMBER_OF_NODES} -eth {ETH_NODE_INDEX} -neo {NEO_NODE_INDEX} -dual
```

```
./start_kardia_network.sh -n 6 -eth 1 -neo 2 -dual
```

or run in terminal

```
mkdir -p ~/.kardiachain/node1/data/ethereum
docker run --rm -d --name node1 -v ~/.kardiachain/node1/data/ethereum:/root/.ethereum --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --dual --ethstat --ethstatname eth-dual-test-1 --addr :3000 --name node1 --rpc --rpcport 8545 --txn --clearDataDir
docker run --rm -d --name node2 --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --neodual --addr :3001 --name node2 --rpc --rpcport 8546 --clearDataDir
docker run --rm -d --name node3 --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --addr :3002 --name node3 --rpc --rpcport 8547 --clearDataDir
docker run --rm -d --name node4 --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --addr :3003 --name node4 --rpc --rpcport 8548 --clearDataDir
docker run --rm -d --name node5 --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --addr :3004 --name node5 --rpc --rpcport 8549 --clearDataDir
docker run --rm -d --name node6 --net="host" kardiachain/go-kardia --dualchain --dev --numValid 6 --addr :3005 --name node6 --rpc --rpcport 8550 --clearDataDir
```

