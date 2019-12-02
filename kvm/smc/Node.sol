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
 * Node contains node's information regard nodeId, nodeName, Reward percentage for stakers.
 **/
contract Node {

    string _nodeId;
    string _nodeName;
    uint16 _rewardPercentage;
    uint64[] _validatedBlocks;
    address payable _owner;
    address _master;

    modifier isMaster {
        require(msg.sender == _master, "Only master can access this function");
        _;
    }

    constructor(address master, address payable owner, string memory nodeId, string memory nodeName, uint16 rewardPercentage) public {
        _master = master;
        _owner = owner;
        _nodeName = nodeName;
        _nodeId = nodeId;
        _rewardPercentage = rewardPercentage;
    }

    modifier isOwner {
        require(
            msg.sender == _owner,
            "Only owner can access this function"
        );
        _;
    }

    function getNodeInfo() public view returns(address owner, string memory nodeId, string memory nodeName, uint16 rewardPercentage, uint256 balance) {
        return (_owner, _nodeId, _nodeName, _rewardPercentage, address(this).balance);
    }

    function getOwner() public view returns(address) {
        return _owner;
    }

    function withdraw() public isOwner {
        _owner.transfer(address(this).balance);
    }

    function getBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // updateNode updates current node information
    function updateNode(string memory nodeName, string memory nodeId, uint16 rewardPercentage) public isOwner {
        _nodeName = nodeName;
        _nodeId = nodeId;
        _rewardPercentage = rewardPercentage;
    }

    // updateValidatedBlock adds validated block's height
    function updateValidatedBlock(uint64 blockHeight) public isMaster {
        _validatedBlocks.push(blockHeight);
    }

    // getNumberOfValidatedBlocks returns total validated blocks
    function getNumberOfValidatedBlocks() public view returns (uint256) {
        return _validatedBlocks.length;
    }

    // getValidatedBlockHeightByIndex returns blockHeight stored in _validatedBlocks by index.
    function getValidatedBlockHeightByIndex(uint64 index) public view returns (uint64) {
        require(index >= 0 && uint256(index) < _validatedBlocks.length, "invalid index");
        return _validatedBlocks[index];
    }
}
