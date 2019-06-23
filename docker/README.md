# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

# Run local private testnet
- See [local](https://github.com/kardiachain/go-kardia/tree/master/docker/local) for more details.

# Run private testnet environment 
 ## Prerequisites
 * Setup [NEO network](https://github.com/CityOfZion/neo-local)
 * Setup [TRON network](https://github.com/tronprotocol/tron-deployment)
 ## Kardia network include:
 * 9 Nodes (standalone virtual machines) include:
    * 3 ETH Dual nodes
        * Rinkeby network 
    * 3 NEO Dual nodes 
        * [NEO API server](https://cloud.docker.com/repository/docker/kardiachain/neo-api-server-privnet)
    * 3 TRX Dual nodes
        * [TRON Solidity Node](https://cloud.docker.com/repository/docker/kardiachain/dual-tron)

 * Get IP address of 9 Nodes, for example: 10.0.0.2 -> 10.0.0.10

#### ETH nodes
* Run 3 ETH nodes, replace <ip-node1 -> ip-node9> from IP address of 9 nodes
```
./start_kardia_network.sh -n 1 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9
./start_kardia_network.sh -n 2 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9
./start_kardia_network.sh -n 3 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9
```

#### NEO nodes
* Run 3 NEO nodes, replace <ip-node1 -> ip-node9> from IP address of 9 nodes, <ip-neo-network> from IP address of NEO network 
```
./start_kardia_network.sh -n 4 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9 -neoip <ip-neo-network>
./start_kardia_network.sh -n 5 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9 -neoip <ip-neo-network>
./start_kardia_network.sh -n 6 -ip ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9 -neoip <ip-neo-network>
```

#### TRX nodes
* Run 3 TRX nodes, replace <ip-node1 -> ip-node9> from IP address of 9 nodes, <ip-trx-network> from IP address of TRX network
```
./start_kardia_network.sh -n 7 -ip <ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9> -trxip <ip-tron-network>
./start_kardia_network.sh -n 8 -ip <ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9> -trxip <ip-tron-network>
./start_kardia_network.sh -n 9 -ip <ip-node1,ip-node2,ip-node3,ip-node4,ip-node5,ip-node6,ip-node7,ip-node8,ip-node9> -trxip <ip-tron-network>
```