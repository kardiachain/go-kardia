pragma solidity ^0.4.24;

/**
 * Node contains node's information regard nodeId, nodeName, IpAddress, Port, Reward percentage for stakers.
 * because double is difficult to handle in solidity, therefore, percentage variable will be an integer and will be divided by 100 when calculating.
 * eg: 5% is 500
 **/
contract Node {

    string _nodeId;
    string _nodeName;
    string _ipAddress;
    string _port;
    uint16 _rewardPercentage;
    address _owner;
    address _master;

    modifier isMaster {
        require(msg.sender == _master, "Only master can access this function");
        _;
    }

    constructor(address master, address owner, string nodeId, string nodeName, string ipAddress, string port, uint16 rewardPercentage) public {
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

    function getNodeInfo() public view returns(address owner, string nodeId, string nodeName, string ipAddress, string port, uint16 rewardPercentage, uint256 balance) {
        return (_owner, _nodeId, _nodeName, _ipAddress, _port, _rewardPercentage, address(this).balance);
    }

    // reward KAI to user, calculating processes will be done at Kardia.
    // calculating processes include:
    // 1. get all users that stake for this node and theirs stake's amount
    // 2. from total stake, get contributed percentage amount.
    // 3. Get total reward for a node from Master contract (calculate by block).
    // 4. loop through list of stakers and call rewardToUser with amount calculated by the following formula:
    //    totalReward*(rewardPercentage/100)*(contributedPercentage)
    function rewardToUser(uint256 amount, address user) public isOwner {
        user.transfer(amount);
    }

    function getOwner() public view returns(address) {
        return _owner;
    }

    function withdraw() public isMaster {
        _owner.transfer(address(this).balance);
    }
}
