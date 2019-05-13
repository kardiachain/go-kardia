#!/bin/bash

function implode {
    local IFS="$1"; shift; echo "$*";
}

# Number of nodes and prefix name
NODES=3
PORT=3000
RPC_PORT=8545
###
# Indexes of Dual chain and main chain validator nodes
MAIN_VAL_INDEXES=(1,2,3)
MAIN_CHAIN_VAL_INDEXES=$(implode , ${MAIN_VAL_INDEXES[@]})

# Image
IMAGE_NAME=gcr.io/strategic-ivy-130823/go-kardia

PACKAGE=start_local_network.sh
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
                        echo "-n, --num_nodes=NUMBER          Specify how many nodes to bring up. At least 6 nodes."
                        exit 0
                        ;;
                -n|--num_nodes)
                        shift
                        if test $# -gt 0; then
                                NODES=$1
                                if [[ ${NODES} -lt 3 ]]; then
                                    echo "At least 3 nodes for running."
                                    exit 1
                                fi
                        else
                                echo "Number of nodes not specified"
                                exit 1
                        fi
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
while [[ $NODE_INDEX -le $NODES ]]
do
    docker run -d --name node${NODE_INDEX} --log-opt max-size=300m --net=host $IMAGE_NAME \
        --dev --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} \
        --name node${NODE_INDEX} \
        --addr :${PORT} --rpc --rpcport ${RPC_PORT} --clearDataDir
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
done

echo "=> Started $NODES nodes."