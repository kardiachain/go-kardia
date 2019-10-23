pragma solidity ^0.4.24;

contract Meta {
    function getCurrentValidatorsInfo() public view returns (address,uint64,uint64) {}
}

contract ValidatorsContract {
    function isValidator(address sender) public view returns (bool) {}
}

contract Staking {

    struct NodeInfo {
        string nodeName;
        string nodeId;
        string ipAddress;
        uint64 port;
        uint256 stakes; // number of stakes a node contributed
        address baseAccount;
    }

    address metaAddress;
    NodeInfo[] candidates;
    mapping(address => uint) nodeAddresses;
    uint64 fromTime;
    uint64 toTime;

    constructor(address meta, uint64 _fromTime, uint64 _toTime) public {
        metaAddress = meta;
        fromTime = _fromTime;
        toTime = _toTime;
    }

    // get current validators contract and check if sender is validator or not in `isValidator` function.
    modifier hasPermission {
        Meta meta = Meta(metaAddress);
        (address contractAddress, , ) = meta.getCurrentValidatorsInfo();
        ValidatorsContract validatorsContract = ValidatorsContract(contractAddress);
        require(
            validatorsContract.isValidator(msg.sender) == true,
            "Only validators can access this function"
        );
        _;
    }

    function getNumberOfCandidates() public view returns (uint) {
        return candidates.length;
    }

    function getCandidate(uint index) public view returns (string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address baseAccount) {
        NodeInfo storage node = candidates[index];
        return (node.nodeName, node.nodeId, node.ipAddress, node.port, node.stakes, node.baseAccount);
    }

    function stake(string nodeName, string nodeId, string ipAddress, uint64 port, address baseAccount) public payable {
        NodeInfo memory node;
        uint index = nodeAddresses[baseAccount];
        node = candidates[index];
        if (node.baseAccount != baseAccount) { // node does not exist
            node = NodeInfo(nodeName, nodeId, ipAddress, port, msg.value, baseAccount);
            candidates.push(node);
            nodeAddresses[baseAccount] = candidates.length - 1;
        } else { // node exists
            node.stakes = node.stakes + msg.value;
            candidates[index] = node;
        }
    }

    // getBalance returns current balance of smart contract
    function getBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // refund is used when a validators period end.
    function refund() public hasPermission {
        for (uint i=0; i < candidates.length; i++) {
            NodeInfo storage node = candidates[i];
            node.baseAccount.transfer(node.stakes);
        }
    }
}
