// SPDX-License-Identifier: MIT
pragma solidity =0.5.16;
import "./interfaces/IStaking.sol";
import "./interfaces/IValidator.sol";
import "./Minter.sol";
import "./Safemath.sol";
import "./Ownable.sol";
import "./EnumerableSet.sol";
import "./Validator.sol";

contract Staking is IStaking, Ownable {
    using EnumerableSet for EnumerableSet.AddressSet;
    using SafeMath for uint256;

    struct Params {
        uint256 baseProposerReward;
        uint256 bonusProposerReward;
        uint256 maxValidator;
    }

    address private _previousProposer; // last proposer address
    Params public  params; // staking params
    address[] public allVals; // list all validators
    mapping(address => address) public ownerOf; // Owner of the validator
    mapping(address => address) public valOf; // Validator of the owner
    mapping(address => uint256) public balanceOf; // Balance of the validator
    uint256 public totalSupply = 5000000000 * 10**18; // Total Supply
    uint256 public totalBonded; // Total bonded
    address[] public valSets;
    mapping(address => EnumerableSet.AddressSet) private valOfDel; // validators of delegator
    Minter public minter; // minter contract


    // Functions with this modifier can only be executed by the validator
    modifier onlyValidator() {
        require(valOf[msg.sender] != address(0x0), "Ownable: caller is not the validator");
        _;
    }

    constructor() public {
        params = Params({
            baseProposerReward: 1 * 10**16,
            bonusProposerReward: 4 * 10**16,
            maxValidator: 21
        });

        minter = new Minter();
    }

    function setParams(uint256 baseProposerReward, uint256 bonusProposerReward) external onlyOwner {
        params.baseProposerReward = baseProposerReward;
        params.bonusProposerReward = bonusProposerReward;
    }

    // create new validator
    function createValidator(
        bytes32 name,
        uint256 rate, 
        uint256 maxRate, 
        uint256 maxChangeRate, 
        uint256 minSelfDelegation
    ) external returns (address val) {
        require(ownerOf[msg.sender] == address(0x0), "Valdiator owner exists");
        bytes memory bytecode = type(Validator).creationCode;
        bytes32 salt = keccak256(abi.encodePacked(name, rate, maxRate, 
            maxChangeRate, minSelfDelegation, msg.sender));
        assembly {
            val := create2(0, add(bytecode, 32), mload(bytecode), salt)
        }
        IValidator(val).initialize(name, msg.sender, rate, maxRate, 
            maxChangeRate, minSelfDelegation);
        
        emit CreateValidator(
            name,
            msg.sender,
            rate,
            maxRate,
            maxChangeRate,
            minSelfDelegation
        );

        allVals.push(val);
        ownerOf[msg.sender] = val;
        valOf[val] = msg.sender;
    }

    // Update signer address
    function updateSigner(address signerAddr) external onlyValidator {
        require(ownerOf[signerAddr] == address(0x0));
        address oldSignerAddr = valOf[msg.sender];
        valOf[msg.sender] = signerAddr;
        ownerOf[oldSignerAddr] = address(0x0);
        ownerOf[signerAddr] = msg.sender;
    }

    function allValsLength() external view returns(uint) {
        return allVals.length;
    }

    function setPreviousProposer(address previousProposer) public onlyOwner {
        _previousProposer = previousProposer;
    }
    
    function setMaxValidator(uint256 _maxValidator) public onlyOwner {
        params.maxValidator = _maxValidator;
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
        uint256 proposerMultiplier = params.baseProposerReward.add(
            params.bonusProposerReward.mulTrun(previousFractionVotes)
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
        if (valSets.length < params.maxValidator) {
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
            votingPowers[i] = balanceOf[valAddr].div(1 * 10**8);
        }
        return (signerAddrs, votingPowers);
    }

    function deposit() external payable {
    }
}