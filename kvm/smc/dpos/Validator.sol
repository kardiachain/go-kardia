// SPDX-License-Identifier: MIT
pragma solidity =0.5.16;
import "./EnumerableSet.sol";
import "./interfaces/IValidator.sol";
import "./interfaces/IStaking.sol";

import "./Safemath.sol";
import "./Ownable.sol";


contract Validator is IValidator, Ownable {
    using SafeMath for uint256;
    using EnumerableSet for EnumerableSet.AddressSet;
    using SafeMath for uint256;

    uint256 oneDec = 1 * 10**18;
    uint256 powerReduction = 1 * 10**8;

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

    // SigningInfo defines a validator's signing info for monitoring their
    // liveness activity.
    struct SigningInfo {
        // height at which validator was first a candidate OR was unjailed
        uint256 startHeight;
        // index offset into signed block bit array
        uint256 indexOffset;
        // whether or not a validator has been tombstoned (killed out of validator set)
        bool tombstoned;
        // missed blocks counter 
        uint256 missedBlockCounter;
        // time for which the validator is jailed until.
        uint256 jailedUntil;
    }
    
    struct MissedBlock {
        mapping(uint256 => bool) items;
    }
    
    struct InforValidator {
        bytes32 name;  // validator name
        address valAddr; // address of the validator
        uint256 tokens; // all token stake
        bool jailed;
        uint256 minSelfDelegation;
        uint256 delegationShares; // delegation shares
        uint256 accumulatedCommission;
        uint256 ubdEntryCount; // unbonding delegation entries
        uint256 updateTime; // last update time
        Status status; // validator status
        uint256 unbondingTime; // unbonding time
        uint256 unbondingHeight; // unbonding height
    }
    
    struct Params {
        uint256 downtimeJailDuration;
        uint256 slashFractionDowntime;
        uint256 unbondingTime;
        uint256 slashFractionDoubleSign;
        uint256 signedBlockWindow;
        uint256 minSignedPerWindow;
        uint256 minStake;
    }

    uint256 constant public UNBONDING_TiME = 10; // 7 days
    
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
    MissedBlock private missedBlock; // missed block
    Params public params;
    IStaking private _staking;

     // Functions with this modifier can only be executed by the validator
    modifier onlyValidator() {
        require(inforValidator.valAddr == msg.sender, "Ownable: caller is not the validator");
        _;
    }

    constructor() public {
        transferOwnership(msg.sender);
        _staking = IStaking(msg.sender);

        params = Params({
            downtimeJailDuration: 259200,
            slashFractionDowntime: 1 * 10**14,
            unbondingTime: 1814400,
            slashFractionDoubleSign: 5 * 10**16,
            signedBlockWindow: 100,
            minSignedPerWindow: 5 * 10**16,
            minStake: 10000 * 10**18 // 10 000 kai
        });
    }
    
    // called one by the staking at time of deployment  
    function initialize (
        bytes32 _name, 
        address _owner,
        uint256 _rate, 
        uint256 _maxRate, 
        uint256 _maxChangeRate, 
        uint256 _minSelfDelegation
    ) external onlyOwner {
            
        require(
            _maxRate <= oneDec,
            "commission max rate cannot be more than 100%"
        );
        require(
            _maxChangeRate <= _maxRate,
            "commission max change rate can not be more than the max rate"
        );
        require(
            _rate <= _maxRate,
            "commission rate cannot be more than the max rate"
        );

        inforValidator.name = _name;
        inforValidator.minSelfDelegation = _minSelfDelegation;
        inforValidator.valAddr = _owner;
        inforValidator.updateTime = block.timestamp;
        inforValidator.status = Status.Unbonded;
        
        commission = Commission({
            maxRate: _maxRate,
            maxChangeRate: _maxChangeRate,
            rate: _rate
        });
        
        _initializeValidator();
        signingInfo.startHeight = block.number;
    }

    // update signer address
    function updateSigner(address signerAddr) external onlyValidator {
        inforValidator.valAddr = signerAddr;
        _staking.updateSigner(signerAddr);
    }
    
    // delegate for this validator
    function delegate() external payable {
        _delegate(msg.sender, msg.value);
        _staking.delegate(msg.value);
        _staking.addDelegation(msg.sender);
    }

    // update validate info
    function update(bytes32 _name, uint256 _commissionRate, uint256 _minSelfDelegation) external onlyValidator {
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

        if (_minSelfDelegation > 0) {
            require(
                _minSelfDelegation > inforValidator.minSelfDelegation,
                "minimum self delegation cannot be decrease"
            );
            require(
                _minSelfDelegation <= inforValidator.tokens,
                "self delegation below minimum"
            );
            inforValidator.minSelfDelegation = _minSelfDelegation;
        }
        
        if (_name[0] != 0) {
            inforValidator.name = _name;
        }

        emit UpdateValidator(_name, _commissionRate, _minSelfDelegation);
    }
    
    // _allocateTokens allocate tokens to a particular validator, splitting according to commission
    function allocateToken(uint256 _rewards) external onlyOwner {
        uint256 _commission = _rewards.mulTrun(commission.rate);
        uint256 shared = _rewards.sub(_commission);
        inforValidator.accumulatedCommission += _commission;
        currentRewards.reward += shared;
    }
    
    // validator is jailed when the validator operation misbehave
    function jail(uint256 _jailedUntil, bool _tombstoned) external onlyOwner {
        _jail(_jailedUntil, _tombstoned);
    }

    // Unjail is used for unjailing a jailed validator, thus returning
    // them into the bonded validator set, so they can begin receiving provisions
    // and rewards again.
    function unjail() external onlyValidator {
        require(inforValidator.jailed, "validator not jailed");
        // cannot be unjailed if tombstoned
        require(signingInfo.tombstoned == false, "validator jailed");
        uint256 jailedUntil = signingInfo.jailedUntil;

        require(jailedUntil < block.timestamp, "validator jailed");
        Delegation storage del = delegationByAddr[inforValidator.valAddr];
        uint256 tokens = _tokenFromShare(del.shares);
        require(
            tokens > inforValidator.minSelfDelegation,
            "self delegation too low to unjail"
        );

        signingInfo.jailedUntil = 0;
        inforValidator.jailed = false;
    }
    
    // Validator is slashed when the Validator operation misbehave 
    function slash(uint256 _infrationHeight, uint256 _power, uint256 _slashFactor) external onlyOwner {
        _slash(_infrationHeight, _power, _slashFactor);
    }

    function undelegate(uint256 _amount) external {
        _undelegate(msg.sender, _amount);
        _staking.undelegate(_amount);
    }

    function _undelegate(address payable from, uint256 _amount) private {
        require(_checkUndelegateAmount(from, _amount) == true, "Undelegate amount invalid");
        require(ubdEntries[from].length < 7, "too many unbonding delegation entries");
        require(delegations.contains(from), "delegation not found");
        
        _withdrawRewards(from);
        Delegation storage del = delegationByAddr[from];
        uint256 shares = _shareFromToken(_amount);
        require(del.shares >= shares, "not enough delegation shares");
        del.shares -= shares;
        _initializeDelegation(from);
        bool isValidatorOperator = inforValidator.valAddr == from;
        if (
            isValidatorOperator &&
            !inforValidator.jailed &&
            _tokenFromShare(del.shares) < inforValidator.minSelfDelegation
        ) {
            inforValidator.jailed = true; // jail validator
        }

        uint256 amountRemoved = _removeDelShares(shares);

        if (inforValidator.status == Status.Unbonding && inforValidator.unbondingTime < block.timestamp) {
            msg.sender.transfer(amountRemoved);
            if (del.shares == 0) {
                _removeDelegation(msg.sender);
            }
            emit Withdraw(msg.sender, amountRemoved);
        } else {
            inforValidator.ubdEntryCount++;
            uint256 completionTime = block.timestamp.add(UNBONDING_TiME);
            ubdEntries[from].push(
                UBDEntry({
                    completionTime: completionTime,
                    blockHeight: block.number,
                    amount: amountRemoved
                })
            );
            emit Undelegate(from, _amount, completionTime);
        }

        if (inforValidator.status == Status.Bonded && 
            inforValidator.tokens.div(powerReduction) == 0) {
            _staking.removeFromSets();
            _stop();
        }
    }

    function _checkUndelegateAmount(address _delAddr, uint256 _amount) private view returns (bool) {
         Delegation storage del = delegationByAddr[_delAddr];
         if (del.stake.sub(_amount) == 0) {
             return true;
         }
        if (del.stake.sub(_amount) >= params.minStake) {
            return true;
        }
        return false;
    }
    
    // withdraw rewards from a delegation
    function withdrawRewards() external {
        require(delegations.contains(msg.sender), "delegator not found");
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
    function withdraw() external {
        require(delegations.contains(msg.sender), "delegation not found");
        Delegation memory del = delegationByAddr[msg.sender];
        UBDEntry[] storage entries = ubdEntries[msg.sender];
        uint256 amount = 0;
        uint256 entryCount = 0;

        for (uint256 i = 0; i < entries.length; i++) {
            // solhint-disable-next-line not-rely-on-time
            if (entries[i].completionTime < block.timestamp) {
                amount = amount.add(entries[i].amount);
                entries[i] = entries[entries.length - 1];
                entries.pop();
                i--;
                entryCount++;
            }
        }
    
        require(amount > 0, "no unbonding amount to withdraw");
        msg.sender.transfer(amount);

        if (del.shares == 0 && entries.length == 0) {
            _removeDelegation(msg.sender);
        }

        inforValidator.ubdEntryCount = inforValidator.ubdEntryCount.sub(entryCount);
        emit Withdraw(msg.sender, amount);
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
    
    // validate validator signature, must be called once per validator per block
    function validateSignature(
        uint256 _votingPower,
        bool _signed
    ) external onlyOwner returns (bool) {
        // counts blocks the validator should have signed
        uint256 index = signingInfo.indexOffset % params.signedBlockWindow;
        signingInfo.indexOffset++;
        bool previous = missedBlock.items[index];
        bool missed = !_signed;
        if (!previous && missed) { // value has changed from not missed to missed, increment counter
            signingInfo.missedBlockCounter++;
            missedBlock.items[index] = true;
        } else if (previous && !missed) { // value has changed from missed to not missed, decrement counter
            signingInfo.missedBlockCounter--;
            missedBlock.items[index] = false;
        }

        if (missed) {
            emit Liveness(signingInfo.missedBlockCounter, block.number);
        }
        
        uint256 minHeight = signingInfo.startHeight + params.signedBlockWindow;
        uint256 minSignedPerWindow = params.signedBlockWindow.mulTrun(params.minSignedPerWindow);
        uint256 maxMissed = params.signedBlockWindow - minSignedPerWindow;
        
        // if past the minimum height and the validator has missed too many blocks, punish them
        if (block.number > minHeight && signingInfo.missedBlockCounter > maxMissed) {
            if (!inforValidator.jailed) {
                _slash(block.number - 2, _votingPower, params.slashFractionDowntime);
                inforValidator.jailed = true; // jail validator

                signingInfo.jailedUntil = block.timestamp.add(params.downtimeJailDuration);
                signingInfo.missedBlockCounter = 0;
                _resetMissedBlock(signingInfo.indexOffset);
                signingInfo.indexOffset = 0;
                // delete missedBlock;
                
                return true;
            }
        }
        return false;
     }

    function _resetMissedBlock(uint256 _indexOffset) private {
        for (uint i = 0; i < _indexOffset; i++) {
            missedBlock.items[i] = false;
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
        require(_amount >= params.minStake, "Amount must greater than min stake amount");
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
    
    function _addToken(uint256 _amount) private returns(uint256) {
        uint256 issuedShares = 0;
        if (inforValidator.tokens == 0) {
            issuedShares = oneDec;
        } else {
            issuedShares = _shareFromToken(_amount);
        }
        inforValidator.tokens = inforValidator.tokens.add(_amount);
        inforValidator.delegationShares = inforValidator.delegationShares.add(issuedShares);
        return inforValidator.delegationShares;
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
        uint256 previousPeriod = currentRewards.period - 1;
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
        _staking.burn(tokensToBurn);
    }
    
    function _jail(uint256 _jailedUntil, bool _tombstoned) private {
        inforValidator.jailed = true;
        signingInfo.jailedUntil = _jailedUntil;
        signingInfo.tombstoned = _tombstoned;
        _staking.removeFromSets();
        _stop();
    }

   function doubleSign(
        uint256 votingPower,
        uint256 distributionHeight
    ) external onlyOwner {
        _slash(
            distributionHeight.sub(1),
            votingPower,
            params.slashFractionDoubleSign
        );
        // // (Dec 31, 9999 - 23:59:59 GMT).
        _jail(253402300799, true);

        emit Slashed(votingPower, 2);
    }

    // start start validator
    function start() external onlyValidator {
        require(inforValidator.status != Status.Bonded);
        require(!inforValidator.jailed);
        require(inforValidator.tokens.div(powerReduction) > 0);
        _staking.startValidator();
        inforValidator.status = Status.Bonded;
        signingInfo.startHeight = block.number;
    }

    // stop validator
    function stop() external onlyOwner {
        _stop();
    }

    function _stop() private {
        inforValidator.status = Status.Unbonding;
        inforValidator.unbondingHeight = block.number;
        inforValidator.unbondingTime = block.timestamp.add(UNBONDING_TiME);
    }
}