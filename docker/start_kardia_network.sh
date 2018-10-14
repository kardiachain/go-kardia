#!/bin/bash

NODES=6
PORT=3000
RPC_PORT=8545
IMAGE_NAME=kardiachain/go-kardia
ETH_NODE_INDEX=1
NEO_NODE_INDEX=2

if [ -z $1 ]; then
	echo "=> No number of nodes. Default number of nodes = 6"
else
    NODES=$1
	echo "=> The number of nodes = $NODES"
fi

if [ -z $2 ]; then
    echo "=> No eth dual was defined. Default eth dual node index = 1"
else
    ETH_NODE_INDEX=$2
fi


if [ -z $3 ]; then
   echo "=> No neo dual was defined. Default neo dual node index = 2"
else
   NEO_NODE_INDEX=$3
fi

# Remove container
docker ps -a | grep ${IMAGE_NAME} | awk '{print $1}' | xargs docker rm -f

# Start nodes
NODE_INDEX=1
while [ $NODE_INDEX -le $NODES ]
do
    if [ $NODE_INDEX -eq $ETH_NODE_INDEX ]; then
        mkdir -p ~/.kardiachain/node${NODE_INDEX}/data/ethereum
        docker run --rm -d --name node${NODE_INDEX} -v ~/.kardiachain/node${NODE_INDEX}/data/ethereum:/root/.ethereum --net=host $IMAGE_NAME --dev --numValid ${NODES} --dual --ethstat --ethstatname eth-dual-test-${NODE_INDEX} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --txn --clearDataDir
    elif [ $NODE_INDEX -eq $NEO_NODE_INDEX ]; then
        docker run --rm -d --name node${NODE_INDEX} --net=host $IMAGE_NAME --dev --numValid ${NODES} --neodual --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --clearDataDir
    else
        docker run --rm -d --name node${NODE_INDEX} --net=host $IMAGE_NAME --dev --numValid ${NODES} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --clearDataDir
    fi
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
done

echo "=> Started $NODES nodes."