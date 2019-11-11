pragma solidity ^0.5.8;

/**
 * Staker contains information of a user who wants to stake KAI to one or more nodes (candidates to become validators).
 * - When user send a raw transaction which contains Staker's required information and transaction's type SIGNUP_STAKER,
 * system will create one contract address for that staker
 * - When user sends a payable transaction with type STAKE, validator will read it and trigger Master to store the value.
 * - When user wants to withdraw, system will do the following steps:
 *  1/ user sends WITHDRAW type transaction.
 *  2/ check if (currentBlock-startedAt) <= _lockedPeriod or not, if it does, transfer staked KAI to user.
 *  3/ validators trigger Master's withdraw function to update stake amount in node.
 *  4/ Node which is staked by user will receive user tx and calculate percentage bonus to user and transfer reward to user.
 **/
contract Staker {

    string constant stakeFunc = "stake(address,uint256)";
    string constant withdrawFunc = "withdraw(address,uint256)";
    string constant isAvailableNodes = "IsAvailableNodes(address)";

    address _master;
    address payable _owner;
    uint _lockedPeriod;
    uint256 _minimumStake;

    struct StakeInfo {
        address node;
        uint startedAt;
        uint256 amount;
    }

    StakeInfo[] stakeInfo;
    mapping(address=>uint) _hasStaked;

    modifier isOwner() {
        require(msg.sender == _owner, "sender is not owner");
        _;
    }

    constructor(address master, address payable owner, uint lockedPeriod, uint256 minimumStake) public {
        _master = master;
        _owner = owner;
        _lockedPeriod = lockedPeriod;
        _minimumStake = minimumStake;
        StakeInfo memory emptyStakeInfo = StakeInfo(address(0x0), 0, 0);
        stakeInfo.push(emptyStakeInfo);
    }

    function stake(address node) public payable isOwner {
        (, bytes memory result)  = _master.staticcall(abi.encodeWithSignature(isAvailableNodes, node));
        uint64 nodeIndex = abi.decode(result, (uint64));
        require(nodeIndex > 0);

        if (msg.value == 0 || msg.value < _minimumStake) return;
        uint index = _hasStaked[node];
        if (index > 0) {
            stakeInfo[index].amount += msg.value;
        } else {
            stakeInfo.push(StakeInfo(node, block.number, msg.value));
            _hasStaked[node] = stakeInfo.length - 1;
        }
        _master.call(abi.encodeWithSignature(stakeFunc, node, msg.value));
    }

    function withdraw(address fromAddr, uint256 amount) public isOwner {
        uint index = _hasStaked[fromAddr];
        if (index > 0) {
            // check lockedPeriod with startedAt
            // only withdraw staked KAI here, after this is successfully withdrawn, staked node will send reward to user.
            if (block.number-stakeInfo[index].startedAt > _lockedPeriod) {
                _owner.transfer(amount);
                stakeInfo[index].amount -= amount;
                _master.call(abi.encodeWithSignature(withdrawFunc, fromAddr, amount));
            }
        }
    }

    function getStakeAmount(address node) public view returns (uint256 amount, bool valid) {
        uint index = _hasStaked[node];
        if (index > 0) {
            return (stakeInfo[index].amount, true);
        }
        return (0, false);
    }
}
