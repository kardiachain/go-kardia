#!/bin/bash

NODES=6
PORT=3000
RPC_PORT=8545
IMAGE_NAME=kardiachain/go-kardia
ETH_NODE_INDEX=1
NEO_NODE_INDEX=2
PACKAGE=start_kardia_network.sh
DUALCHAIN=

while test $# -gt 0; do
        case "$1" in
                -h|--help)
                        echo "$PACKAGE - Starts a Kardia network"
                        echo " "
                        echo "$PACKAGE [options]"
                        echo " "
                        echo "options:"
                        echo "-h, --help                      Show brief help"
                        echo "-n, --num_nodes=NUMBER          Specify how many nodes to bring up"
                        echo "-eth, --ether_node_index=INDEX  Index of ether node"
                        echo "-neo, --neo_node_index=INDEX    Index of neo node"
                        echo "-dual, --dual_chain             Enable running dual chains"
                        exit 0
                        ;;
                -n|--num_nodes)
                        shift
                        if test $# -gt 0; then
                                NODES=$1
                        else
                                echo "Number of nodes not specified"
                                exit 1
                        fi
                        shift
                        ;;
                -eth|--ether_node_index)
                        shift
                        if test $# -gt 0; then
                                ETH_NODE_INDEX=$1
                        else
                                echo "Ether node index not specified"
                                exit 1
                        fi
                        shift
                        ;;
                -neo|--neo_node_index)
                        shift
                        if test $# -gt 0; then
                                NEO_NODE_INDEX=$1
                        else
                                echo "Neo node index not specified"
                                exit 1
                        fi
                        shift
                        ;;
                -dual)
                        DUALCHAIN=--dualchain
                        shift
                        ;;
                *)
                        break
                        ;;
        esac
done

# Remove container
docker ps -a | grep ${IMAGE_NAME} | awk '{print $1}' | xargs docker rm -f

# Start nodes
NODE_INDEX=1
while [ $NODE_INDEX -le $NODES ]
do
    if [ $NODE_INDEX -eq $ETH_NODE_INDEX ]; then
        mkdir -p ~/.kardiachain/node${NODE_INDEX}/data/ethereum
        docker run --rm -d --name node${NODE_INDEX} -v ~/.kardiachain/node${NODE_INDEX}/data/ethereum:/root/.ethereum --net=host $IMAGE_NAME $DUALCHAIN --dev --numValid ${NODES} --dual --ethstat --ethstatname eth-dual-test-${NODE_INDEX} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --txn --clearDataDir
    elif [ $NODE_INDEX -eq $NEO_NODE_INDEX ]; then
        docker run --rm -d --name node${NODE_INDEX} --net=host $IMAGE_NAME $DUALCHAIN --dev --numValid ${NODES} --neodual --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --clearDataDir
    else
        docker run --rm -d --name node${NODE_INDEX} --net=host $IMAGE_NAME $DUALCHAIN --dev --numValid ${NODES} --addr :${PORT} --name node${NODE_INDEX} --rpc --rpcport ${RPC_PORT} --clearDataDir
    fi
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
done

echo "=> Started $NODES nodes."