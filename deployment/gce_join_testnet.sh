#!/bin/bash

# Number of nodes to join testnet
NODES=1
# Network settings
PORT=3000
RPC_PORT=8545
# Prefix name node
PREFIX_NAME="go-kardia-"
# Validator list
MAIN_CHAIN_VAL_INDEX="1,2,3,4,5,6,7,8,9,10,11,12"
# Change the bootnodes list to join testnet
ENODES=(
        "enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@35.240.162.163:3000"
        "enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@35.198.193.180:3000"
        "enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@35.247.151.179:3000"
        "enode://fc41a71d7a74d8665dbcc0f48c9a601e30b714ed50647669ef52c03f7123f2ae078dcaa36389e2636e1055f5f60fdf38d89a226ee84234f006b333cad2d2bcee@35.198.254.247:3000"
        "enode://ebf46faca754fc90716d665e6c6feb206ca437c9e5f16690e690513b302935053a9d722b88d2ab0b972f46448f3a53378bf5cfe01b8373af2e54197b17617e1c@35.247.186.157:3000"
        "enode://80c4fbf65122d817d3808afcb683fc66d9f9e19b476ea0ee3f757dca5cd18316ecb8999bfea4e9a5acc9968504cb919997a5c1ab623c5c533cb662291149b0a3@35.247.187.113:3000"
        "enode://5d7ed8131916b10ea545a559abe464307109a3d62ddbe19c368988bbdb1dd2330b6f3bbb479d0bdd79ef360d7d9175008d90f7d51122969210793e8a752cecd6@35.198.239.141:3000"
        "enode://7ecd4ea1bf4efa34dac41a16d7ccd14e23d3993dd3f0a54d722ee76d170718adba7f246c082bada922c875ffaaa4618e307b68e44c2847d4f4e3b767884c02b7@35.197.131.201:3000"
        "enode://4857f792ef779c511f6d7643f0991409f77e41124ced14385217535641315f5dc9927e7301ffd7afc7ae8025663e17f593306adf7d3ffac7c6aa625c250de0d5@35.186.147.190:3000"
        "enode://ad67c2502fc2723f2dcf25a140744382eb3e4e50d7e4dd910c423f7aa4fe0fbbcc2207d22ef6edf469dd6fbea73efa8d87b4b876a0d6e386c4e00b6a51c2a3f8@35.240.141.247:3000"
        )
PEER=$(printf ",%s" "${ENODES[@]}")
PEER=${PEER:1}

IMAGE_NAME=gcr.io/indigo-history-235904/go-kardia:v0.7.2

# (1 vCPU, 3.75 GB memory)
ZONE="asia-southeast1-a"
MACHINE_TYPE="n1-standard-1"

while test $# -gt 0; do
        case "$1" in
                -h|--help)
                        echo "Join Kardia testnet"
                        echo " "
                        echo "options:"
                        echo "-h, --help                      Show brief help"
                        echo "-n, --num_nodes=NUMBER          Specify how many nodes to join testnet"
                        exit 0
                        ;;
                -n|--num_nodes)
                        shift
                        if test $# -gt 0; then
                                NODES=$1
                        else
                                echo "Number of nodes not specified. Default number of node = 1"
                                exit 1
                        fi
                        shift
                        ;;
                *)
                        break
                        ;;
        esac
done

random_name() {
    random_name=$(base64 /dev/urandom | tr -dc 'a-z' | fold -w 6 | head -n 1)
    echo $random_name
}

cloud_create_instances() {
  prefix_name=$1
  num_nodes=$2

  for ((i=14;i<$num_nodes+14;i+=1));
    do
        # Create a sequence of node names, used to create instances
        name=$prefix_name$(random_name)"00${i}"
        args=(
        --machine-type=${MACHINE_TYPE} \
        --zone=${ZONE} \
        --subnet=default \
        --network-tier=PREMIUM \
        --maintenance-policy=MIGRATE \
        --container-image=${IMAGE_NAME} \
        --container-arg="--dev" \
        --container-arg="--mainChainValIndexes" \
        --container-arg="${MAIN_CHAIN_VAL_INDEX}" \
        --container-arg="--addr" \
        --container-arg=":${PORT}" \
        --container-arg="--name"
        --container-arg="${name}" \
        --container-arg="--rpc" \
        --container-arg="--rpcport"
        --container-arg="${RPC_PORT}" \
        --container-arg="--peer"
        --container-arg="${PEER}" \
        --boot-disk-size=40GB \
        --boot-disk-type=pd-standard \
       )
       echo "Creating instance:" "${name}"
       gcloud compute instances create-with-container "${name}" "${args[@]}"
    done
}

# Create instances
echo "Creating instances..."
cloud_create_instances "$PREFIX_NAME" "$NODES"