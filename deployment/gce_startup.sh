#!/bin/bash

IMAGE_NAME=gcr.io/strategic-ivy-130823/go-kardia:milestone3
NODES=3

ETH_NODE_INDEX=1
PORT=3000
RPC_PORT=8545

# Pull go-kardia docker from Google Registry
docker pull $IMAGE_NAME

docker ps -a | grep ${IMAGE_NAME} | awk '{print $1}' | xargs docker rm -f

# Run the nodes
NODE_INDEX=1
while [ $NODE_INDEX -le $NODES ]
do
    if [ $NODE_INDEX -eq $ETH_NODE_INDEX ]; then
        mkdir -p /var/kardiachain/node${NODE_INDEX}/ethereum
        docker run -d --name node${NODE_INDEX} -v /var/kardiachain/node${NODE_INDEX}/ethereum:/root/.ethereum --net=host $IMAGE_NAME --dev --numValid ${NODES} --ethstat --ethstatname eth-dual-test-${NODE_INDEX} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --txn --clearDataDir
    else
        docker run -d --name node${NODE_INDEX} --net=host $IMAGE_NAME --dev --numValid ${NODES} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --clearDataDir
    fi
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
done

echo "=> Started $NODES nodes."
