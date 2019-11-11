/*

Master is used to stores nodes info including available and pending nodes, stakers.

*/

pragma solidity ^0.4.24;


contract Master {

    address[] _genesisNodes = [
    0x0000000000000000000000000000000000000010,
    0x0000000000000000000000000000000000000011,
    0x0000000000000000000000000000000000000012
    ];

    address[] _genesisOwners = [
    0xc1fe56E3F58D3244F606306611a5d10c8333f1f6,
    0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5,
    0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd
    ];

    address[] _genesisStakers = [
    0x0000000000000000000000000000000000000020,
    0x0000000000000000000000000000000000000021,
    0x0000000000000000000000000000000000000022
    ];

    // genesisStakes initial stakes for genesis nodes.
    uint256 constant genesisStakes = 1000000000000000000000000000;

    mapping(address=>bool) _isGenesis;
    mapping(address=>bool) _isGenesisOwner;

    modifier isGenesis {
        require(
            _isGenesis[msg.sender] || _isGenesisOwner[msg.sender], "user does not have genesis permission"
        );
        _;
    }

    modifier isValidatorOrGenesis {
        bool result = _isGenesisOwner[msg.sender] || _isGenesis[msg.sender];
        if (!result) {
            result = IsValidator(msg.sender);
        }
        require(result, "sender is neither validator and genesis");
        _;
    }

    modifier isAvailableNodes {
        require(IsAvailableNodes(msg.sender) > 0, "sender does not belong in any availableNodes");
        _;
    }

    modifier isStaker {
        require(IsStaker(msg.sender), "user is not staker");
        _;
    }

    struct StakerInfo {
        address staker;
        uint256 amount;
    }

    struct NodeInfo {
        address node;
        address owner;
        uint256 stakes;
        uint64 totalStaker;
    }

    struct NodeIndex {
        mapping(uint64=>StakerInfo) stakerInfo;
        mapping(address=>uint64) stakerAdded;
    }

    struct PendingInfo {
        NodeInfo node;
        uint64 vote;
        mapping(address=>bool) votedAddress;
        bool done;
    }

    struct PendingDeleteInfo {
        NodeInfo node;
        uint64 index;
        uint64 vote;
        mapping(address=>bool) votedAddress;
        bool done;
    }

    struct Validators {
        uint64 totalNodes;
        uint64 startAtBlock;
        uint64 endAtBlock;
        mapping(uint64=>NodeInfo) nodes;
        mapping(address=>uint64) addedNodes;
    }

    // _history contains all validators through period.
    Validators[] _history;

    // availableNodes is used to mark all nodes passed the voting process or genesisNodes
    // use index as an index to make it easy to handle with update/remove list since it's a bit complicated handling list in solidity.
    NodeInfo[] _availableNodes;
    mapping(address=>uint) _availableAdded;
    mapping(address=>NodeIndex) _nodeIndex;

    // pendingNodes is a map contains all pendingNodes that are added by availableNodes and are waiting for +2/3 vote.
    PendingInfo[] _pendingNodes;
    mapping(address=>uint) _pendingAdded;

    // _startAtBlock stores started block in every consensusPeriod
    uint64 _startAtBlock;

    // _nextBlock stores started block for the next consensusPeriod
    uint64 _nextBlock;

    // _consensusPeriod indicates the period a consensus has.
    uint64 _consensusPeriod;

    uint64 _maxValidators;

    PendingDeleteInfo[] _pendingDeletedNodes;
    mapping(address=>uint) _deletingAdded;

    mapping(address=>bool) _stakers;

    constructor(uint64 consensusPeriod, uint64 maxValidators) public {
        _startAtBlock = 0;
        _nextBlock = 0;
        _consensusPeriod = consensusPeriod;
        _maxValidators = maxValidators;
        _availableNodes.push(NodeInfo(0x0, 0x0, 0, 0));
        _pendingNodes.push(PendingInfo(_availableNodes[0], 0, true));
        _pendingDeletedNodes.push(PendingDeleteInfo(_availableNodes[0], 0, 0, true));

        for (uint i=0; i < _genesisNodes.length; i++) {
            address genesisAddress = _genesisNodes[i];
            _isGenesis[genesisAddress] = true;
            _isGenesisOwner[_genesisOwners[i]] = true;

            _availableNodes.push(NodeInfo(genesisAddress, _genesisOwners[i], 0, 1));
            _availableAdded[genesisAddress] = i+1;
            _nodeIndex[genesisAddress].stakerInfo[0] = StakerInfo(address(0x0), 0);
        }
    }

    // addNode adds a node to pending list
    function addPendingNode(address nodeAddress, address owner) public isAvailableNodes {
        if (_pendingAdded[nodeAddress] == 0) {
            _pendingNodes.push(PendingInfo(NodeInfo(nodeAddress, owner, 0, 1), 1, false));
            _pendingNodes[_pendingNodes.length-1].votedAddress[msg.sender] = true;
            _pendingAdded[nodeAddress] = _pendingNodes.length-1;
        }
    }

    function GetPendingNode(uint64 index) public view returns (address nodeAddress, uint256 stakes, uint64 vote) {
        PendingInfo storage info = _pendingNodes[index];
        return (info.node.node, info.node.stakes, info.vote);
    }

    function GetTotalPending() public view returns (uint) {
        return _pendingNodes.length - 1;
    }

    function GetTotalAvailableNodes() public view returns (uint) {
        return _availableNodes.length - 1;
    }

    function GetTotalPendingDelete() public view returns (uint) {
        return _pendingDeletedNodes.length - 1;
    }

    function getAvailableNode(uint index) public view returns (address nodeAddress, address owner, uint256 stakes) {
        NodeInfo storage info = _availableNodes[index];
        return (info.node, info.owner, info.stakes);
    }

    function getAvailableNodeIndex(address node) public view returns (uint index) {
        return _availableAdded[node];
    }

    // votePending is used when a valid user (belongs to availableNodes) vote for a node.
    function votePending(uint64 index) public isAvailableNodes {
        require(index > 0 && index < _pendingNodes.length, "invalid index");
        if (!_pendingNodes[index].votedAddress[msg.sender]) {
            _pendingNodes[index].vote += 1;
            _pendingNodes[index].votedAddress[msg.sender] = true;

            // if vote >= 2/3 _totalAvailableNodes then update node to availableNodes
            if (isQualified(_pendingNodes[index].vote)) {
                updatePending(index);
            }
        }
    }

    // requestDelete requests delete an availableNode based on its index in _availableNodes.
    function requestDelete(uint64 index) public isAvailableNodes {
        require(index > 0 && index < _availableNodes.length, "invalid index");
        // get node from availableNodes
        NodeInfo storage node = _availableNodes[index];

        // if node is not genesis
        if (!_isGenesis[node.node] && _deletingAdded[node.node] == 0) {
            _pendingDeletedNodes.push(PendingDeleteInfo(node, index, 1, false));
            _pendingDeletedNodes[_pendingDeletedNodes.length-1].votedAddress[msg.sender] = true;
            _deletingAdded[node.node] = _pendingDeletedNodes.length-1;
        }
    }

    function getRequestDeleteNode(uint64 index) public view returns (uint64 nodeIndex, address nodeAddress, uint256 stakes, uint64 vote) {
        PendingDeleteInfo storage info = _pendingDeletedNodes[index];
        return (info.index, info.node.node, info.node.stakes, info.vote);
    }

    // voteDeleting votes to delete an availableNode based on index in _pendingDeletedNodes
    function voteDeleting(uint64 index) public isAvailableNodes {
        require(index > 0 && index < _pendingDeletedNodes.length, "invalid index");
        PendingDeleteInfo storage info = _pendingDeletedNodes[index];
        if (!info.votedAddress[msg.sender]) {
            info.vote += 1;
            info.votedAddress[msg.sender] = true;
            _pendingDeletedNodes[index] = info;

            // if vote >= 2/3 _totalAvailableNodes then update node to availableNodes
            if (isQualified(info.vote)) {
                updateDeletePending(index);
            }
        }
    }

    // isQualified checks if vote count is greater than or equal with 2/3 total or not.
    function isQualified(uint64 count) internal view returns (bool) {
        return count >= ((_availableNodes.length-1)*2/3) + 1;
    }

    // updatePending updates pending node into _availableNodes
    function updatePending(uint64 index) internal {
        require(index > 0 && index < _pendingNodes.length, "invalid index");
        // get pending info at index
        PendingInfo storage info = _pendingNodes[index];
        _pendingNodes[index].done = true;
        if (_availableAdded[info.node.node] > 0) return;
        // append pending info to availableNodes
        _availableNodes.push(info.node);
        _availableAdded[info.node.node] = _availableNodes.length-1;
        _nodeIndex[info.node.node].stakerInfo[0] = StakerInfo(address(0x0), 0);
    }

    function hasPendingVoted(uint64 index) public view returns (bool) {
        return _pendingNodes[index].votedAddress[msg.sender];
    }

    function deleteAvailableNode(uint64 index) internal {
        require(index > 0 && index < _availableNodes.length, "invalid index");

        // get node info from index
        NodeInfo storage nodeInfo = _availableNodes[index];

        // update _availableAdded to false
        _availableAdded[nodeInfo.node] = 0;

        while (index < _availableNodes.length - 1) { // index is not last element
            NodeInfo storage nextNode = _availableNodes[index + 1];
            _availableNodes[index] = nextNode;
            _availableAdded[nextNode.node] = index;
            index += 1;
        }

        // remove index - now is the last element
        delete _availableNodes[index];
        _availableNodes.length--;
    }

    function updateDeletePending(uint64 index) internal {
        require(index > 0 && index < _pendingDeletedNodes.length, "invalid index");
        // get delete pending info at index
        PendingDeleteInfo storage info = _pendingDeletedNodes[index];

        // delete availableNodes
        deleteAvailableNode(info.index);

        // update _deletingAdded to false
        _deletingAdded[info.node.node] = 0;
        _pendingDeletedNodes[index].done = true;
    }

    function changeConsensusPeriod(uint64 consensusPeriod) public isGenesis {
        _consensusPeriod = consensusPeriod;
    }

    // migrateBalance is used for migrating to new version of Master, if there is any new update needed in the future.
    function migrateBalance(address newMasterVersion) public isGenesis {
        newMasterVersion.transfer(address(this).balance);
    }

    function addStaker(address staker) public isValidatorOrGenesis {
        _stakers[staker] = true;
    }

    // stake is called by using delegateCall in staker contract address. therefore msg.sender is staker's contract address
    function stake(address nodeAddress, uint256 amount) public isStaker {
        require(amount > 0, "invalid amount");

        uint index = _availableAdded[nodeAddress];
        if (index > 0) { // sender must be owner of the node.
            // update stakes
            _availableNodes[index].stakes += amount;

            // add staker to stake info if it does not exist, otherwise update stakerInfo
            if (_nodeIndex[nodeAddress].stakerAdded[msg.sender] > 0) {
                uint64 stakerIndex = _nodeIndex[nodeAddress].stakerAdded[msg.sender];
                // update StakeInfo
                _nodeIndex[nodeAddress].stakerInfo[stakerIndex].amount += amount;
            } else { // staker does not exist
                uint64 newIndex = _availableNodes[index].totalStaker;
                _nodeIndex[nodeAddress].stakerAdded[msg.sender] = newIndex;
                _nodeIndex[nodeAddress].stakerInfo[newIndex] = StakerInfo(msg.sender, amount);
                _availableNodes[index].totalStaker += 1;
            }

            // re-index node
            while(index > 1) {
                if (_availableNodes[index].stakes <= _availableNodes[index-1].stakes) break;
                // update _availableAdded
                _availableAdded[_availableNodes[index].node] = index-1;
                _availableAdded[_availableNodes[index-1].node] = index;

                // switch node
                NodeInfo memory temp = _availableNodes[index-1];
                _availableNodes[index-1] = _availableNodes[index];
                _availableNodes[index] = temp;
                index -= 1;
            }
        }
    }

    // withdraw: after user chooses withdraw, staker's contract will call this function to update node's stakes
    function withdraw(address nodeAddress, uint256 amount) public isStaker {
        uint index = _availableAdded[nodeAddress];
        require(index > 0 && _nodeIndex[nodeAddress].stakerAdded[msg.sender] > 0, "invalid index");

        uint64 stakerIndex = _nodeIndex[nodeAddress].stakerAdded[msg.sender];

        // update total stakes. subtract old amount and add new amount.
        _availableNodes[index].stakes -= _nodeIndex[nodeAddress].stakerInfo[stakerIndex].amount;
        _availableNodes[index].stakes += amount;

        // update staker's stakes
        _nodeIndex[nodeAddress].stakerInfo[stakerIndex].amount = amount;

        // re-index node.
        while (index < _availableNodes.length-1) {
            if (_availableNodes[index].stakes > _availableNodes[index+1].stakes) break;
            // update _availableAdded
            _availableAdded[_availableNodes[index].node] = index+1;
            _availableAdded[_availableNodes[index+1].node] = index;

            // switch node
            NodeInfo memory temp = _availableNodes[index+1];
            _availableNodes[index+1] = _availableNodes[index];
            _availableNodes[index] = temp;
            index++;
        }

    }

    // collectValidators base on available nodes, max validators, collect validators and start new consensus period.
    function collectValidators() public isValidatorOrGenesis {
        // update _startAtBlock and _nextBlock
        _startAtBlock = _nextBlock;
        _nextBlock += _consensusPeriod;
        _history.push(Validators(1, _startAtBlock, _nextBlock-1));
        _history[_history.length-1].nodes[0] = _availableNodes[0];

        // get len based on _totalAvailableNodes and _maxValidators
        uint len = _availableNodes.length-1;
        if (len > _maxValidators) len = _maxValidators;
        // check valid nodes.
        for (uint64 i=1; i <= len; i++) {
            if (_availableNodes[i].stakes == 0) continue;
            uint64 currentIndex = _history[_history.length-1].totalNodes;
            _history[_history.length-1].nodes[currentIndex] = _availableNodes[i];
            _history[_history.length-1].addedNodes[_availableNodes[i].node] = currentIndex;
            _history[_history.length-1].totalNodes += 1;
        }
    }

    function getTotalStakes(address node) public view returns (uint256) {
        uint index = _availableAdded[node];
        if (index > 0) {
            return _availableNodes[index].stakes;
        }
        return 0;
    }

    function IsStaker(address staker) public view returns (bool) {
        return _stakers[staker];
    }

    // IsAvailableNodes check whether an address belongs to any available node or not.
    function IsAvailableNodes(address node) public view returns (uint64) {
        uint total = GetTotalAvailableNodes();
        if (total == 0) return 0; // total available is empty
        for (uint64 i=1; i<=total; i++) {
            if (_availableNodes[i].node == node || _availableNodes[i].owner == node) {
                return i;
            }
        }
        return 0;
    }

    // IsValidator check an address whether it belongs into latest validator.
    function IsValidator(address sender) public view returns (bool) {
        if (_history.length == 0) return false;
        Validators memory validators = _history[_history.length-1];
        for (uint64 i=1; i < validators.totalNodes; i++) {
            address owner = _history[_history.length-1].nodes[i].owner;
            address node = _history[_history.length-1].nodes[i].node;
            if (owner == sender || node == sender) {
                return true;
            }
        }
        return false;
    }

    function getLatestValidatorsLength() public view returns (uint64) {
        if (_history.length == 0) return 0;
        return _history[_history.length-1].totalNodes-1;
    }

    function GetLatestValidator(uint64 index) public view returns (address node, address owner, uint256 stakes, uint64 totalStaker) {
        uint64 len = getLatestValidatorsLength();
        require(index <= len, "invalid index");
        NodeInfo memory validator = _history[_history.length-1].nodes[index];
        return (validator.node, validator.owner, validator.stakes, validator.totalStaker);
    }
}
