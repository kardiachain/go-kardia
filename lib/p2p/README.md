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

--entryNode flag has been added that takes the string argument enode. The new node will handshake with given enode and then start the discovery process 

### To test the p2p discovery:

#### Terminal one
```
go install; $GOPATH/bin/go-kardia --addr :3000 --name node1 --clearDataDir 
```
Look in the logs to find the enode address for example: "self=enode://ab1e7f164bd02ab41421f5f1424c5ca8f127c0c0c61c87417abf237abf32ae1ad2f62ab0725335d8144b9d14840dd569d149eb0611f1f2fe239b2fef80fd8aa3@[::]:3000"

Copy from enode until the end.

#### Terminal two
```
go install; $GOPATH/bin/go-kardia --addr :3001 --name node2 --clearDataDir --entryNode "ENODE ADDRESS"
```
You should see the two nodes connecting.

In another terminal, we can connect to the first.
#### Terminal three
```
go install; $GOPATH/bin/go-kardia --addr :3002 --name node3 --clearDataDir --entryNode "ENODE ADDRESS"
```
Initially, you should see node1 and node3 connect to each other and after a few seconds, node2 should connect to node3 automatically.
