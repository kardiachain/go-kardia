#!/bin/bash

IFS=','
function implode {
    local IFS="$1"; shift; echo "$*";
}

replacePeer () {
    sed -i.bak "s/{peer}/${2}/g" "$1"
}

replaceNode () {
    sed -i.bak "s/{node_name}/${2}/g" "$1"
}

replaceNeoIp () {
    sed -i.bak "s/{neoip}/${2}/g" "$1"
}

replaceTrxIp () {
    sed -i.bak "s/{trxip}/${2}/g" "$1"
}


quoteString () {
    echo "$1" | sed -e 's/\\/\\\\/g;s/\//\\\//g;s/&/\\\&/g'
}

# Number of nodes and prefix name
NODES=9
PORT=3000
RPC_PORT=8545
ETH_PORT=8645
ETH_ADDR=30302
###
ETH_NODES=3
NEO_NODES=3
TRX_NODES=3

# Indexes of Dual chain and main chain validator nodes
ETH_VAL_INDEXES=(1 2 3)
NEO_VAL_INDEXES=(4 5 6)
TRX_VAL_INDEXES=(7 8 9)
MAIN_VAL_INDEXES=(2,3,4)
DUAL_ETH_CHAIN_VAL_INDEXES=$(implode , ${ETH_VAL_INDEXES[@]})
DUAL_NEO_CHAIN_VAL_INDEXES=$(implode , ${NEO_VAL_INDEXES[@]})
DUAL_TRX_CHAIN_VAL_INDEXES=$(implode , ${TRX_VAL_INDEXES[@]})
MAIN_CHAIN_VAL_INDEXES=$(implode , ${MAIN_VAL_INDEXES[@]})

# Image
IMAGE_NAME=gcr.io/strategic-ivy-130823/go-kardia

ENODES=(
     "enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@"
	 "enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@"
	 "enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@"
	 "enode://fc41a71d7a74d8665dbcc0f48c9a601e30b714ed50647669ef52c03f7123f2ae078dcaa36389e2636e1055f5f60fdf38d89a226ee84234f006b333cad2d2bcee@"
	 "enode://ebf46faca754fc90716d665e6c6feb206ca437c9e5f16690e690513b302935053a9d722b88d2ab0b972f46448f3a53378bf5cfe01b8373af2e54197b17617e1c@"
	 "enode://80c4fbf65122d817d3808afcb683fc66d9f9e19b476ea0ee3f757dca5cd18316ecb8999bfea4e9a5acc9968504cb919997a5c1ab623c5c533cb662291149b0a3@"
	 "enode://5d7ed8131916b10ea545a559abe464307109a3d62ddbe19c368988bbdb1dd2330b6f3bbb479d0bdd79ef360d7d9175008d90f7d51122969210793e8a752cecd6@"
	 "enode://7ecd4ea1bf4efa34dac41a16d7ccd14e23d3993dd3f0a54d722ee76d170718adba7f246c082bada922c875ffaaa4618e307b68e44c2847d4f4e3b767884c02b7@"
	 "enode://4857f792ef779c511f6d7643f0991409f77e41124ced14385217535641315f5dc9927e7301ffd7afc7ae8025663e17f593306adf7d3ffac7c6aa625c250de0d5@"
	 "enode://ad67c2502fc2723f2dcf25a140744382eb3e4e50d7e4dd910c423f7aa4fe0fbbcc2207d22ef6edf469dd6fbea73efa8d87b4b876a0d6e386c4e00b6a51c2a3f8@"
)


PACKAGE=start_kardia_network.sh

while test $# -gt 0; do
        case "$1" in
                -h|--help)
                        echo "$PACKAGE - Starts a Kardia network"
                        echo " "
                        echo "$PACKAGE [options]"
                        echo " "
                        echo "options:"
                        echo "-h, --help                    Show brief help"
                        echo "-n, --node                    Specify node <id> to bring up"
                        echo "-ip, --ip                     Specify ip address of nodes to adding peer, delimiter by ',' character. At least 9 ip addresses"
                        exit 0
                        ;;
                -n|--node)
                        shift
                        if test $# -gt 0; then
                            NODE_ID=$1
                            re='^[0-9]+$'
                            if ! [[ ${NODE_ID} =~ $re ]] ; then
                               echo "Node id is not a number"
                               exit 1
                            fi
                            NODE_NAME="node${NODE_ID}"
                        else
                            echo "Node id is not specified"
                            exit 1
                        fi
                        shift
                        ;;
                -ip|--ip)
                        shift
                        if test $# -gt 0; then
                            input=$1
                            read -ra IPs <<< "$input" #
                            if [[ ${#IPs[@]} -lt 9 ]]; then
                                echo "At least 9 nodes for running."
                                exit 1
                            fi
                        else
                            echo "IP of nodes not specified"
                            exit 1
                        fi
                        shift
                        ;;
                -neoip|--neoip)
                        shift
                        if test $# -gt 0; then
                            NEO_IP=$1
                        else
                            echo "IP of NEO network not specified"
                            exit 1
                        fi
                        shift
                        ;;
                -trxip|--trxip)
                        shift
                        if test $# -gt 0; then
                            TRX_IP=$1
                        else
                            echo "IP of TRX network not specified"
                            exit 1
                        fi
                        shift
                        ;;
                *)
                        break
                        ;;
        esac
done

if [[ -z "$NODE_NAME" ]]; then
    echo "Node id is not specified."
    exit 1
fi

if [[ ${#IPs[@]} -lt 9 ]]; then
    echo "IP of nodes not specified."
    exit 1
fi

addresses=()
i=0
for ip in "${IPs[@]}"; do #
    echo "Peer: $ip"
    addresses+=("${ENODES[$i]}${ip}:3000")
    ((i++))
done

# Concatenate addresses
peers=$(quoteString "${addresses[*]}")

echo "======== Starting ${NODE_NAME} ============="

# Start nodes

# Create folder for nodes
mkdir -p /tmp/kardiachain/${NODE_NAME}/data/db

if [[ "${ETH_VAL_INDEXES[*]}" =~ "${NODE_ID}" ]]; then
    cp -r eth/dual_eth.yaml eth/${NODE_NAME}.yaml
    replacePeer "eth/${NODE_NAME}.yaml" "$peers"
    replaceNode "eth/${NODE_NAME}.yaml" "$NODE_NAME"
    # Stop container
    docker-compose -f eth/${NODE_NAME}.yaml down
    # Create folder
    mkdir -p /tmp/kardiachain/${NODE_NAME}/data/ethereum
    # Run
    docker-compose -f eth/${NODE_NAME}.yaml up -d
elif [[ "${NEO_VAL_INDEXES[*]}" =~ "${NODE_ID}" ]]; then
    cp -r neo/dual_neo.yaml neo/${NODE_NAME}.yaml
    replacePeer "neo/${NODE_NAME}.yaml" "$peers"
    replaceNode "neo/${NODE_NAME}.yaml" "$NODE_NAME"
    cp -r neo/privnet_neo.json neo/privnet.json

    if [[ -z "$NEO_IP" ]]; then
        echo "IP of NEO network not specified"
        exit 1
    fi
    replaceNeoIp neo/privnet.json "$NEO_IP"
    # Stop container
    docker-compose -f neo/${NODE_NAME}.yaml down
    # Create folder
    mkdir -p /tmp/kardiachain/${NODE_NAME}/Chains
    # Run
    docker-compose -f neo/${NODE_NAME}.yaml up -d
elif [[ "${TRX_VAL_INDEXES[*]}" =~ "${NODE_ID}" ]]; then
    cp -r trx/dual_trx.yaml trx/${NODE_NAME}.yaml
    replacePeer "trx/${NODE_NAME}.yaml" "$peers"
    replaceNode "trx/${NODE_NAME}.yaml" "$NODE_NAME"
    if [[ -z "$TRX_IP" ]]; then
        echo "IP of TRX network not specified"
        exit 1
    fi
    # Create folder
    mkdir -p /tmp/kardiachain/dual-tron/resources
    mkdir -p /tmp/kardiachain/dual-tron/output-directory
    mkdir -p /tmp/kardiachain/dual-tron/logs
    cp -r trx/resources/fullnode_trx.conf trx/resources/fullnode.conf
    replaceTrxIp trx/resources/fullnode.conf "$TRX_IP"
    cp -r trx/resources/fullnode.conf /tmp/kardiachain/dual-tron/resources
    # Stop container
    docker-compose -f trx/${NODE_NAME}.yaml down
    # Run
    docker-compose -f trx/${NODE_NAME}.yaml up -d
fi

echo "=> Started ${NODE_NAME}"