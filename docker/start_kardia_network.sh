#!/bin/bash

NODES=3
PORT=3000
RPC_PORT=8545
IMAGE_NAME=kardiachain/go-kardia

if [ -z $1 ]; then
	echo "=> No number of nodes. Default number of nodes = 3"
else
    NODES=$1
	echo "=> The number of nodes = $NODES"
fi

# Remove container
docker ps -a | grep $IMAGE_NAME | awk '{print $1}' | xargs docker rm -f

# Start nodes
NODE_INDEX=1
while [ $NODE_INDEX -le $NODES ]
do
    docker run --rm -d --name node$NODE_INDEX --net=host $IMAGE_NAME --dev --numValid $NODES --addr :$PORT --name node$NODE_INDEX --rpc --rpcport $RPC_PORT --txn --clearDataDir
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
done

echo "=> Started $NODES nodes."