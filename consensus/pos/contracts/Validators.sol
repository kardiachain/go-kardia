pragma solidity ^0.4.24;

contract Meta {
    function getCurrentValidatorsInfo() public view returns (address,uint64,uint64) {}
    function getCurrentVotingInfo() public view returns (address,uint64,uint64) {}
}

contract VotingContract {
    function getNumberOfValidators() public view returns (uint) {}
    function getValidator(uint index) public view returns (string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address nodeAddress) {}
}

contract Validators {

    struct NodeInfo {
        string nodeName;
        string nodeId;
        string ipAddress;
        uint64 port;
        uint256 stakes; // number of stakes a node contributed
        address nodeAddress;
    }

    address metaAddress;
    NodeInfo[] validators;
    mapping(address => bool) nodeAddresses;
    uint64 fromTime;
    uint64 toTime;

    constructor(address meta, uint64 _fromTime, uint64 _toTime) public {
        metaAddress = meta;
        fromTime = _fromTime;
        toTime = _toTime;

        // get current voting from meta
        Meta metaContract = Meta(metaAddress);
        (address contractAddress, , ) = metaContract.getCurrentVotingInfo();

        // get length of validators from voting
        VotingContract votingContract = VotingContract(contractAddress);
        uint numberOfValidators = votingContract.getNumberOfValidators();

        for (uint i = 0; i < numberOfValidators; i++) {
            // get validator info by index i
            (string memory nodeName, string memory nodeId, string memory ipAddress, uint64 port, uint256 stakes, address nodeAddress) = votingContract.getValidator(i);
            addValidator(nodeName, nodeId, ipAddress, port, stakes, nodeAddress);
        }
    }

    // addValidator adds a node into validators list. Sender must be previous validator.
    function addValidator(string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address nodeAddress) internal {
        NodeInfo memory node = NodeInfo(nodeName, nodeId, ipAddress, port, stakes, nodeAddress);
        validators.push(node);
        nodeAddresses[nodeAddress] = true;
    }

    function getNumberOfValidators() public view returns (uint) {
        return validators.length;
    }

    function getValidator(uint index) public view returns (string, string, string, uint64, uint256, address) {
        NodeInfo storage node = validators[index];
        return (node.nodeName, node.nodeId, node.ipAddress, node.port, node.stakes, node.nodeAddress);
    }
}
