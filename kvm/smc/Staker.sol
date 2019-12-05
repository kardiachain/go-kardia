pragma solidity ^0.5.8;


contract Staker {

    address constant PoSHandler = 0x0000000000000000000000000000000000000005;

    string constant stakeFunc = "stake(address,uint256)";
    string constant withdrawFunc = "withdraw(address,uint256)";
    string constant isAvailableNodes = "isAvailableNodes(address)";
    string constant getLockedPeriod = "getLockedPeriod()";
    string constant getMinimumStakes = "getMinimumStakes()";

    address _master;
    address payable _owner;

    struct StakeInfo {
        address node;
        uint startedAt;
        uint256 amount;
        uint64 lockedPeriod;
    }

    struct RewardInfo {
        uint64 blockHeight;
        uint256 amount;
    }

    StakeInfo[] stakeInfo;
    mapping(address=>uint) _hasStaked;
    mapping(address=>RewardInfo[]) rewards;
    mapping(address=>uint256) totalRewards;

    modifier isOwner() {
        require(msg.sender == _owner, "sender is not owner");
        _;
    }

    modifier isPoSHandler() {
        require(msg.sender == PoSHandler, "sender is not PoSHandler");
        _;
    }

    constructor(address master) public {
        _master = master;
        _owner = msg.sender;
        StakeInfo memory emptyStakeInfo = StakeInfo(address(0x0), 0, 0, 0);
        stakeInfo.push(emptyStakeInfo);
    }

    function stake(address node) public payable isOwner {
        (bool success, bytes memory result)  = _master.staticcall(abi.encodeWithSignature(isAvailableNodes, node));
        require(success, "calling isAvailableNodes to master failed");

        uint64 nodeIndex = abi.decode(result, (uint64));
        require(nodeIndex > 0, "address is not in available nodes");

        uint index = _hasStaked[node];
        if (index > 0) {
            stakeInfo[index].amount += msg.value;
        } else {
            // getLockedPeriod from node
            (success, result) = node.staticcall(abi.encodeWithSignature(getLockedPeriod));
            require(success, "failed to getLockedPeriod");
            uint64 lockedPeriod = abi.decode(result, (uint64));

            // getMinimumStakes from node
            (success, result) = node.staticcall(abi.encodeWithSignature(getMinimumStakes));
            require(success, "failed to getMinimumStakes");

            uint256 minimumStakes = abi.decode(result, (uint256));
            require (msg.value >= minimumStakes, "invalid stakeAmount");

            stakeInfo.push(StakeInfo(node, block.number, msg.value, lockedPeriod));
            _hasStaked[node] = stakeInfo.length - 1;
        }
        (success, ) = _master.call(abi.encodeWithSignature(stakeFunc, node, msg.value));
        require(success, "calling stakeFunc to master failed");
    }

    function withdraw(address fromAddr, uint256 amount) public isOwner {
        uint index = _hasStaked[fromAddr];
        if (index > 0) {
            // check lockedPeriod with startedAt
            // only withdraw staked KAI here, after this is successfully withdrawn, staked node will send reward to user.
            if (block.number-stakeInfo[index].startedAt > stakeInfo[index].lockedPeriod) {
                _owner.transfer(amount);
                stakeInfo[index].amount -= amount;
                (bool success, ) = _master.call(abi.encodeWithSignature(withdrawFunc, fromAddr, amount));
                require(success, "calling withdrawFunc to master failed");
            }
        }
    }

    function getStakeAmount(address node) public view returns (uint256 amount, uint startedAt, bool valid) {
        uint index = _hasStaked[node];
        if (index > 0) {
            return (stakeInfo[index].amount, stakeInfo[index].startedAt, true);
        }
        return (0, 0, false);
    }

    function saveReward(address node, uint64 blockHeight, uint256 amount) public isPoSHandler {
        rewards[node].push(RewardInfo(blockHeight, amount));
        totalRewards[node] += amount;
    }

    function withdrawReward(address node, uint256 amount) public isOwner {
        require(amount <= totalRewards[node], "insufficient amount");
        _owner.transfer(amount);
        totalRewards[node] -= amount;
    }

    function getOwner() public view returns (address) {
        return _owner;
    }
}
