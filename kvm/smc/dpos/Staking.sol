// SPDX-License-Identifier: MIT
pragma solidity ^0.5.0;
import "./interfaces/IStaking.sol";
import "./interfaces/IValidator.sol";
import "./Minter.sol";
import "./Safemath.sol";
import "./Ownable.sol";
import "./Params.sol";
import "./EnumerableSet.sol";
import "./Validator.sol";

contract Staking is IStaking, Ownable {
    using EnumerableSet for EnumerableSet.AddressSet;
    using SafeMath for uint256;

    address internal _previousProposer; // last proposer address
    address[] public allVals; // list all validators
    mapping(address => address) public ownerOf; // Owner of the validator
    mapping(address => address) public valOf; // Validator of the owner
    mapping(address => uint256) public balanceOf; // Balance of the validator
    mapping(address => bool) public vote;
    uint256 public totalSupply = 5000000000 * 10**18; // Total Supply
    uint256 public totalBonded; // Total bonded
    uint256 public proposal;
    uint256 public totalVoted;
    address[] public valSets;
    mapping(address => EnumerableSet.AddressSet) private valOfDel; // validators of delegator
    Minter public minter; // minter contract
    address public params;

    // Functions with this modifier can only be executed by the validator
    modifier onlyValidator() {
        require(valOf[msg.sender] != address(0x0), "Ownable: caller is not the validator");
        _;
    }

    constructor() public {
        params = address(new Params());
        minter = new Minter(params);
       
    }

    // create new validator
    function createValidator(
        bytes32 name,
        uint256 rate, 
        uint256 maxRate, 
        uint256 maxChangeRate 
    ) external returns (address val) {
        require(ownerOf[msg.sender] == address(0x0), "Valdiator owner exists");
        require(
            maxRate <= 1 * 10 ** 18,
            "commission max rate cannot be more than 100%"
        );
        require(
            maxChangeRate <= maxRate,
            "commission max change rate can not be more than the max rate"
        );
        require(
            rate <= maxRate,
            "commission rate cannot be more than the max rate"
        );
        bytes memory bytecode = type(Validator).creationCode;
        bytes32 salt = keccak256(abi.encodePacked(name, rate, maxRate, 
            maxChangeRate, msg.sender));
        assembly {
            val := create2(0, add(bytecode, 32), mload(bytecode), salt)
        }
        IValidator(val).initialize(name, msg.sender, rate, maxRate, 
            maxChangeRate);
        
        emit CreatedValidator(
            name,msg.sender,rate,
            maxRate,maxChangeRate
        );

        allVals.push(val);
        ownerOf[msg.sender] = val;
        valOf[val] = msg.sender;
        IValidator(val).setParams(params);
    }

    function setParams(address _params) external onlyOwner {
        params = _params;
    }

    // Update signer address
    function updateSigner(address signerAddr) external onlyValidator {
        require(ownerOf[signerAddr] == address(0x0), "user already exists");
        address oldSignerAddr = valOf[msg.sender];
        valOf[msg.sender] = signerAddr;
        ownerOf[oldSignerAddr] = address(0x0);
        ownerOf[signerAddr] = msg.sender;
    }

    function allValsLength() external view returns(uint) {
        return allVals.length;
    }
    
    function finalize(
        address[] calldata _signers, 
        uint256[] calldata _votingPower, 
        bool[] calldata _signed
    ) external onlyOwner{
        uint256 previousTotalPower = 0;
        uint256 sumPreviousPrecommitPower = 0;
        for (uint256 i = 0; i < _votingPower.length; i++) {
            previousTotalPower += _votingPower[i];
            if (_signed[i]) {
                sumPreviousPrecommitPower += _votingPower[i];
            }
        }
         if (block.number > 1) {
            _allocateTokens(sumPreviousPrecommitPower,
                previousTotalPower, _signers, _votingPower
            );
        }
        _previousProposer = block.coinbase;
        for (uint256 i = 0; i < _votingPower.length; i++) {
            _validateSignature(_signers[i], _votingPower[i], _signed[i]);
        }
    }

    function _allocateTokens(
        uint256 sumPreviousPrecommitPower,
        uint256 totalPreviousVotingPower,
        address[] memory _signers,
        uint256[] memory powers
    ) private {
        uint256 previousFractionVotes = sumPreviousPrecommitPower.divTrun(
            totalPreviousVotingPower
        );
        uint256 proposerMultiplier = IParams(params).getBaseProposerReward().add(
            IParams(params).getBonusProposerReward().mulTrun(previousFractionVotes)
        );

        uint256 fees = minter.feesCollected();
        uint256 proposerReward = fees.mulTrun(proposerMultiplier);
        _allocateTokensToValidator(_previousProposer, proposerReward);

        uint256 voteMultiplier = 1 * 10**18;
        voteMultiplier = voteMultiplier.sub(proposerMultiplier);
        for (uint256 i = 0; i < _signers.length; i++) {
            uint256 powerFraction = powers[i].divTrun(totalPreviousVotingPower);
            uint256 _rewards = fees.mulTrun(voteMultiplier).mulTrun(
                powerFraction
            );
            _allocateTokensToValidator(_signers[i], _rewards);
        }
    }

    function _allocateTokensToValidator(address signerAddr, uint256 _rewards) private{
        IValidator(ownerOf[signerAddr]).allocateToken(_rewards);
    }

    function _validateSignature( address signerAddr, uint256 votingPower, bool signed) private {
        IValidator val = IValidator(ownerOf[signerAddr]);
        val.validateSignature(votingPower, signed);
    }

    function withdrawRewards(address payable to, uint256 amount) external onlyValidator {
        to.transfer(amount);
    }

    function delegate(uint256 amount) external onlyValidator {
        _delegate(msg.sender, amount);
    }

    function _delegate(address from, uint256 amount) private {
        totalBonded = totalBonded.add(amount);
        balanceOf[from] = balanceOf[from].add(amount);
    }

    function undelegate(uint256 amount) external onlyValidator {
        _undelegate(msg.sender, amount);
    }

    function _undelegate(address from, uint256 amount) private {
        totalBonded = totalBonded.sub(amount);
        balanceOf[from] = balanceOf[from].sub(amount);
    }

    function removeDelegation(address delAddr) external onlyValidator{
        valOfDel[delAddr].remove(msg.sender);
    }

    function addDelegation(address delAddr) external onlyValidator{
        valOfDel[delAddr].add(msg.sender);
    }


    function burn(uint256 amount) external onlyValidator{
        _burn(msg.sender, amount);
    }

    function _burn(address from, uint256 amount) private {
        totalBonded = totalBonded.sub(amount);
        totalSupply = totalSupply.sub(amount);
        balanceOf[from] = balanceOf[from].sub(amount);
        emit Burn(from, amount);
    }

    // slash and jail validator forever
    function doubleSign(
        address signerAddr,
        uint256 votingPower,
        uint256 distributionHeight
    ) external onlyOwner {
        IValidator(ownerOf[signerAddr]).doubleSign(votingPower, distributionHeight);
    }

    function mint() external onlyOwner returns (uint256) {
        uint256 fees =  minter.mint(); 
        totalSupply = totalSupply.add(fees);
        emit Mint(fees);
        return fees;
    }

    // get validators of the delegator
    function getValidatorsByDelegator(address delAddr)
        public
        view
        returns (address[] memory)
    {
        uint256 total = valOfDel[delAddr].length();
        address[] memory valAddrs = new address[](total);
        for (uint256 i = 0; i < total; i++) {
            valAddrs[i] = valOfDel[delAddr].at(i);
        }
        return valAddrs;
    }

    function startValidator() external onlyValidator {
        if (valSets.length < IParams(params).getMaxValidators()) {
            valSets.push(msg.sender);
            return;
        }
        uint256 toStop;
        uint256 minAmount = balanceOf[valSets[0]];
        for (uint i = 0; i < valSets.length; i ++) {
            require(valSets[i] != msg.sender);
            if (balanceOf[valSets[i]] < minAmount) {
                toStop = i;
                minAmount = balanceOf[valSets[i]];
            }
        }

        require(balanceOf[msg.sender] > minAmount, "Amount must greater than min amount");
        _stopValidator(toStop);
        valSets[toStop] = msg.sender;
    }

    function _stopValidator(uint setIndex) private {
        IValidator(valSets[setIndex]).stop();
    }

    function _isProposer(address _valAddr) private view returns (bool) {
        for (uint i = 0; i < valSets.length; i++) {
            if (valOf[valSets[i]] == _valAddr) {
                return true;
            }
        }
        return false;
    }

    function removeFromSets() external onlyValidator {
        for (uint i = 0; i < valSets.length; i ++) {
            if (valSets[i] == msg.sender) {
                valSets[i] = valSets[valSets.length - 1];
                valSets.pop();
            }
        }
    } 

    // get current validator sets
    function getValidatorSets() external view returns (address[] memory, uint256[] memory) {
        uint256 total = valSets.length;
        address[] memory signerAddrs = new address[](total);
        uint256[] memory votingPowers = new uint256[](total);
        for (uint i = 0; i < total; i++) {
            address valAddr = valSets[i];
            signerAddrs[i] = valOf[valAddr];
            votingPowers[i] = balanceOf[valAddr].div(1 * 10 ** 8);
        }
        return (signerAddrs, votingPowers);
    }

    function setMaxValidators(uint256 _maxValidators) external onlyOwner {
        require(totalVoted >= ((2*sumVotingPowerProposer())/3), "Insufficient voting power");
        IParams(params).updateMaxValidator(_maxValidators);
        _resetVote();
    }

    function proposalMaxValidators(uint256 _maxValidators) external onlyOwner {
        proposal = _maxValidators;
    }

    function setProposalFail() external onlyOwner {
        _resetVote();
    }

    // resetVote performs after set max validator success or proposal fail
    function _resetVote() private {
        for (uint i = 0; i <  valSets.length; i++) {
            vote[valOf[valSets[i]]] = false;
        }
        totalVoted = 0;
    }

    function addVote() external {
        require(_isProposer(msg.sender) == true, "Not the proposer");
        require(vote[msg.sender] == false, "Vote only once");  
        vote[msg.sender] = true;
    }

    function sumVotingPowerProposer() public returns (uint256) {
        uint256 sumVotingPower;
        for (uint i = 0; i <  valSets.length; i++) {
            if (vote[valOf[valSets[i]]] == true) {
                totalVoted +=  balanceOf[valSets[i]].div(1 * 10 ** 8);
            }
            sumVotingPower += balanceOf[valSets[i]].div(1 * 10 ** 8);
        }
        return sumVotingPower;
    }
    
    function setPreviousProposer(address previousProposer) external onlyOwner {
        _previousProposer = previousProposer;
    }

    function setMintParams(
        uint256 _inflationRateChange,
        uint256 _goalBonded,
        uint256 _blocksPerYear,
        uint256 _inflationMax,
        uint256 _inflationMin
    ) external onlyOwner {
        IParams(params).updateMintParams(_inflationRateChange, _goalBonded, _blocksPerYear, _inflationMax, _inflationMin);
    }

    function setValidatorParams(
        uint256 _downtimeJailDuration,
        uint256 _slashFractionDowntime,
        uint256 _unbondingTime,
        uint256 _slashFractionDoubleSign,
        uint256 _signedBlockWindow,
        uint256 _minSignedPerWindow,
        uint256 _minStake,
        uint256 _minValidatorBalance
    ) external onlyOwner {
        IParams(params).updateValidatorParams(
            _downtimeJailDuration,
            _slashFractionDowntime,
            _unbondingTime,
            _slashFractionDoubleSign,
            _signedBlockWindow,
            _minSignedPerWindow,
            _minStake,
            _minValidatorBalance
        );
    }

    function deposit() external payable {
    }
}