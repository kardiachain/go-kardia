/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

pragma solidity ^0.5.8;

contract Staker {

    address constant PoSHandler = 0x0000000000000000000000000000000000000005;

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

    constructor(address master, address payable owner, uint lockedPeriod, uint256 minimumStake) public {
        _master = master;
        _owner = owner;
        _lockedPeriod = lockedPeriod;
        _minimumStake = minimumStake;
        StakeInfo memory emptyStakeInfo = StakeInfo(address(0x0), 0, 0);
        stakeInfo.push(emptyStakeInfo);
    }

    function stake(address node) public payable isOwner {
        (bool success, bytes memory result)  = _master.staticcall(abi.encodeWithSignature(isAvailableNodes, node));
        require(success, "calling isAvailableNodes to master failed");

        uint64 nodeIndex = abi.decode(result, (uint64));
        require(nodeIndex > 0, "address is not in available nodes");
        require (msg.value >= _minimumStake, "invalid stakeAmount");

        uint index = _hasStaked[node];
        if (index > 0) {
            stakeInfo[index].amount += msg.value;
        } else {
            stakeInfo.push(StakeInfo(node, block.number, msg.value));
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
            if (block.number-stakeInfo[index].startedAt > _lockedPeriod) {
                _owner.transfer(amount);
                stakeInfo[index].amount -= amount;
                (bool success, ) = _master.call(abi.encodeWithSignature(withdrawFunc, fromAddr, amount));
                require(success, "calling withdrawFunc to master failed");
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

    function saveReward(address node, uint64 blockHeight, uint256 amount) public isPoSHandler {
        rewards[node].push(RewardInfo(blockHeight, amount));
        totalRewards[node] += amount;
    }

    function withdrawReward(address node, uint256 amount) public isOwner {
        require(amount <= totalRewards[node], "insufficient amount");
        _owner.transfer(amount);
        totalRewards[node] -= amount;
    }
}
