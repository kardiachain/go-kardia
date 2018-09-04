# Test JSON-RPC API request
The default address of the rpc server is http://0.0.0.0:0000

Use the following `Postman collection` to test
https://www.getpostman.com/collections/24f2f4a58ad6b0c7a958

Runs the node with `--rpc` flag:
```
./go-kardia --dev --numValid 3 --addr :3000 --name node1 --txn --clearDataDir --rpc
```
Send a json-rpc request to the node using curl:
```
curl -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"kai_blockNumber","params":[],"id":1}' localhost:8545
```
The response will be in the following form:
```
{"jsonrpc":"2.0","id":1,"result":100}
```
