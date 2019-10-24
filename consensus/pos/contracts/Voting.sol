pragma solidity ^0.4.24;

contract Meta {
    function getCurrentStakingInfo() public view returns (address,uint64,uint64) {}
    function maxValidators() public view returns (uint) {}
    function minStakes() public view returns (uint64) {}
}

contract ValidatorsContract {
    function isValidator(address sender) public view returns (bool) {}
}

contract StakingContract {
    function getCandidate(uint index) public view returns (string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address baseAccount) {}
    function getNumberOfCandidates() public view returns (uint) {}
}

contract Voting {

    struct NodeInfo {
        string nodeName;
        string nodeId;
        string ipAddress;
        uint64 port;
        uint256 stakes; // number of stakes a node contributed
        address baseAccount;
    }

    struct VoteInfo {
        uint index;
        uint count;
    }

    address metaAddress;
    address stakingAddress;

    uint64 fromTime;
    uint64 toTime;
    uint numberOfValidCandidates;

    // candidates sorts a list of candidates based on their stakes.
    mapping(uint=>NodeInfo) candidates;

    // voteList is a list voted nodes, key is index and value is number of vote.
    mapping(uint=>uint) voteList;

    uint numberOfVotedNodes;

    // voted stores voted address.
    mapping(address=>mapping(uint=>bool)) voted;

    constructor(address meta, uint64 _fromTime, uint64 _toTime) public {
        fromTime = _fromTime;
        toTime = _toTime;
        metaAddress = meta;
        numberOfValidCandidates = 0;
        numberOfVotedNodes = 0;

        Meta metaContract = Meta(metaAddress);

        (stakingAddress, , ) = metaContract.getCurrentStakingInfo();
        uint256 minStakes = metaContract.minStakes();

        StakingContract stakingContract = StakingContract(stakingAddress);
        uint numberOfCandidates = stakingContract.getNumberOfCandidates();

        // loop through numberOfCandidates to get nodeInfo from staking contract for each index. Each NodeInfo must be greater than minStakes to be valid.
        for (uint i=0; i < numberOfCandidates; i++) {
            (string memory nodeName, string memory nodeId, string memory ipAddress, uint64 port, uint256 stakes, address baseAccount) = stakingContract.getCandidate(i);
            NodeInfo memory node = NodeInfo(nodeName, nodeId, ipAddress, port, stakes, baseAccount);
            if (node.stakes >= minStakes) {
                candidates[numberOfValidCandidates] = node;
                numberOfValidCandidates = numberOfValidCandidates + 1;
            }
        }
    }

    function numberOfCandidates() public view returns (uint) {
        return numberOfValidCandidates;
    }

    function getCandidate(uint index) public view returns (uint idx, string nodeName, string nodeId, string ipAddress, uint64 port, uint256 stakes, address baseAccount) {
        NodeInfo storage node = candidates[index];
        return (index, node.nodeName, node.nodeId, node.ipAddress, node.port, node.stakes, node.baseAccount);
    }

    function vote(uint index) public {
        if (voted[msg.sender][index]) return;
        voted[msg.sender][index] = true;

        uint voteCount = voteList[index];
        if (voteCount == 0) {
            // it is the first time this index is voted.
            // update it to numberOfVotedNodes
            numberOfVotedNodes = numberOfVotedNodes + 1;
        }
        voteList[index] = voteCount + 1;
    }

    function getVoteCount(uint index) public view returns (uint) {
        return voteList[index];
    }

    function getAvailablePeriod() public view returns (uint64, uint64) {
        return (fromTime, toTime);
    }
}
