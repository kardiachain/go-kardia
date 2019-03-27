#!/bin/bash

function implode {
    local IFS="$1"; shift; echo "$*";
}

# Number of nodes and prefix name
NODES=6
PORT=3000
RPC_PORT=8545
ETH_PORT=8645
ETH_ADDR=30302
###
ETH_NODES=3
NEO_NODES=3

# Indexes of Dual chain and main chain validator nodes
ETH_VAL_INDEXES=(1 2 3)
NEO_VAL_INDEXES=(4 5 6)
MAIN_VAL_INDEXES=(2,3,4)
DUAL_ETH_CHAIN_VAL_INDEXES=$(implode , ${ETH_VAL_INDEXES[@]})
DUAL_NEO_CHAIN_VAL_INDEXES=$(implode , ${NEO_VAL_INDEXES[@]})
MAIN_CHAIN_VAL_INDEXES=$(implode , ${MAIN_VAL_INDEXES[@]})

# Image
IMAGE_NAME=gcr.io/strategic-ivy-130823/go-kardia

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
                        echo "-n, --num_nodes=NUMBER          Specify how many nodes to bring up. At least 6 nodes."
                        exit 0
                        ;;
                -n|--num_nodes)
                        shift
                        if test $# -gt 0; then
                                NODES=$1
                                if [[ ${NODES} -lt 6 ]]; then
                                    echo "At least 6 nodes for running."
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
while [ $NODE_INDEX -le $NODES ]
do
    if [[ " ${ETH_VAL_INDEXES[*]} " =~ " ${NODE_INDEX} " ]]; then
         mkdir -p ~/.kardia/node${NODE_INDEX}/data/ethereum
         docker run -d --name node${NODE_INDEX} -v ~/.kardia/node${NODE_INDEX}/data/ethereum:/root/.ethereum --net=host $IMAGE_NAME \
         --dev --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} \
         --dual --dualchain --dualChainValIndexes ${DUAL_ETH_CHAIN_VAL_INDEXES} \
         --ethstat --ethstatname eth-dual-privnet-${NODE_INDEX} \
         --name node${NODE_INDEX} \
         --addr :${PORT} --rpc --rpcport ${RPC_PORT} --ethAddr :${ETH_ADDR} --ethRPCPort ${ETH_PORT} --clearDataDir --noProxy
    elif [[ " ${NEO_VAL_INDEXES[*]} " =~ " ${NODE_INDEX} " ]]; then
        docker run -d --name node${NODE_INDEX} --net=host $IMAGE_NAME \
        --dev --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} \
        --neodual --dualchain --dualChainValIndexes ${DUAL_NEO_CHAIN_VAL_INDEXES} \
        --name node${NODE_INDEX} \
        --addr :${PORT} --rpc --rpcport ${RPC_PORT} --clearDataDir --noProxy
    else
        docker run -d --name node${NODE_INDEX} --net=host $IMAGE_NAME \
        --dev --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} \
        --name node${NODE_INDEX} \
        --addr :${PORT} --rpc --rpcport ${RPC_PORT} --clearDataDir --noProxy
    fi
    ((NODE_INDEX++))
    ((PORT++))
    ((RPC_PORT++))
    ((ETH_PORT++))
    ((ETH_ADDR++))
done

echo "=> Started $NODES nodes."