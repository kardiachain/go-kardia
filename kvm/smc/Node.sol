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

/**
 * Node contains node's information regard to nodeId, nodeName, Reward percentage for stakers.
 **/
contract Node {

    string _nodeId;
    string _nodeName;
    uint16 _rewardPercentage;
    uint64[] _validatedBlocks;
    uint64[] _rejectedBlocks;
    uint64 _lockedPeriod;
    uint256 _minimumStakes;
    address payable _owner;
    address _master;

    modifier isMaster {
        require(msg.sender == _master, "Only master can access this function");
        _;
    }

    constructor(address master, string memory nodeId, string memory nodeName, uint16 rewardPercentage, uint64 lockedPeriod, uint256 minimumStakes) public {
        _master = master;
        _owner = msg.sender;
        _nodeName = nodeName;
        _nodeId = nodeId;
        _rewardPercentage = rewardPercentage;
        _lockedPeriod = lockedPeriod;
        _minimumStakes = minimumStakes;
    }

    modifier isOwner {
        require(
            msg.sender == _owner,
            "Only owner can access this function"
        );
        _;
    }

    function getNodeInfo() public view returns(address owner, string memory nodeId, string memory nodeName, uint16 rewardPercentage, uint64 lockedPeriod, uint256 minimumStakes, uint256 balance) {
        return (_owner, _nodeId, _nodeName, _rewardPercentage, _lockedPeriod, _minimumStakes, address(this).balance);
    }

    function getOwner() public view returns(address) {
        return _owner;
    }

    function getLockedPeriod() public view returns (uint64) {
        return _lockedPeriod;
    }

    function getMinimumStakes() public view returns (uint256) {
        return _minimumStakes;
    }

    // withdraw sends block's rewards to owner's address.
    function withdraw() public isOwner {
        _owner.transfer(address(this).balance);
    }

    // getBalance returns current balance of current's node.
    function getBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // updateNode updates current node information
    function updateNode(string memory nodeName, string memory nodeId, uint16 rewardPercentage, uint64 lockedPeriod, uint256 minimumStakes) public isOwner {
        _nodeName = nodeName;
        _nodeId = nodeId;
        _rewardPercentage = rewardPercentage;
        _lockedPeriod = lockedPeriod;
        _minimumStakes = minimumStakes;
    }

    function updateBlock(uint64 blockHeight, bool rejected) public isMaster {
        if (rejected) {
            _rejectedBlocks.push(blockHeight);
        } else {
            _validatedBlocks.push(blockHeight);
        }
    }

    // getNumberOfValidatedBlocks returns total validated blocks
    function getNumberOfValidatedBlocks() public view returns (uint) {
        return _validatedBlocks.length;
    }

    // getValidatedBlockHeightByIndex returns blockHeight stored in _validatedBlocks by its index.
    function getValidatedBlockHeightByIndex(uint64 index) public view returns (uint64) {
        require(index >= 0 && index < _validatedBlocks.length, "invalid index");
        return _validatedBlocks[index];
    }

    // getNumberOfRejectedBlocks returns total rejected blocks
    function getNumberOfRejectedBlocks() public view returns (uint) {
        return _rejectedBlocks.length;
    }

    // getRejectedBlockHeightByIndex returns rejected blockHeight by its index in _rejectedBlocks.
    function getRejectedBlockHeightByIndex(uint64 index) public view returns (uint64) {
        require(index >= 0 && index < _rejectedBlocks.length, "invalid index");
        return _rejectedBlocks[index];
    }

    // getRejectedValidatedInfo returns total rejected and validated blocks
    function getRejectedValidatedInfo() public view returns (uint rejectedBlocks, uint validatedBlocks) {
        return (_rejectedBlocks.length, _validatedBlocks.length);
    }
}
