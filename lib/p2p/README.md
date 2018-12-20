## P2P Network Protocols.

This package implements the underlying peer-to-peer network communication between Kardia nodes.
Based on go-ethereum P2P library, Kardia p2p library allows rapidly adding & modifying new protocols on the Kardia service top layer.

### License
This is based on the work of go-ethereum P2P library, which is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in this repository in the `LICENSE-3RD-PARTY.txt` file.

### Notes:
The Kardia P2P library is based on the go-ethereum discovery v4.

P2P Network now does not require the --dev flag

Nodes will need to connect to each other initially and will find new peers based on the known peers.

--bootNode flag has been added that takes the string argument enode. The new node will handshake with given enode and then start the discovery process. This only accepts one node as argument. If more than one node is present, or if the given node is not found, the program sends a warning.

### To set up a p2p node using bootNode flag:

1) Ensure your firewall settings allow for TCP and UDP communication.
2) Copy the enode address along with the IP of the remote node that you wish to connect to.
3) Start your node with your required flags as well as the --bootNode flag and the enode address + IP of the remote node.</br>
   a) You may also want to start your node with "--loglevel trace" for verbose logs.
4) Ensure you have connected by searching your logs for "Bonding with peer" and the first 16 characters of the ID.</br>
  a) If you have started your node with --loglevel trace, you will also see ">> PING/v4", "<< PING/v4", ">> PONG/v4", and "<< PONG/v4".
     This ensures your node is communicating and bonding with the bootNode.
  
### To test the p2p discovery:

#### Terminal one
```
go install; $GOPATH/bin/go-kardia --addr :3000 --name node1 --clearDataDir 
```
Look in the logs to find the enode address for example: "self=enode://ab1e7f164bd02ab41421f5f1424c5ca8f127c0c0c61c87417abf237abf32ae1ad2f62ab0725335d8144b9d14840dd569d149eb0611f1f2fe239b2fef80fd8aa3@[::]:3000"

Copy from enode until the end.

#### Terminal two
```
go install; $GOPATH/bin/go-kardia --addr :3001 --name node2 --clearDataDir --bootNode "ENODE ADDRESS"
```
You should see the two nodes connecting.

In another terminal, we can connect to the first.
#### Terminal three
```
go install; $GOPATH/bin/go-kardia --addr :3002 --name node3 --clearDataDir --bootNode "ENODE ADDRESS"
```
Initially, you should see node1 and node3 connect to each other and after a few seconds, node2 should connect to node3 automatically.
