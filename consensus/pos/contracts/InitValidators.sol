pragma solidity ^0.4.24;

contract InitValidators {

    struct NodeInfo {
        string nodeName;
        string nodeId;
        string ipAddress;
        uint64 port;
        uint256 stakes; // number of stakes a node contributed
        address baseAccount;
    }

    NodeInfo[] validators;

    constructor() public {
        validators.push(NodeInfo("node1", "enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860", "127.0.0.1", 3000, 1000000000000000000000000000, 0xc1fe56E3F58D3244F606306611a5d10c8333f1f6));
        validators.push(NodeInfo("node1", "enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0", "127.0.0.1", 3001, 1000000000000000000000000000, 0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5));
        validators.push(NodeInfo("node1", "enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da", "127.0.0.1", 3002, 1000000000000000000000000000, 0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd));
    }

    function isValidator(address sender) public view returns (bool) {
        for (uint i=0; i < validators.length; i++) {
            if (validators[i].baseAccount == sender) {
                return true;
            }
        }
        return false;
    }

    function getNumberOfValidators() public view returns (uint) {
        return validators.length;
    }

    function getValidator(uint index) public view returns (string, string, string, uint64, uint256, address, uint) {
        NodeInfo storage node = validators[index];
        // last returned value is voted count which is not used in InitValidators, but keep it here to make it consistent with validators smc for future used.
        return (node.nodeName, node.nodeId, node.ipAddress, node.port, node.stakes, node.baseAccount, 0);
    }
}
