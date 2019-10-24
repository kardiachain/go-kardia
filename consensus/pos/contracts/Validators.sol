pragma solidity ^0.4.24;

contract Meta {
    function getCurrentVotingInfo() public view returns (address,uint64,uint64) {}
    function maxValidators() public view returns (uint) {}
}

contract VotingContract {
    function getVoteCount(uint index) public view returns (uint) {}
    function numberOfCandidates() public view returns (uint) {}
    function getCandidate(uint index) public view returns (uint idx, string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address baseAccount) {}
}

contract Validators {

    struct NodeInfo {
        string nodeName;
        string nodeId;
        string ipAddress;
        uint64 port;
        uint256 stakes; // number of stakes a node contributed
        address nodeAddress;
        uint voteCount;
    }

    uint lengthOfValidators;
    mapping(uint=>NodeInfo) validators;
    uint64 fromTime;
    uint64 toTime;

    constructor(address meta, uint64 _fromTime, uint64 _toTime) public {
        fromTime = _fromTime;
        toTime = _toTime;
        lengthOfValidators = 0;

        // get current voting from meta
        (address contractAddress, , ) = Meta(meta).getCurrentVotingInfo();
        uint maxValidators = Meta(meta).maxValidators();

        // get length of validators from voting
        VotingContract votingContract = VotingContract(contractAddress);
        uint numberOfValidators = votingContract.numberOfCandidates();

        for (uint i = 0; i < numberOfValidators; i++) {
            // get validator info by index i
            (, string memory nodeName, string memory nodeId, string memory ipAddress, uint64 port, uint256 stakes, address nodeAddress) = votingContract.getCandidate(i);

            // get vote count. if vote count == 0 continue. otherwise sort through current list if the list is less than maxValidators
            uint voteCount = votingContract.getVoteCount(i);

            if (voteCount > 0) {
                NodeInfo memory node = NodeInfo(nodeName, nodeId, ipAddress, port, stakes, nodeAddress, voteCount);
                if (lengthOfValidators == 0) {
                    validators[lengthOfValidators] = node;
                    lengthOfValidators = lengthOfValidators + 1;
                } else {
                    // sort current node with lengthOfValidators and maxValidators
                    if (sort(node, 0, lengthOfValidators, maxValidators)) {
                        lengthOfValidators = lengthOfValidators + 1;
                    }
                }
            }
        }
    }

    function getNumberOfValidators() public view returns (uint) {
        return lengthOfValidators;
    }

    function getValidator(uint index) public view returns (string, string, string, uint64, uint256, address, uint) {
        NodeInfo storage node = validators[index];
        return (node.nodeName, node.nodeId, node.ipAddress, node.port, node.stakes, node.nodeAddress, node.voteCount);
    }

    function sort(NodeInfo node, uint index, uint currentLength, uint maxLength) internal returns (bool) {
        if (index >= maxLength) return false;
        if (index >= currentLength) {
            validators[index] = node;
            return true;
        }
        if (node.voteCount > validators[index].voteCount) {
            return switchIndex(node, index, currentLength, maxLength);
        }
        return sort(node, index+1, currentLength, maxLength);
    }

    function switchIndex(NodeInfo node, uint index, uint currentLength, uint maxLength) internal returns (bool) {
        if (index >= maxLength) return false;
        if (index < currentLength) {
            NodeInfo storage currentNode = validators[index];
            validators[index] = node;
            return switchIndex(currentNode, index+1, currentLength, maxLength);
        }
        validators[index] = node;
        return true;
    }
}
