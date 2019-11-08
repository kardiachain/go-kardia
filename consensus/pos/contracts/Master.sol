/*

Master is a place that stores validator addresses, stakers, etc.
Note: most of the variables are being used in mapping instead of array because it is complicated to handle with array in solidity.
(eg: handling nested array; while by default, a map can be empty, an array is needed to be initialized.)

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
        mapping(uint64=>StakerInfo) stakerInfo;
        mapping(address=>uint64) stakerAdded;
    }

    struct PendingInfo {
        NodeInfo node;
        uint64 vote;
        mapping(address=>bool) votedAddress;
    }

    struct PendingDeleteInfo {
        NodeInfo node;
        uint64 index;
        uint64 vote;
        mapping(address=>bool) votedAddress;
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
    mapping(uint64=>NodeInfo) _availableNodes;
    mapping(address=>uint64) _availableAdded;

    // totalAvailableNodes is a counter for availableNodes
    uint64 _totalAvailableNodes;

    // pendingNodes is a map contains all pendingNodes that are added by availableNodes and are waiting for +2/3 vote.
    mapping(uint64=>PendingInfo) _pendingNodes;
    mapping(address=>uint64) _pendingAdded;

    // _totalPendingNodes is a counter for pendingNodes
    uint64 _totalPendingNodes = 1;

    // _startAtBlock stores started block in every consensusPeriod
    uint64 _startAtBlock;

    // _nextBlock stores started block for the next consensusPeriod
    uint64 _nextBlock;

    // _consensusPeriod indicates the period a consensus has.
    uint64 _consensusPeriod;

    uint64 _maxValidators;

    mapping(uint64=>PendingDeleteInfo) _pendingDeletedNodes;
    uint64 _totalPendingDeletedNodes = 1;
    mapping(address=>uint64) _deletingAdded;

    mapping(address=>bool) _stakers;

    constructor(uint64 consensusPeriod, uint64 maxValidators) public {
        _startAtBlock = 0;
        _nextBlock = 0;
        _consensusPeriod = consensusPeriod;
        _totalAvailableNodes = uint64(_genesisNodes.length);
        _totalPendingNodes = 0;
        _maxValidators = maxValidators;

        // init empty data at the first element.
        _availableNodes[0] = NodeInfo(0x0, 0x0, 0, 0);
        _pendingNodes[0] = PendingInfo(_availableNodes[0], 0);
        _pendingDeletedNodes[0] = PendingDeleteInfo(_availableNodes[0], 0, 0);

        for (uint64 i=0; i < _totalAvailableNodes; i++) {
            address genesisAddress = _genesisNodes[i];
            _isGenesis[genesisAddress] = true;
            _isGenesisOwner[_genesisOwners[i]] = true;

            // omit [0] element to use it to check exists or not element.
            // Add 2 here to point that the next index is 2 after adding genesis staker.

            _availableNodes[i+1] = NodeInfo(genesisAddress, _genesisOwners[i], 0, 1);
            _availableNodes[i+1].stakerInfo[0] = StakerInfo(0x0, 0);
            _availableAdded[genesisAddress] = i+1;
        }
        _totalAvailableNodes += 1;
    }

    // addNode adds a node to pending list
    function addPendingNode(address nodeAddress, address owner) public isAvailableNodes {
        if (_pendingAdded[nodeAddress] == 0) {
            _pendingNodes[_totalPendingNodes] = PendingInfo(NodeInfo(nodeAddress, owner, 0, 1), 1);
            _pendingNodes[_totalPendingNodes].votedAddress[msg.sender] = true;

            _pendingAdded[nodeAddress] = _totalPendingNodes;
            _totalPendingNodes += 1;
        }
    }

    function GetPendingNode(uint64 index) public view returns (address nodeAddress, uint256 stakes, uint64 vote) {
        if (index > 0 && index < _totalPendingNodes) {
            PendingInfo storage info = _pendingNodes[index];
            return (info.node.node, info.node.stakes, info.vote);
        }
    }

    function GetTotalPending() public view returns (uint64) {
        return _totalPendingNodes - 1;
    }

    function GetTotalAvailableNodes() public view returns (uint64) {
        return _totalAvailableNodes - 1;
    }

    function GetTotalPendingDelete() public view returns (uint64) {
        return _totalPendingDeletedNodes - 1;
    }

    // votePending is used when a valid user (belongs to availableNodes) vote for a node.
    function votePending(uint64 index) public isAvailableNodes {
        if (index > 0 && index < _totalPendingNodes) {
            PendingInfo storage info = _pendingNodes[index];
            if (!info.votedAddress[msg.sender]) {
                _pendingNodes[index].vote += 1;
                _pendingNodes[index].votedAddress[msg.sender] = true;

                // if vote >= 2/3 _totalAvailableNodes then update node to availableNodes
                if (isQualified(info.vote)) {
                    updatePending(index);
                }
            }
        }
    }

    // requestDelete requests delete an availableNode based on its index in _availableNodes.
    function requestDelete(uint64 index) public isAvailableNodes {
        // if index is in _availableNodes boundary
        if (index >= _totalAvailableNodes) return;
        // get node from availableNodes
        NodeInfo storage node = _availableNodes[index];

        // if node is not genesis
        if (!_isGenesis[node.node] && _deletingAdded[node.node] > 0) {
            _pendingDeletedNodes[_totalPendingDeletedNodes] = PendingDeleteInfo(node, index, 1);
            _pendingDeletedNodes[_totalPendingDeletedNodes].votedAddress[msg.sender] = true;
            _deletingAdded[node.node] = _totalPendingDeletedNodes;
            _totalPendingDeletedNodes += 1;
        }

    }

    // voteDeleting votes to delete an availableNode based on index in _pendingDeletedNodes
    function voteDeleting(uint64 index) public isAvailableNodes {
        if (index > 0 && index < _totalPendingDeletedNodes) {
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
    }

    // isQualified checks if vote count is greater than or equal with 2/3 total or not.
    function isQualified(uint64 count) internal view returns (bool) {
        return count >= ((_totalAvailableNodes-1) * 2)/3;
    }

    // updatePending updates current index into _availableNodes
    function updatePending(uint64 index) internal {

        if (index == 0 || index >= _totalPendingNodes) {
            return;
        }

        // get pending info at index
        PendingInfo storage info = _pendingNodes[index];

        // append pending info to availableNodes
        _availableNodes[_totalAvailableNodes] = info.node;
        _availableAdded[info.node.node] = _totalAvailableNodes;
        _totalAvailableNodes += 1;

        if (index != _totalPendingNodes - 1) { // index is not last element
            // loop through pending info, remove current index, and re-index the rest.
            while (index < _totalPendingNodes - 1) {
                // get next info and assign to current index
                PendingInfo storage nextNode = _pendingNodes[index + 1];
                _pendingNodes[index] = nextNode;
                _pendingAdded[nextNode.node.node] = index;
                index += 1;
            }
        }
        // delete last element to prevent duplicate.
        _totalPendingNodes -= 1;
    }

    function deleteAvailableNode(uint64 index) internal {
        if (index == 0 || index >= _totalAvailableNodes) return;

        // get node info from index
        NodeInfo storage nodeInfo = _availableNodes[index];

        // update _availableAdded to false
        _availableAdded[nodeInfo.node] = 0;

        if (index != _totalAvailableNodes - 1) { // index is not last element
            while (index < _totalAvailableNodes - 1) {
                NodeInfo storage nextNode = _availableNodes[index + 1];
                _availableNodes[index] = nextNode;
                _availableAdded[nextNode.node] = index;
                index += 1;
            }
        }
        _totalAvailableNodes -= 1;
    }

    function updateDeletePending(uint64 index) internal {
        if (index >= _totalPendingDeletedNodes) {
            return;
        }
        // get delete pending info at index
        PendingDeleteInfo storage info = _pendingDeletedNodes[index];

        // delete availableNodes
        deleteAvailableNode(info.index);

        // update _deletingAdded to false
        _deletingAdded[info.node.node] = 0;

        if (index != _totalPendingDeletedNodes - 1) { // index is not last element
            // loop through pending info, remove current index, and re-index the rest.
            while (index < _totalPendingDeletedNodes - 1) {
                // get next info and assign to current index
                PendingDeleteInfo storage nextNode = _pendingDeletedNodes[index + 1];
                _pendingDeletedNodes[index] = nextNode;
                _pendingAdded[nextNode.node.node] = index;
                index += 1;
            }
        }
        // delete last element to prevent duplicate.
        _totalPendingDeletedNodes -= 1;
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
        if (amount == 0) return;

        uint64 index = _availableAdded[nodeAddress];
        if (index > 0) { // sender must be owner of the node.
            // update stakes
            _availableNodes[index].stakes += amount;

            // add staker to stake info if it does not exist, otherwise update stakerInfo
            if (_availableNodes[index].stakerAdded[msg.sender] > 0) {
                uint64 stakerIndex = _availableNodes[index].stakerAdded[msg.sender];
                // update StakeInfo
                _availableNodes[index].stakerInfo[stakerIndex].amount += amount;
            } else { // staker does not exist
                uint64 newIndex = _availableNodes[index].totalStaker;
                _availableNodes[index].stakerAdded[msg.sender] = newIndex;
                _availableNodes[index].stakerInfo[newIndex] = StakerInfo(msg.sender, amount);
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

    // withdraw: after user chooses withdraw, delegateCall will call this function from user's staker contract to update node's stakes
    function withdraw(address nodeAddress, uint256 amount) public isStaker {
        uint64 index = _availableAdded[nodeAddress];
        if (index > 0 && _availableNodes[index].stakerAdded[msg.sender] > 0) {
            uint64 stakerIndex = _availableNodes[index].stakerAdded[msg.sender];

            // update total stakes. subtract old amount and add new amount.
            _availableNodes[index].stakes -= _availableNodes[index].stakerInfo[stakerIndex].amount;
            _availableNodes[index].stakes += amount;

            // update staker's stakes
            _availableNodes[index].stakerInfo[stakerIndex].amount = amount;

            // re-index node.
            while (index < _totalAvailableNodes-1) {
                if (_availableNodes[index].stakes > _availableNodes[index+1].stakes) break;
                // update _availableAdded
                _availableAdded[_availableNodes[index].node] = index+1;
                _availableAdded[_availableNodes[index+1].node] = index;

                // switch node
                NodeInfo memory temp = _availableNodes[index+1];
                _availableNodes[index+1] = _availableNodes[index];
                _availableNodes[index] = temp;
            }
        }
    }

    function collectValidators() public isValidatorOrGenesis {
        // update _startAtBlock and _nextBlock
        _startAtBlock = _nextBlock;
        _nextBlock += _consensusPeriod;
        _history.push(Validators(1, _startAtBlock, _nextBlock-1));
        _history[_history.length-1].nodes[0] = _availableNodes[0];

        // get len based on _totalAvailableNodes and _maxValidators
        uint64 len = _totalAvailableNodes-1;
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
        uint64 index = _availableAdded[node];
        if (index > 0) {
            return _availableNodes[index].stakes;
        }
        return 0;
    }

    function IsStaker(address staker) public view returns (bool) {
        return _stakers[staker];
    }

    function IsAvailableNodes(address node) public view returns (uint64) {
        return _availableAdded[node];
    }

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
        return _history[_history.length-1].totalNodes;
    }

    function GetLatestValidator(uint64 index) public view returns (address node, address owner, uint256 stakes, uint64 totalStaker) {
        uint64 len = getLatestValidatorsLength();
        if (index > len) return;
        NodeInfo memory validator = _history[_history.length-1].nodes[index];
        return (validator.node, validator.owner, validator.stakes, validator.totalStaker);
    }
}
