# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

# Test docker environment 

Build docker image: 

```
cd <path/to/go-kardia>
docker build -t kardiachain/go-kardia .
```

Example, 3 Kardia nodes: 
```
./start_local_network.sh -n 3
```
or run in terminal

```
mkdir -p ~/.kardia/node1/data/db
mkdir -p ~/.kardia/node2/data/db
mkdir -p ~/.kardia/node3/data/db
docker run --rm -d --name node1 --log-opt max-size=300m -v ~/.kardia/node1/data/db:/root/.kardia/node1 --net="host" kardiachain/go-kardia --dualchain --dev --mainChainValIndexes 1,2,3 --addr :3000 --name node1 --rpc --rpcport 8545 --clearDataDir
docker run --rm -d --name node2 --log-opt max-size=300m -v ~/.kardia/node2/data/db:/root/.kardia/node2 --net="host" kardiachain/go-kardia --dualchain --dev --mainChainValIndexes 1,2,3 --addr :3001 --name node2 --rpc --rpcport 8546 --clearDataDir
docker run --rm -d --name node3 --log-opt max-size=300m -v ~/.kardia/node3/data/db:/root/.kardia/node3 --net="host" kardiachain/go-kardia --dualchain --dev --mainChainValIndexes 1,2,3 --addr :3002 --name node3 --rpc --rpcport 8547 --clearDataDir
```

