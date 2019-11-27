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
 * Node contains node's information regard nodeId, nodeName, IpAddress, Port, Reward percentage for stakers.
 **/
contract Node {

    string _nodeId;
    string _nodeName;
    string _ipAddress;
    string _port;
    uint16 _rewardPercentage;
    address payable _owner;
    address _master;

    modifier isMaster {
        require(msg.sender == _master, "Only master can access this function");
        _;
    }

    constructor(address master, address payable owner, string memory nodeId, string memory nodeName, string memory ipAddress, string memory port, uint16 rewardPercentage) public {
        _master = master;
        _owner = owner;
        _nodeName = nodeName;
        _nodeId = nodeId;
        _ipAddress = ipAddress;
        _port = port;
        _rewardPercentage = rewardPercentage;
    }

    modifier isOwner {
        require(
            msg.sender == _owner,
            "Only owner can access this function"
        );
        _;
    }

    function getNodeInfo() public view returns(address owner, string memory nodeId, string memory nodeName, string memory ipAddress, string memory port, uint16 rewardPercentage, uint256 balance) {
        return (_owner, _nodeId, _nodeName, _ipAddress, _port, _rewardPercentage, address(this).balance);
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
}
