// SPDX-License-Identifier: MIT
pragma solidity ^0.5.0;
import "./EnumerableSet.sol";
import "./interfaces/IValidator.sol";
import "./interfaces/IStaking.sol";
import {IParams} from "./interfaces/IParams.sol";


import "./Safemath.sol";
import "./Ownable.sol";


contract Validator is IValidator, Ownable {
    using SafeMath for uint256;
    using EnumerableSet for EnumerableSet.AddressSet;
    using SafeMath for uint256;

    uint256 oneDec = 1 * 10 ** 18;
    uint256 powerReduction = 1 * 10 ** 10;

    enum Status { Unbonding, Unbonded, Bonded}

    /*
     * DelStartingInfo represents the starting info for a delegator reward
     * period. It tracks the previous validator period, the delegation's amount of
     * staking token, and the creation height (to check later on if any slashes have
     * occurred)
    */
    struct Delegation {
        uint256 stake; // share delegator's
        uint256 previousPeriod; // previousPeriod uses calculates reward
        uint256 height; // creation height
        uint256 shares;
        address owner;
    }

    // Validator Commission
    struct Commission {
        // the commission rate charged to delegators, as a fraction
        uint256 rate;
        // maximum commission rate which validator can ever charge, as a fraction
        uint256 maxRate;
        // maximum daily increase of the validator commission, as a fraction
        uint256 maxChangeRate;
    }

    // Unbounding Entry
    struct UBDEntry {
         // KAI to receive at completion
        uint256 amount;
        // height which the unbonding took place
        uint256 blockHeight;
        // unix time for unbonding completion
        uint256 completionTime;
    }

    // validator slash event
    struct SlashEvent {
        uint256 period; // slash validator period 
        uint256 fraction; // fraction slash rate
        uint256 height;
    }

    /*
     * CurrentReward represents current rewards and current period for 
     * a validator kept as a running counter and incremented each block 
     * as long as the validator's tokens remain constant.
    */
    struct CurrentReward {
        uint256 period;
        uint256 reward;
    }

    /* 
     * HistoricalReward represents historical rewards for a validator.
     * Height is implicit within the store key.
     * cumulativeRewardRatio is the sum from the zeroeth period
     * until this period of rewards / tokens, per the spec.
     * The referenceCount indicates the number of objects
     * which might need to reference this historical entry at any point.
     * ReferenceCount = number of outstanding delegations which ended the associated period (and might need to read that record)
     *   + number of slashes which ended the associated period (and might need to
     *  read that record)
     *   + one per validator for the zeroeth period, set on initialization
    */
    struct HistoricalReward {
        uint256 cumulativeRewardRatio;
        uint256 referenceCount;
    }

    // SigningInfo defines a validator's signing info for monitoring their+
    // liveness activity.
    struct SigningInfo {
        // height at which validator was first a candidate OR was unjailed
        uint256 startHeight;
        // index offset into signed block bit array
        uint indexOffset;
        // whether or not a validator has been tombstoned (killed out of validator set)
        bool tombstoned;
        // missed blocks counter 
        uint missedBlockCounter;
        // time for which the validator is jailed until.
        uint256 jailedUntil;
    }
    
    struct InforValidator {
        bytes32 name;  // validator name
        address signer; // address of the validator
        uint256 tokens; // all token stake
        bool jailed;
        uint256 delegationShares; // delegation shares
        uint256 accumulatedCommission;
        uint256 ubdEntryCount; // unbonding delegation entries
        uint256 updateTime; // last update time
        uint256 minSelfDelegation;
        Status status; // validator status
        uint256 unbondingTime; // unbonding time
        uint256 unbondingHeight; // unbonding height
    }
    
    EnumerableSet.AddressSet private delegations; // all delegations
    mapping(address => Delegation) public delegationByAddr; // delegation by address
    mapping(uint256 => HistoricalReward) hRewards;
    mapping(address => UBDEntry[]) public ubdEntries;
    
    InforValidator public inforValidator;
    Commission public commission; // validator commission
    CurrentReward private currentRewards;// current validator rewards
    HistoricalReward private historicalRewards; // historical rewards
    SlashEvent[] public slashEvents; // slash events
    SigningInfo public signingInfo; // signing info
    mapping(uint => bool) private missedBlock; // missed block
    address public params;
    address public treasury;
    IStaking private _staking;

     // Functions with this modifier can only be executed by the validator
    modifier onlyValidator() {
        require(inforValidator.signer == msg.sender, "Ownable: caller is not the validator");
        _;
    }

    modifier onlyDelegator() {
        require(delegations.contains(msg.sender), "delegation not found");
        _;
    }

    constructor() public {
        transferOwnership(msg.sender);
        _staking = IStaking(msg.sender);
    }
    
    // called one by the staking at time of deployment  
    function initialize (
        bytes32 _name, 
        address _signer,
        uint256 _rate, 
        uint256 _maxRate, 
        uint256 _maxChangeRate
    ) external onlyOwner {
        inforValidator.name = _name;
        inforValidator.signer = _signer;
        inforValidator.updateTime = block.timestamp;
        inforValidator.status = Status.Unbonded;
                
        commission = Commission({
            maxRate: _maxRate,
            maxChangeRate: _maxChangeRate,
            rate: _rate
        });
        
        _initializeValidator();
    }

    function setParams(address _params) external onlyOwner {
        params = _params;
    }

    function setTreasury(address _treasury) external onlyOwner {
        treasury = _treasury;
    }
    
    // update signer address
    function updateSigner(address signerAddr) external onlyValidator {
        require(signerAddr != msg.sender);
        emit UpdatedSigner(inforValidator.signer, signerAddr);
        inforValidator.signer = signerAddr;
        _staking.updateSigner(signerAddr);
    }
    
    // delegate for this validator
    function delegate() external payable {
        _delegate(msg.sender, msg.value);
        _staking.delegate(msg.value);
        _staking.addDelegation(msg.sender);
    }

    function selfDelegate(address payable val, uint256 amount) external onlyOwner {
         _delegate(val, amount);
        _staking.delegate(amount);
        _staking.addDelegation(val);
    }

    function _updateName(bytes32 _name) private {
        inforValidator.name = _name;
    }

    function _updateCommissionRate(uint256 _commissionRate) private {
        if (_commissionRate > 0) {
            require(
                // solhint-disable-next-line not-rely-on-time
                block.timestamp.sub(inforValidator.updateTime) >= 86400,
                "commission cannot be changed more than one in 24h"
            );
            require(_commissionRate <= commission.maxRate, "commission cannot be more than the max rate");
            if (_commissionRate > commission.rate) {
                require(
                    _commissionRate.sub(commission.rate) <=
                        commission.maxChangeRate,
                    "commission cannot be changed more than max change rate"
                );
            }
            commission.rate = _commissionRate;
            inforValidator.updateTime = block.timestamp;
        }
    }

    // update validate info
    function updateCommissionRate(uint256 _commissionRate) external onlyValidator {
        _updateCommissionRate(_commissionRate);
        emit UpdateCommissionRate(_commissionRate);
    }

    function updateName(bytes32 _name) external payable onlyValidator {
        require(msg.value >= IParams(params).getMinAmountChangeName(), "Min amount is 10000 KAI");
        _updateName(_name);
        emit UpdateName(_name);
         _staking.burn(msg.value, 1);
        address(uint160(address(treasury))).transfer(msg.value);
    }
    
    // _allocateTokens allocate tokens to a particular validator, splitting according to commission
    function allocateToken(uint256 _rewards) external onlyOwner {
        uint256 _commission = _rewards.mulTrun(commission.rate);
        uint256 shared = _rewards.sub(_commission);
        inforValidator.accumulatedCommission += _commission;
        currentRewards.reward += shared;
    }

    // Unjail is used for unjailing a jailed validator, thus returning
    // them into the bonded validator set, so they can begin receiving provisions
    // and rewards again.
    function unjail() external onlyValidator {
        require(inforValidator.jailed, "validator not jailed");
        // cannot be unjailed if tombstoned
        require(signingInfo.tombstoned == false, "validator jailed");
        require(signingInfo.jailedUntil < block.timestamp, "validator jailed");
        Delegation storage del = delegationByAddr[inforValidator.signer];
        uint256 tokens = _tokenFromShare(del.shares);
        require(
            tokens > 0,
            "self delegation too low to unjail"
        );

        signingInfo.jailedUntil = 0;
        inforValidator.jailed = false;
    }

    function undelegate() external onlyDelegator{
        Delegation storage del = delegationByAddr[msg.sender];
        uint256 amount = _tokenFromShare(del.shares);
        _undelegate(msg.sender, amount);
        _staking.undelegate(amount);
    }

    function undelegateWithAmount(uint256 _amount) external onlyDelegator{
        require(_checkUndelegateAmount(msg.sender, _amount) == true, "Undelegate amount invalid");
        require(ubdEntries[msg.sender].length < 7, "too many unbonding delegation entries");
        _undelegate(msg.sender, _amount);
        _staking.undelegate(_amount);
    }

    function _undelegate(address payable from, uint256 _amount) private {
        _withdrawRewards(from);
        Delegation storage del = delegationByAddr[from];
        uint256 shares = _shareFromToken(_amount);
        if (shares > del.shares) {
            shares = del.shares;
        }
        del.shares = del.shares.sub(shares);
        _initializeDelegation(from);

        uint256 amountRemoved = _removeDelShares(shares);

        if (_isUnbonding() && _isUnbondingComplete()) {
            _withdraw(from, amountRemoved);
        } else {
            inforValidator.ubdEntryCount++;
            uint256 completionTime = block.timestamp.add(IParams(params).getUnbondingTime());
            if (_isUnbonding()) {
                completionTime = inforValidator.unbondingTime;
            }
            ubdEntries[from].push(
                UBDEntry({
                    completionTime: completionTime,
                    blockHeight: block.number,
                    amount: amountRemoved
                })
            );
            emit Undelegate(from, amountRemoved, completionTime);
        }
        _stopIfNeeded();
    }

    function _isUnbonding() private view returns (bool) {
        return inforValidator.status == Status.Unbonding;
    }

    function _isUnbondingComplete() private view returns (bool) {
        return inforValidator.unbondingTime < block.timestamp;
    }

    function _isBonded() private view returns (bool) {
        return inforValidator.status == Status.Bonded;
    }

    function _checkUndelegateAmount(address _delAddr, uint256 _amount) private view returns (bool) {
         Delegation storage del = delegationByAddr[_delAddr];
         if (del.stake.sub(_amount) == 0) {
             return true;
         }
        if (del.stake.sub(_amount) >= IParams(params).getMinStake()) {
            return true;
        }
        return false;
    }
    
    // withdraw rewards from a delegation
    function withdrawRewards() external onlyDelegator {
        _withdrawRewards(msg.sender);
        _initializeDelegation(msg.sender);
    }
    
    // the validator withdraws commission
    function withdrawCommission() external onlyValidator {
        uint256 _commission = inforValidator.accumulatedCommission;
        require(_commission > 0, "no validator commission to reward");
        _staking.withdrawRewards(msg.sender, _commission);
        inforValidator.accumulatedCommission = 0;
        emit WithdrawCommissionReward(_commission);
    }
    
    // withdraw token delegator's
    function withdraw() external onlyDelegator {
        UBDEntry[] storage entries = ubdEntries[msg.sender];
        uint256 amount = 0;
        for (uint256 i = 0; i < entries.length; i++) {
            // solhint-disable-next-line not-rely-on-time
            if (entries[i].completionTime < block.timestamp) {
                amount = amount.add(entries[i].amount);
                entries[i] = entries[entries.length - 1];
                entries.pop();
                i--;
                inforValidator.ubdEntryCount--;
            }
        }
        _withdraw(msg.sender, amount);
    }

    function _withdraw(address payable to, uint256 amount) private {
        require(amount > 0, "no unbonding amount to withdraw");
        if (delegationByAddr[to].shares <= 100 && ubdEntries[to].length == 0) {
            _removeDelegation(to);
        }
        to.transfer(amount);
        emit Withdraw(to, amount);
    }
    
    function getCommissionRewards() external view returns(uint256) {
        return inforValidator.accumulatedCommission;
    }
    
    // get rewards from a delegation
    function getDelegationRewards(address _delAddr) external view returns (uint256) {
        require(delegations.contains(_delAddr), "delegation not found");
        Delegation memory del = delegationByAddr[_delAddr];
        uint256 rewards = _calculateDelegationRewards(
            _delAddr,
            currentRewards.period - 1
        );

        uint256 currentReward = currentRewards.reward;
        if (currentReward > 0) {
            uint256 stake = _tokenFromShare(del.shares);
            rewards += stake.mulTrun(currentReward.divTrun(inforValidator.tokens));
        }
        return rewards;
    }

    function getDelegations() public view returns (address[] memory, uint256[] memory) {
        uint256 total = delegations.length();
        address[] memory delAddrs = new address[](total);
        uint256[] memory shares = new uint256[](total);
        for (uint256 i = 0; i < total; i++) {
            address delAddr = delegations.at(i);
            delAddrs[i] = delAddr;
            shares[i] = delegationByAddr[delAddr].shares;
        }
        return (delAddrs, shares);
    }
    // validate validator signature, must be called once per validator per block
    function validateSignature(
        uint256 _votingPower,
        bool _signed
    ) external onlyOwner{
        // counts blocks the validator should have signed
        uint index = signingInfo.indexOffset % IParams(params).getSignedBlockWindow();
        signingInfo.indexOffset++;
        bool previous = missedBlock[index];
        bool missed = !_signed;
        if (!previous && missed) { // value has changed from not missed to missed, increment counter
            signingInfo.missedBlockCounter++;
            missedBlock[index] = true;
        }
        if (previous && !missed) { // value has changed from missed to not missed, decrement counter
            signingInfo.missedBlockCounter--;
            missedBlock[index] = false;
        }

        uint256 minHeight = signingInfo.startHeight.add(IParams(params).getSignedBlockWindow());
        uint minSignedPerWindow = IParams(params).getSignedBlockWindow().mulTrun(IParams(params).getMinSignedPerWindow());
        uint maxMissed = IParams(params).getSignedBlockWindow().sub(minSignedPerWindow);
        // if past the minimum height and the validator has missed too many blocks, punish them
        if (block.number > minHeight && signingInfo.missedBlockCounter > maxMissed) {
            if (!inforValidator.jailed) {
                _slash(block.number.sub(2), _votingPower, IParams(params).getSlashFractionDowntime());
                _jail(block.timestamp.add(IParams(params).getDowntimeJailDuration()), false);
                signingInfo.missedBlockCounter = 0;
                _resetMissedBlock(signingInfo.indexOffset);
                signingInfo.indexOffset = 0;
                emit Slashed(_votingPower, 1);       
            }
        }
    }

    function _resetMissedBlock(uint256 _indexOffset) private {
        for (uint i = 0; i < _indexOffset; i++) {
            missedBlock[i] = false;
        }
    }

    // remove delegation
    function _removeDelegation(address _delAddr) private {
        delegations.remove(_delAddr);
        delete delegationByAddr[_delAddr];
        _staking.removeDelegation(_delAddr);
    }
    
    // remove share delegator's
    function _removeDelShares(uint256 _shares) private returns (uint256) {
        uint256 remainingShares = inforValidator.delegationShares;
        uint256 issuedTokens = 0;
        remainingShares = remainingShares.sub(_shares);
        if (remainingShares == 0) {
            issuedTokens = inforValidator.tokens;
            inforValidator.tokens = 0;
        } else {
            issuedTokens = _tokenFromShare(_shares);
            inforValidator.tokens = inforValidator.tokens.sub(issuedTokens);
        }
        inforValidator.delegationShares = remainingShares;
        return issuedTokens;
    }
    
    function _updateValidatorSlashFraction(uint256 _fraction) private {
        uint256 newPeriod = _incrementValidatorPeriod();
        _incrementReferenceCount(newPeriod);
        slashEvents.push(SlashEvent({
            period: newPeriod,
            fraction: _fraction,
            height: block.number
        }));
    }

    // initialize starting info for a new validator
    function _initializeValidator() private {
        currentRewards.period = 1;
        currentRewards.reward = 0;
        inforValidator.accumulatedCommission = 0;
    }
    
    function _delegate(address payable _delAddr, uint256 _amount) private {
        require(_amount >= IParams(params).getMinStake(), "Amount must greater than min stake amount");
        // add delegation if not exists;
        if (!delegations.contains(_delAddr)) {
            delegations.add(_delAddr);
            _beforeDelegationCreated();
        } else {
            _beforeDelegationSharesModified(_delAddr);
        }
        uint256 shared = _addTokenFromDel(_amount);
        // increment stake amount
        Delegation storage del = delegationByAddr[_delAddr];
        del.shares = del.shares.add(shared);
        _initializeDelegation(_delAddr);
        emit Delegate(_delAddr, _amount);
    }

    function _beforeDelegationCreated() private {
        _incrementValidatorPeriod();
    }
    
    // increment validator period, returning the period just ended
    function _incrementValidatorPeriod() private returns (uint256) {
        uint256 previousPeriod = currentRewards.period.sub(1);
        uint256 current = 0;
        if (currentRewards.reward > 0) {
            current = currentRewards.reward.divTrun(inforValidator.tokens);
        }
        uint256 historical = hRewards[previousPeriod]
            .cumulativeRewardRatio;
        _decrementReferenceCount(previousPeriod);

        hRewards[currentRewards.period].cumulativeRewardRatio = historical.add(current);
        hRewards[currentRewards.period].referenceCount = 1;
        currentRewards.period++;
        currentRewards.reward = 0;
        return previousPeriod.add(1);
    }
    
    // decrement the reference count for a historical rewards value, and delete if zero references remain
    function _decrementReferenceCount(uint256 _period) private {
        hRewards[_period].referenceCount--;
        if (hRewards[_period].referenceCount == 0) {
            delete hRewards[_period];
        }
    }
    
    function _beforeDelegationSharesModified(address payable delAddr) private {
        _withdrawRewards(delAddr);
    }
    
    function _withdrawRewards(address payable _delAddr) private {
        uint256 endingPeriod = _incrementValidatorPeriod();
        uint256 rewards = _calculateDelegationRewards(_delAddr, endingPeriod);
        _decrementReferenceCount(delegationByAddr[_delAddr].previousPeriod);
        if (rewards > 0) {
            _staking.withdrawRewards(_delAddr, rewards);
        }
    }
    
    // calculate the total rewards accrued by a delegation
    function _calculateDelegationRewards(
        address _delAddr,
        uint256 endingPeriod
    ) private view returns (uint256) {
        // fetch starting info for delegation
        Delegation memory delegationInfo = delegationByAddr[_delAddr];
        uint256 rewards = 0;
        for (uint256 i = 0; i < slashEvents.length; i++) {
            SlashEvent memory slashEvent = slashEvents[i];
            if (
                slashEvent.height > delegationInfo.height &&
                slashEvent.height < block.number
            ) {
                uint256 _endingPeriod = slashEvent.period;
                if (_endingPeriod > delegationInfo.previousPeriod) {
                    rewards += _calculateDelegationRewardsBetween(
                        delegationInfo.previousPeriod,
                        slashEvent.period,
                        delegationInfo.stake
                    );
                    delegationInfo.stake = delegationInfo.stake.mulTrun(
                        oneDec.sub(slashEvent.fraction)
                    );
                    delegationInfo.previousPeriod = _endingPeriod;
                }
            }
        }
        rewards += _calculateDelegationRewardsBetween(
            delegationInfo.previousPeriod,
            endingPeriod,
            delegationInfo.stake
        );
        return rewards;
    }
    
    // calculate the rewards accrued by a delegation between two periods
    function _calculateDelegationRewardsBetween(
        uint256 _startingPeriod,
        uint256 _endingPeriod,
        uint256 _stake
    ) private view returns (uint256) {
        HistoricalReward memory starting = hRewards[_startingPeriod];
        HistoricalReward storage ending = hRewards[_endingPeriod];
        uint256 difference = ending.cumulativeRewardRatio.sub(
            starting.cumulativeRewardRatio
        );
        return _stake.mulTrun(difference); // return staking * (ending - starting)
    }
    
    function _shareFromToken(uint256 _amount) private view returns(uint256) {
        return inforValidator.delegationShares.
        mul(_amount).div(inforValidator.tokens);
    }
    
    // calculate share delegator's
    function _addTokenFromDel(uint256 _amount) private returns (uint256) {
        uint256 issuedShares = 0;
        if (inforValidator.tokens == 0) {
            issuedShares = oneDec;
        } else {
            issuedShares = _shareFromToken(_amount);
        }
        inforValidator.tokens = inforValidator.tokens.add(_amount);
        inforValidator.delegationShares = inforValidator.delegationShares.add(issuedShares);
        return issuedShares;
    }
    
    // initialize starting info for a new delegation
    function _initializeDelegation(address _delAddr) private {
        Delegation storage del = delegationByAddr[_delAddr];
        uint256 previousPeriod = currentRewards.period.sub(1);
        _incrementReferenceCount(previousPeriod);
        delegationByAddr[_delAddr].height = block.number;
        delegationByAddr[_delAddr].previousPeriod = previousPeriod;
        uint256 stake = _tokenFromShare(del.shares);
        delegationByAddr[_delAddr].stake = stake;
    }
    
    // increment the reference count for a historical rewards value
    function _incrementReferenceCount(uint256 _period) private {
        hRewards[_period].referenceCount++;
    }
    
    // token worth of provided delegator shares
    function _tokenFromShare(uint256 _shares) private view returns (uint256) {
        return _shares.mul(inforValidator.tokens).div(inforValidator.delegationShares);
    }
    
    function _slash(uint256 _infrationHeight, uint256 _power, uint256 _slashFactor) private {
        require(_infrationHeight <= block.number, "cannot slash infrations in the future");
        
        uint256 slashAmount = _power.mul(powerReduction).mulTrun(_slashFactor);
        if (_infrationHeight < block.number) {
            uint256 totalDel = delegations.length();
            for (uint256 i = 0; i < totalDel; i++) {
                address delAddr = delegations.at(i);
                UBDEntry[] storage entries = ubdEntries[delAddr];
                for (uint256 j = 0; j < entries.length; j++) {
                    UBDEntry storage entry = entries[j];
                    if (entry.amount == 0) continue;
                    // if unbonding started before this height, stake did not contribute to infraction;
                    if (entry.blockHeight < _infrationHeight) continue;
                    // solhint-disable-next-line not-rely-on-time
                    if (entry.completionTime < block.timestamp) {
                        // unbonding delegation no longer eligible for slashing, skip it
                        continue;
                    }
                    uint256 amountSlashed = entry.amount.mulTrun(_slashFactor);
                    entry.amount = entry.amount.sub(amountSlashed);
                    slashAmount = slashAmount.sub(amountSlashed);
                }
            }
        }

        uint256 tokensToBurn = slashAmount;
        if (tokensToBurn > inforValidator.tokens) {
            tokensToBurn = inforValidator.tokens;
        }

        if (inforValidator.tokens > 0) {
            uint256 effectiveFraction = tokensToBurn.divTrun(inforValidator.tokens);
            _updateValidatorSlashFraction(effectiveFraction);
        }

        inforValidator.tokens = inforValidator.tokens.sub(tokensToBurn);
        address(uint160(address(treasury))).transfer(tokensToBurn);
        _staking.burn(tokensToBurn, 0);
    }
    
    function _jail(uint256 _jailedUntil, bool _tombstoned) private {
        inforValidator.jailed = true;
        signingInfo.jailedUntil = _jailedUntil;
        signingInfo.tombstoned = _tombstoned;
        _staking.removeFromSets();
        _stop();
    }

    function _stopIfNeeded() private {
        if (!_isBonded()) return;
        uint256 minStake = IParams(params).getMinValidatorStake();
        if (inforValidator.jailed || inforValidator.tokens < minStake) {
            _stop();
           _staking.removeFromSets();
        }
    }

   function doubleSign(
        uint256 votingPower,
        uint256 distributionHeight
    ) external onlyOwner {
        _slash(
            distributionHeight.sub(1),
            votingPower,
            IParams(params).getSlashFractionDoubleSign()
        );
        // // (Dec 31, 9999 - 23:59:59 GMT).
        _jail(253402300799, true);

        emit Slashed(votingPower, 2);
    }

    // start start validator
    function start() external onlyValidator {
        require(inforValidator.status != Status.Bonded, "validator bonded");
        require(!inforValidator.jailed, "validator jailed");
        require(inforValidator.tokens.div(powerReduction) > 0, "zero voting power");
        require(inforValidator.tokens >= IParams(params).getMinValidatorStake(), "Address balance must greater or equal minimum validator balance");

        _staking.startValidator();
        inforValidator.status = Status.Bonded;
        signingInfo.startHeight = block.number;
        emit Started();
    }

    // stop validator
    function stop() external onlyOwner {
        _stop();
    }

    function _stop() private {
        inforValidator.status = Status.Unbonding;
        inforValidator.unbondingHeight = block.number;
        inforValidator.unbondingTime = block.timestamp.add(IParams(params).getUnbondingTime());
        emit Stopped();
    }

    function getUBDEntries(address delAddr)
        external
        view
        returns (uint256[] memory, uint256[] memory)
    {
        uint256 total = ubdEntries[delAddr].length;
        uint256[] memory balances = new uint256[](total);
        uint256[] memory completionTime = new uint256[](total);

        for (uint256 i = 0; i < total; i++) {
            completionTime[i] = ubdEntries[delAddr][i].completionTime;
            balances[i] = ubdEntries[delAddr][i].amount;
        }
        return (balances, completionTime);
    }

    function getMissedBlock()
        public
        view
        returns (bool[] memory)
    {
        bool[] memory _missedBlock = new bool[](IParams(params).getSignedBlockWindow());
        for (uint i = 0; i < IParams(params).getSignedBlockWindow(); i++) {
            _missedBlock[i] = missedBlock[i];
        }

        return _missedBlock;
    }

    function getDelegatorStake(address _delAddr)
        public
        view
        returns (uint256)
    {
        require(delegations.contains(_delAddr), "delegation not found");
        Delegation memory del = delegationByAddr[_delAddr];
        return _tokenFromShare(del.shares);
    }

    function getSlashEventsLength() public view returns(uint256) {
        return slashEvents.length;
    }

    function () external payable {
    }
}