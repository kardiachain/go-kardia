#!/bin/bash
#
# Create instances on GCE to host Kardia testnet nodes
# 15 Kardia nodes with: mainChainValIndexes 2,3,4,5,6,7,8
# +  3 ETH dual nodes: dualChainValIndexes 1,2,3
# +  3 NEO dual nodes: dualChainValIndexes 4,5,6

# Docker images
KARDIA_GO_IMAGE=gcr.io/strategic-ivy-130823/go-kardia:latest
KARDIA_SCAN_IMAGE=gcr.io/strategic-ivy-130823/kardia-scan:latest
NEO_API_SERVER_IMAGE=gcr.io/strategic-ivy-130823/neo_api_server_testnet:latest

# Number of nodes and prefix name
NODES=15
ETH_NODES=3
NEO_NODES=3
NAME_PREFIX="kardia-testnet-"

# Indexes of Dual chain and main chain validator nodes
DUAL_ETH_CHAIN_VAL_INDEXES="1,2,3"
DUAL_NEO_CHAIN_VAL_INDEXES="4,5,6"
MAIN_CHAIN_VAL_INDEXES="2,3,4,5,6,7,8"

# ports
PORT=3000
RPC_PORT=8545

# Instance specs
ZONE="us-west1-b"
MACHINE_TYPE="n1-standard-1"
IMAGE="cos-stable-70-11021-67-0"

# Eth Instance specs
ETH_CUSTOM_CPU=2
ETH_CUSTOM_MEMORY="10GB"
ETH_BOOT_DISK_SIZE="50GB"

# Index of instance hosting Kardia-scan
KARDIA_SCAN_NODE=3

# URL config for NEO dual node
NEO_API_URL="http://127.0.0.1:5000"
NEO_CHECK_TX_URL="https://neoscan-testnet.io/api/test_net/v1/get_transaction/"
# FIXME receiver address should be created dynamically
NEO_RECEIVER_ADDRESS="AaXPGsJhyRb55r8tREPWWNcaTHq4iiTFAH"
KARDIA_URL="127.0.0.1:8545"

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



#############################################################################
# cloud_create_instances creates instances with specific specs
#   cloud_create_instances [name_prefix] [num_nodes] [eth_num_nodes]
# Globals:
#   ZONE                  - Zone of the instances to create
#   MACHINE_TYPE          - Specifies the machine type used for the instances
#   IMAGE                 - Boot image for instance
#   ETH_CUSTOM_CPU        - Number of CPUs used on instance hosting Ethereum node
#   ETH_CUSTOM_MEMORY     - Memory size of instance hosting Ethereum node
#   ETH_BOOT_DISK_SIZE    - Disk size of instance hosting Ethereum node
# Arguments:
#   name_prefix           - name prefix of instances
#   num_nodes             - number of instances
#   eth_num_nodes         - number of instances hosting Ethereum node
#   neo_num_nodes         - number of instances hosting Neo node
# Returns:
#   None
#############################################################################
cloud_create_instances() {
  name_prefix=$1
  num_nodes=$2
  eth_num_nodes=$3
  neo_num_nodes=$4
  nodes=()
  ether_dual_nodes=()

  # Create a sequence of node names, used to create instances
  if [ ${num_nodes} -eq 1 ]; then
    nodes=("$name_prefix")
  fi

  if [ ${eth_num_nodes} -gt 0 ]; then
    # Trailing zeros are added to make GCE sort instances by name correctly
    # Sequence of Ethereum node names, e.g. 3 Ethereum nodes: ([prefix]001 [prefix]002 [prefix]003)
    ether_dual_nodes=($(seq -f ${name_prefix}%03g 1 ${eth_num_nodes}))
    echo "=======> ETH dual node:" ${ether_dual_nodes[@]}
  fi

  next_node=$((eth_num_nodes+1))

  if [ ${neo_num_nodes} -gt 0 ]; then
      # Sequence of Ethereum node names, e.g. 3 Neo nodes: ([prefix]004 [prefix]005 [prefix]006)
      neo_dual_nodes=($(seq -f ${name_prefix}%03g ${next_node} $((next_node+neo_num_nodes-1))))
      echo "=======> NEO dual node:" ${neo_dual_nodes[@]}
  fi

  next_node=$((next_node+neo_num_nodes))

  if [ ${num_nodes} -gt 1 ]; then
      # Sequence of Kardia node names, e.g. 9 nodes: ([prefix]007 [prefix]008 [prefix]009 [prefix]010 [prefix]011 [prefix]012 [prefix]013 [prefix]014 [prefix]015 )
      nodes=($(seq -f ${name_prefix}%03g ${next_node} ${num_nodes}))
      echo "=======> Kardia node:" ${nodes[@]}
  fi

  # Specs of instance hosting Ethereum node
  eth_args=(
    --zone ${ZONE} \
    --subnet=default \
    --network-tier=PREMIUM \
    --maintenance-policy=MIGRATE \
    --tags=http-server,https-server \
    --image=${IMAGE} \
    --image-project=cos-cloud \
    --custom-cpu=${ETH_CUSTOM_CPU} \
    --custom-memory=${ETH_CUSTOM_MEMORY} \
    --boot-disk-size=${ETH_BOOT_DISK_SIZE} \
    --boot-disk-type=pd-standard \
    )

  # Specs of instance hosting Kardia node
  args=(
    --machine-type=${MACHINE_TYPE} \
    --zone ${ZONE} \
    --subnet=default \
    --network-tier=PREMIUM \
    --maintenance-policy=MIGRATE \
    --tags=http-server,https-server \
    --image=${IMAGE} \
    --image-project=cos-cloud \
    --boot-disk-size=30GB \
    --boot-disk-type=pd-standard \
    )

  # Create new instances on GCE
  if [ $eth_num_nodes -gt 0 ]; then
  (
    # DEBUG: Print all commands
    set -x
    gcloud compute instances create "${ether_dual_nodes[@]}" "${eth_args[@]}"
  )
  fi

  if [ $neo_num_nodes -gt 0 ]; then
  (
    # DEBUG: Print all commands
    set -x
    gcloud compute instances create "${neo_dual_nodes[@]}" "${args[@]}"
  )
  fi

  (
    # DEBUG: Print all commands
    set -x
    gcloud compute instances create "${nodes[@]}" "${args[@]}"
  )

}


#############################################################################
# cloud_find_instances finds instances matching the specified pattern
#   cloud_find_instances [filter]
# Globals:
#   none
# Arguments:
#   filter - instance name or prefix
# Returns:
#   string list - list info of instances name:public_ip:private_ip:status
#############################################################################
cloud_find_instances() {
  filter="$1"
  instances=()

  while read -r name public_ip private_ip status; do
    printf "%-30s | public_ip=%-16s private_ip=%s staus=%s\n" "$name" "$public_ip" "$private_ip" "$status"

    instances+=("$name:$public_ip:$private_ip")
  done < <(gcloud compute instances list \
             --filter "$filter" \
             --format 'value(name,networkInterfaces[0].accessConfigs[0].natIP,networkInterfaces[0].networkIP,status)')
}

#############################################################################
# cloud_find_instances finds instances by names matching the specified prefix
#   cloud_find_instances [name_prefix]
# Globals:
#   NAME_PREFIX
# Arguments:
#   name_prefix - instance name prefix
# Returns:
#   string list - list info of instances name:public_ip:private_ip:status
#############################################################################
cloud_find_instances_by_prefix() {
  name_prefix="$1"
  cloud_find_instances "name~^$name_prefix"
}


#############################################################################
# cloud_find_instances finds instances by name
#   cloud_find_instances [name]
# Globals:
#   none
# Arguments:
#   name - instance name
# Returns:
#   string list - list info of instances name:public_ip:private_ip:status
#############################################################################
cloud_find_instance_by_name() {
  name="$1"
  cloud_find_instances "name=$name"
}


#############################################################################
# Implementation flow
#
# 1. Create instances
# 2. Get name and private IP of instances
# 3. Use name and private IP to SSH instances
# 4. Docker pull and run image to start a new node of testnet
#
#############################################################################
# Create instances
echo "Creating instances..."
cloud_create_instances "$NAME_PREFIX" "$NODES" "$ETH_NODES" "$NEO_NODES"

# Get instance info
cloud_find_instances_by_prefix "$NAME_PREFIX"
echo "instances: ${#instances[@]}"

# Store address of instance in format: [enode][private_ip]:[PORT]
addresses=()
public_ips=()
for (( i=0; i<$NODES; i++ ));
do
    # instance[i] is a string "$name:$public_ip:$private_ip"
    # Split instance[i] by ":" to acquire name, public_ip, and private_ip
    # name and private_ip used to SSH to instance
    IFS=: read -r name public_ip private_ip <<< "${instances[$i]}"
    # 10 seed nodes
    if [ $i -lt 10 ] ; then
        addresses+=("${ENODES[$i]}${private_ip}:${PORT}")
    fi
    public_ips+=("http://${public_ip}:${RPC_PORT}")
done

# Concatenate addresses
peers=$(IFS=, ; echo "${addresses[*]}")
rpc_hosts=$(IFS=, ; echo "${public_ips[*]}")

node_index=1
docker_cmd="docker pull ${KARDIA_GO_IMAGE}"
for info in "${instances[@]}";
do
  IFS=: read -r name public_ip private_ip <<< "$info"

  run_cmd=
  if [ "$node_index" -le "$ETH_NODES" ]; then
      # cmd to run instance hosting Ethereum node
      run_cmd="mkdir -p /var/kardiachain/node${node_index}/ethereum; docker run -d -p ${PORT}:${PORT}/udp -p ${PORT}:${PORT} -p ${RPC_PORT}:${RPC_PORT} --name node${node_index} -v /var/kardiachain/node${node_index}/ethereum:/root/.ethereum ${KARDIA_GO_IMAGE} --dev --dual --dualchain --dualChainValIndexes ${DUAL_ETH_CHAIN_VAL_INDEXES} --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} --ethstat --ethstatname eth-dual-test-${node_index} --addr :${PORT} --name node${node_index} --rpc --rpcport ${RPC_PORT} --clearDataDir --peer ${peers}"
      # instance 3 hosts kardia-scan frontend
      # use http://${public_ip}:8080 to see the kardia-scan frontend
      if [ "$node_index" -eq "$KARDIA_SCAN_NODE" ]; then
        run_cmd="$run_cmd;docker pull ${KARDIA_SCAN_IMAGE}; docker run -e "RPC_HOSTS=${rpc_hosts}" -e "publicIP=http://${public_ip}:8080" -p 8080:80 ${KARDIA_SCAN_IMAGE}"
      fi
  elif [ "$node_index" -le $((ETH_NODES + NEO_NODES)) ]; then
      # cmd to run neo api server
      # cmd to run instance hosting Neo node
      run_cmd="docker run -d -p ${PORT}:${PORT}/udp -p ${PORT}:${PORT} -p ${RPC_PORT}:${RPC_PORT} --name node${node_index} ${KARDIA_GO_IMAGE} --dev --dualchain --neodual --dualChainValIndexes ${DUAL_NEO_CHAIN_VAL_INDEXES} --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} --addr :${PORT} --name node${node_index} --rpc --rpcport ${RPC_PORT} --clearDataDir --peer ${peers} --neoSubmitTxUrl=${NEO_API_URL} --neoCheckTxUrl=${NEO_CHECK_TX_URL} --neoReceiverAddress=${NEO_RECEIVER_ADDRESS}"
      if [ "$node_index" -eq $((ETH_NODES + 1)) ]; then
        NEO_API_URL=http://${public_ip}:5000
        run_cmd="$run_cmd;docker pull ${NEO_API_SERVER_IMAGE}"
        run_cmd="$run_cmd;docker run -d --name neo-api-server --env kardia=${KARDIA_URL} -p 5000:5000 -p 8080:8080 ${NEO_API_SERVER_IMAGE}"
      fi

  else
      # cmd to run instance hosting Kardia node
      run_cmd="docker run -d -p ${PORT}:${PORT}/udp -p ${PORT}:${PORT} -p ${RPC_PORT}:${RPC_PORT} --name node${node_index} ${KARDIA_GO_IMAGE} --dev --mainChainValIndexes ${MAIN_CHAIN_VAL_INDEXES} --addr :${PORT} --name node${node_index} --rpc --rpcport ${RPC_PORT} --clearDataDir --peer ${peers}"
  fi

  # SSH to instance
  (
    echo "Instance $node_index cmds: "

    # DEBUG: Print all commands
    set -x
    gcloud compute ssh "${name}" --zone="${ZONE}" --command="${docker_cmd};${run_cmd}"
  ) &
  ((node_index++))
done
