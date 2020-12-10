# Test JSON-RPC API request

The default address of the RPC server is http://0.0.0.0:8545. This means you can access RPC from other
containers/hosts.  
The JSON RPC endpoints are exposed on top of HTTP. WebSocket and/or IPC transports will be supported in the future

Use the following `Postman collection` to test
https://documenter.getpostman.com/view/8826990/TVRhcpMT

Start Kardia network [README](https://github.com/kardiachain/go-kardia/tree/master/README.md)

Send a json-rpc request to the node using curl:

```
curl -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"kai_blockNumber","params":[],"id":1}' localhost:8545
```

The response will be in the following form:

```
{"jsonrpc":"2.0","id":1,"result":100}
```

List of all supported APIs can be found here: https://github.com/kardiachain/go-kardia/wiki/Kardia-JSON-RPC-API

### License
Kardia JSON-RPC is based on the work of HTTP endpoint server in go-ethereum RPC libary.
The go-ethereum RPC library is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in this repository in the `LICENSE-3RD-PARTY.txt` file.
