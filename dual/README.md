# Dual node
Currently support Ether-Kardia dual node.  

### Connects to Ether Rinkeby testnet
Runs node with `--dual` flag to start dual mode, acting as a full node syncing on both Kardia network and Ethereum Rinkeby testnet.  
As a Rinkeby full node, dual node needs 4GB+ RAM, 10GB+ local storage in SSD, and open ports 30300-30399 to connect to other public Eth nodes. 
Node can use `--ethstat` flag to report stats to official Rinkeby website.
```
./go-kardia --dual --name node1 --ethstat --ethstatname eth-kardia-test
```

### Monitor node in Ether and Kardia network
Node will show up normally in both Ether and Kardia network monitoring.  
Check [Rinkeby stat website](https://www.rinkeby.io/#stats) to see node with special type `GethKarida` and chosen name above.
Node should be connected to other Eth peers and start syncing blocks.

Use [Kardiascan](https://github.com/kardiachain/KardiaScan#run-development-mode) to monitor Kardia stats. 
