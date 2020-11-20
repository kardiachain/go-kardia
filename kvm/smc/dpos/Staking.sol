pragma solidity 0.6.0;
import {SafeMath} from "./Safemath.sol";
import {IStaking} from "./IStaking.sol";
import "./EnumerableSet.sol";
import {Ownable} from "./Ownable.sol";

contract Staking is IStaking, Ownable {
    using SafeMath for uint256;
    using EnumerableSet for EnumerableSet.AddressSet;

    uint256 oneDec = 1 * 10**18;
    uint256 powerReduction = 1 * 10**9;

    struct Delegation {
        uint256 shares;
        address owner;
    }

    struct UBDEntry {
        uint256 amount;
        uint256 blockHeight;
        uint256 completionTime;
    }

    struct Commission {
        uint256 rate;
        uint256 maxRate;
        uint256 maxChangeRate;
    }

    struct Validator {
        address owner;
        uint256 tokens;
        uint256 delegationShares;
        bool jailed;
        Commission commission;
        uint256 minSelfDelegation;
        uint256 updateTime;
        uint256 ubdEntryCount;
        mapping(uint256 => ValHRewards) hRewards;
        mapping(uint256 => ValSlashEvent) slashEvents;
        uint256 slashEventCounter;
        MissedBlock missedBlock;
    }

    struct DelStartingInfo {
        uint256 stake;
        uint256 previousPeriod;
        uint256 height;
    }

    struct ValSlashEvent {
        uint256 validatorPeriod;
        uint256 fraction;
        uint256 height;
    }

    struct ValCurrentReward {
        uint256 period;
        uint256 reward;
    }

    // validator historical rewards
    struct ValHRewards {
        uint256 cumulativeRewardRatio;
        uint256 reference_count;
    }

    struct ValSigningInfo {
        uint256 startHeight;
        uint256 indexOffset;
        bool tombstoned;
        uint256 missedBlockCounter;
        uint256 jailedUntil;
    }

    struct Params {
        uint256 baseProposerReward;
        uint256 bonusProposerReward;
        uint256 maxValidators;
        uint256 downtimeJailDuration;
        uint256 slashFractionDowntime;
        uint256 unbondingTime;
        uint256 slashFractionDoubleSign;
        uint256 signedBlockWindow;
        uint256 minSignedPerWindow;
        // mint params
        uint256 inflationRateChange;
        uint256 goalBonded;
        uint256 blocksPerYear;
        uint256 inflationMax;
        uint256 inflationMin;
    }

    mapping(address => Validator) valByAddr;
    EnumerableSet.AddressSet vals;
    mapping(address => mapping(address => UBDEntry[])) ubdEntries;
    mapping(address => mapping(address => DelStartingInfo)) delStartingInfo;
    mapping(address => ValCurrentReward) valCurrentRewards;
    mapping(address => ValSigningInfo) valSigningInfos;
    mapping(address => uint256) valAccumulatedCommission;
    mapping(address => mapping(address => Delegation)) delByAddr;
    mapping(address => EnumerableSet.AddressSet) delVals;
    mapping(address => EnumerableSet.AddressSet) dels;

    // sort
    address[] valRanks;
    mapping(address => uint256) valRankIndexes;

    struct MissedBlock {
        mapping(uint256 => bool) items;
    }

    bool _needSort;

    // supply
    uint256 public totalSupply = 5000000000 * 10**18; // 5B
    uint256 public totalBonded;
    uint256 public inflation;
    uint256 public annualProvision;
    uint256 public _feesCollected;
    // mint

    Params _params;
    address _previousProposer;

    constructor() public {
        _params = Params({
            maxValidators: 21,
            downtimeJailDuration: 259200, // 3 days
            baseProposerReward: 1 * 10**16, // 1%
            bonusProposerReward: 4 * 10**16, // 4%
            slashFractionDowntime: 10 * 10**14, // 10%
            unbondingTime: 86400, // 1 day
            slashFractionDoubleSign: 50 * 10**16, // 50%
            signedBlockWindow: 100,
            minSignedPerWindow: 5 * 10**16,
            goalBonded: 10 * 10**16, // 10%
            blocksPerYear: 6307200, // assumption 5s per block
            inflationMax: 7 * 10**16, // 20%
            inflationMin: 189216 * 10**11, // 1,89216%
            inflationRateChange: 2 * 10**16 // 4%
        });
    }

    // @notice Will receive any eth sent to the contract
    function deposit() external payable {}

    function setParams(
        uint256 maxValidators,
        uint256 downtimeJailDuration,
        uint256 baseProposerReward,
        uint256 bonusProposerReward,
        uint256 slashFractionDowntime,
        uint256 unbondingTime,
        uint256 slashFractionDoubleSign,
        uint256 signedBlockWindow,
        uint256 minSignedPerWindow
    ) public onlyOwner {
        if (maxValidators > 0) {
            _params.maxValidators = maxValidators;
        }
        if (downtimeJailDuration > 0) {
            _params.downtimeJailDuration = downtimeJailDuration;
        }
        if (baseProposerReward > 0) {
            _params.baseProposerReward = baseProposerReward;
        }
        if (bonusProposerReward > 0) {
            _params.bonusProposerReward = bonusProposerReward;
        }
        if (slashFractionDowntime > 0) {
            _params.slashFractionDowntime = slashFractionDowntime;
        }
        if (unbondingTime > 0) {
            _params.unbondingTime = unbondingTime;
        }
        if (slashFractionDoubleSign > 0) {
            _params.slashFractionDoubleSign = slashFractionDoubleSign;
        }

        if (signedBlockWindow > 0) {
            _params.signedBlockWindow = signedBlockWindow;
        }

        if (minSignedPerWindow > 0) {
            _params.minSignedPerWindow = minSignedPerWindow;
        }
    }

    function setTotalBonded(uint256 amount) public onlyOwner {
        totalBonded = amount;
    }

    function setMintParams(
        uint256 inflationRateChange,
        uint256 goalBonded,
        uint256 blocksPerYear,
        uint256 inflationMax,
        uint256 inflationMin
    ) public onlyOwner {
        if (inflationRateChange > 0) {
            _params.inflationRateChange = inflationRateChange;
        }
        if (goalBonded > 0) {
            _params.goalBonded = goalBonded;
        }
        if (blocksPerYear > 0) {
            _params.blocksPerYear = blocksPerYear;
        }
        if (inflationMax > 0) {
            _params.inflationMax = inflationMax;
        }
        if (inflationMin > 0) {
            _params.inflationMin = inflationMin;
        }
    }

    function createValidator(
        uint256 commssionRate,
        uint256 maxRate,
        uint256 maxChangeRate,
        uint256 minSeftDelegation
    ) public payable {
        _createValidator(
            msg.sender,
            msg.value,
            commssionRate,
            maxRate,
            maxChangeRate,
            minSeftDelegation
        );

        emit CreateValidator(
            msg.sender,
            msg.value,
            commssionRate,
            maxRate,
            maxChangeRate,
            minSeftDelegation
        );
    }

    function _createValidator(
        address payable valAddr,
        uint256 amount,
        uint256 rate,
        uint256 maxRate,
        uint256 maxChangeRate,
        uint256 minSelfDelegation
    ) private {
        require(!vals.contains(valAddr), "validator already exist");
        require(amount > 0, "invalid delegation amount");
        require(amount > minSelfDelegation, "self delegation below minimum");
        require(
            maxRate <= oneDec,
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

        Commission memory commission = Commission({
            rate: rate,
            maxRate: maxRate,
            maxChangeRate: maxChangeRate
        });

        vals.add(valAddr);
        // solhint-disable-next-line not-rely-on-time
        uint256 updateTime = block.timestamp;
        valByAddr[valAddr].commission = commission;
        valByAddr[valAddr].minSelfDelegation = minSelfDelegation;
        valByAddr[valAddr].updateTime = updateTime;
        valByAddr[valAddr].owner = valAddr;
        _afterValidatorCreated(valAddr);
        _delegate(valAddr, valAddr, amount);
        valSigningInfos[valAddr].startHeight = block.number;
    }

    function updateValidator(uint256 commissionRate, uint256 minSelfDelegation)
        public
    {
        _updateValidator(msg.sender, commissionRate, minSelfDelegation);
    }

    function _updateValidator(
        address valAddr,
        uint256 commissionRate,
        uint256 minSelfDelegation
    ) private {
        require(vals.contains(valAddr), "validator not found");
        Validator storage val = valByAddr[valAddr];
        if (commissionRate > 0) {
            require(
                // solhint-disable-next-line not-rely-on-time
                block.timestamp.sub(val.updateTime) >= 86400,
                "commission cannot be changed more than one in 24h"
            );
            require(
                commissionRate <= val.commission.maxRate,
                "commission cannot be more than the max rate"
            );
            require(
                commissionRate.sub(val.commission.rate) <=
                    val.commission.maxChangeRate,
                "commission cannot be changed more than max change rate"
            );
        }
        if (minSelfDelegation > 0) {
            require(
                minSelfDelegation > val.minSelfDelegation,
                "minimum self delegation cannot be decrease"
            );
            require(
                minSelfDelegation <= val.tokens,
                "self delegation below minimum"
            );
            val.minSelfDelegation = minSelfDelegation;
        }

        if (commissionRate > 0) {
            val.commission.rate = commissionRate;
            // solhint-disable-next-line not-rely-on-time
            val.updateTime = block.timestamp;
        }

        emit UpdateValidator(msg.sender, commissionRate, minSelfDelegation);
    }

    function _afterValidatorCreated(address valAddr) private {
        _initializeValidator(valAddr);
    }

    function _afterDelegationModified(address valAddr, address delAddr)
        private
    {
        _initializeDelegation(valAddr, delAddr);
    }

    function _delegate(
        address payable delAddr,
        address valAddr,
        uint256 amount
    ) private {
        // add delegation if not exists;
        if (!dels[valAddr].contains(delAddr)) {
            dels[valAddr].add(delAddr);
            delVals[delAddr].add(valAddr);
            delByAddr[valAddr][delAddr].owner = delAddr;
            _beforeDelegationCreated(valAddr);
        } else {
            _beforeDelegationSharesModified(valAddr, delAddr);
        }

        uint256 shared = _addTokenFromDel(valAddr, amount);

        totalBonded = totalBonded.add(amount);

        // increment stake amount
        Delegation storage del = delByAddr[valAddr][delAddr];
        del.shares = del.shares.add(shared);
        _afterDelegationModified(valAddr, delAddr);
        addValidatorRank(valAddr);
        emit Delegate(valAddr, delAddr, amount);
    }

    function _addTokenFromDel(address valAddr, uint256 amount)
        private
        returns (uint256)
    {
        Validator storage val = valByAddr[valAddr];
        uint256 issuedShares = 0;
        if (val.tokens == 0) {
            issuedShares = oneDec;
        } else {
            issuedShares = _shareFromToken(valAddr, amount);
        }
        val.tokens = val.tokens.add(amount);
        val.delegationShares = val.delegationShares.add(issuedShares);
        return issuedShares;
    }

    function delegate(address valAddr) public payable {
        require(vals.contains(valAddr), "validator not found");
        require(msg.value > 0, "invalid delegation amount");
        _delegate(msg.sender, valAddr, msg.value);
    }

    function _undelegate(
        address valAddr,
        address payable delAddr,
        uint256 amount
    ) private {
        require(
            ubdEntries[valAddr][delAddr].length < 7,
            "too many unbonding delegation entries"
        );
        require(dels[valAddr].contains(delAddr), "delegation not found");
        _beforeDelegationSharesModified(valAddr, delAddr);

        Validator storage val = valByAddr[valAddr];
        Delegation storage del = delByAddr[valAddr][delAddr];
        uint256 shares = _shareFromToken(valAddr, amount);
        require(del.shares >= shares, "not enough delegation shares");
        del.shares -= shares;
        _afterDelegationModified(valAddr, delAddr);
        bool isValidatorOperator = valAddr == delAddr;
        if (
            isValidatorOperator &&
            !val.jailed &&
            _tokenFromShare(valAddr, del.shares) < val.minSelfDelegation
        ) {
            _jail(valAddr);
        }

        uint256 amountRemoved = _removeDelShares(valAddr, shares);
        val.ubdEntryCount++;
        if (val.tokens.div(powerReduction) == 0) {
            removeValidatorRank(valAddr);
        } else {
            addValidatorRank(valAddr);
        }

        // solhint-disable-next-line not-rely-on-time
        uint256 completionTime = block.timestamp.add(_params.unbondingTime);
        ubdEntries[valAddr][delAddr].push(
            UBDEntry({
                completionTime: completionTime,
                blockHeight: block.number,
                amount: amountRemoved
            })
        );

        emit Undelegate(valAddr, msg.sender, amount, completionTime);
    }

    function _removeDelShares(address valAddr, uint256 shares)
        private
        returns (uint256)
    {
        Validator storage val = valByAddr[valAddr];
        uint256 remainingShares = val.delegationShares;
        uint256 issuedTokens = 0;
        remainingShares = remainingShares.sub(shares);
        if (remainingShares == 0) {
            issuedTokens = val.tokens;
            val.tokens = 0;
        } else {
            issuedTokens = _tokenFromShare(valAddr, shares);
            val.tokens = val.tokens.sub(issuedTokens);
        }
        val.delegationShares = remainingShares;
        return issuedTokens;
    }

    function undelegate(address valAddr, uint256 amount) public {
        require(amount > 0, "invalid undelegate amount");
        _undelegate(valAddr, msg.sender, amount);
    }

    function _jail(address valAddr) private {
        valByAddr[valAddr].jailed = true;
        removeValidatorRank(valAddr);
    }

    function _slash(
        address valAddr,
        uint256 infrationHeight,
        uint256 power,
        uint256 slashFactor
    ) private {
        require(
            infrationHeight <= block.number,
            "cannot slash infrations in the future"
        );
        Validator storage val = valByAddr[valAddr];
        uint256 slashAmount = power.mul(powerReduction).mulTrun(slashFactor);
        if (infrationHeight < block.number) {
            uint256 totalDel = dels[valAddr].length();
            for (uint256 i = 0; i < totalDel; i++) {
                address delAddr = dels[valAddr].at(i);
                UBDEntry[] storage entries = ubdEntries[valAddr][delAddr];
                for (uint256 j = 0; j < entries.length; j++) {
                    UBDEntry storage entry = entries[j];
                    if (entry.amount == 0) continue;
                    // if unbonding started before this height, stake did not contribute to infraction;
                    if (entry.blockHeight < infrationHeight) continue;
                    // solhint-disable-next-line not-rely-on-time
                    if (entry.completionTime < block.timestamp) {
                        // unbonding delegation no longer eligible for slashing, skip it
                        continue;
                    }
                    uint256 amountSlashed = entry.amount.mulTrun(slashFactor);
                    entry.amount = entry.amount.sub(amountSlashed);
                    slashAmount = slashAmount.sub(amountSlashed);
                }
            }
        }

        uint256 tokensToBurn = slashAmount;
        if (tokensToBurn > val.tokens) {
            tokensToBurn = val.tokens;
        }

        if (val.tokens > 0) {
            uint256 effectiveFraction = tokensToBurn.divTrun(val.tokens);
            _beforeValidatorSlashed(valAddr, effectiveFraction);
        }

        val.tokens = val.tokens.sub(tokensToBurn);
        _burn(tokensToBurn);
        removeValidatorRank(valAddr);
    }

    function _burn(uint256 amount) private {
        totalBonded -= amount;
        totalSupply -= amount;
        emit Burn(amount);
    }

    function _updateValidatorSlashFraction(address valAddr, uint256 fraction)
        private
    {
        uint256 newPeriod = _incrementValidatorPeriod(valAddr);
        _incrementReferenceCount(valAddr, newPeriod);
        valByAddr[valAddr].slashEvents[valByAddr[valAddr]
            .slashEventCounter] = ValSlashEvent({
            validatorPeriod: newPeriod,
            fraction: fraction,
            height: block.number
        });
        valByAddr[valAddr].slashEventCounter++;
    }

    function _beforeValidatorSlashed(address valAddr, uint256 fraction)
        private
    {
        _updateValidatorSlashFraction(valAddr, fraction);
    }

    function getTotalSupply() public view returns (uint256) {
        return totalSupply;
    }

    function _withdraw(address valAddr, address payable delAddr) private {
        require(dels[valAddr].contains(delAddr), "delegation not found");
        Delegation memory del = delByAddr[valAddr][delAddr];
        Validator storage val = valByAddr[valAddr];
        UBDEntry[] storage entries = ubdEntries[valAddr][delAddr];
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
        delAddr.transfer(amount);
        totalBonded = totalBonded.sub(amount);

        if (del.shares == 0 && entries.length == 0) {
            _removeDelegation(valAddr, delAddr);
        }

        val.ubdEntryCount = val.ubdEntryCount.sub(entryCount);
        if (val.delegationShares == 0 && val.ubdEntryCount == 0) {
            _removeValidator(valAddr);
        }

        emit Withdraw(valAddr, delAddr, amount);
    }

    function _removeDelegation(address valAddr, address delAddr) private {
        dels[valAddr].remove(delAddr);
        delete delByAddr[valAddr][delAddr];
        delete delStartingInfo[valAddr][delAddr];
        delVals[delAddr].remove(valAddr);
    }

    function _removeValidator(address valAddr) private {
        // remove validator
        vals.remove(valAddr);
        uint256 commission = valAccumulatedCommission[valAddr];
        if (commission > 0) {
            // substract total supply
            totalSupply = totalSupply.sub(commission);
        }

        // remove other index
        delete valAccumulatedCommission[valAddr];
        delete valCurrentRewards[valAddr];
        delete valSigningInfos[valAddr];

        delete valByAddr[valAddr];
        removeValidatorRank(valAddr);
    }

    function withdraw(address valAddr) public {
        _withdraw(valAddr, msg.sender);
    }

    function _calculateDelegationRewards(
        address valAddr,
        address delAddr,
        uint256 endingPeriod
    ) private view returns (uint256) {
        DelStartingInfo memory startingInfo = delStartingInfo[valAddr][delAddr];
        uint256 rewards = 0;
        uint256 slashEventCounter = valByAddr[valAddr].slashEventCounter;
        for (uint256 i = 0; i < slashEventCounter; i++) {
            ValSlashEvent memory slashEvent = valByAddr[valAddr].slashEvents[i];
            if (
                slashEvent.height > startingInfo.height &&
                slashEvent.height < block.number
            ) {
                uint256 _endingPeriod = slashEvent.validatorPeriod;
                if (_endingPeriod > startingInfo.previousPeriod) {
                    rewards += _calculateDelegationRewardsBetween(
                        valAddr,
                        startingInfo.previousPeriod,
                        slashEvent.validatorPeriod,
                        startingInfo.stake
                    );
                    startingInfo.stake = startingInfo.stake.mulTrun(
                        oneDec.sub(slashEvent.fraction)
                    );
                    startingInfo.previousPeriod = _endingPeriod;
                }
            }
        }
        rewards += _calculateDelegationRewardsBetween(
            valAddr,
            startingInfo.previousPeriod,
            endingPeriod,
            startingInfo.stake
        );
        return rewards;
    }

    function _calculateDelegationRewardsBetween(
        address valAddr,
        uint256 startingPeriod,
        uint256 endingPeriod,
        uint256 stake
    ) private view returns (uint256) {
        ValHRewards memory starting = valByAddr[valAddr]
            .hRewards[startingPeriod];
        ValHRewards memory ending = valByAddr[valAddr].hRewards[endingPeriod];
        uint256 difference = ending.cumulativeRewardRatio.sub(
            starting.cumulativeRewardRatio
        );
        return stake.mulTrun(difference);
    }

    function _incrementValidatorPeriod(address valAddr)
        private
        returns (uint256)
    {
        Validator memory val = valByAddr[valAddr];

        ValCurrentReward storage rewards = valCurrentRewards[valAddr];
        uint256 previousPeriod = rewards.period.sub(1);
        uint256 current = 0;
        if (rewards.reward > 0) {
            current = rewards.reward.divTrun(val.tokens);
        }
        uint256 historical = valByAddr[valAddr].hRewards[previousPeriod]
            .cumulativeRewardRatio;
        _decrementReferenceCount(valAddr, previousPeriod);

        valByAddr[valAddr].hRewards[rewards.period]
            .cumulativeRewardRatio = historical.add(current);
        valByAddr[valAddr].hRewards[rewards.period].reference_count = 1;
        rewards.period++;
        rewards.reward = 0;
        return previousPeriod.add(1);
    }

    function _decrementReferenceCount(address valAddr, uint256 period) private {
        valByAddr[valAddr].hRewards[period].reference_count--;
        if (valByAddr[valAddr].hRewards[period].reference_count == 0) {
            delete valByAddr[valAddr].hRewards[period];
        }
    }

    function _incrementReferenceCount(address valAddr, uint256 period) private {
        valByAddr[valAddr].hRewards[period].reference_count++;
    }

    function _initializeDelegation(address valAddr, address delAddr) private {
        Delegation storage del = delByAddr[valAddr][delAddr];
        uint256 previousPeriod = valCurrentRewards[valAddr].period - 1;
        _incrementReferenceCount(valAddr, previousPeriod);
        delStartingInfo[valAddr][delAddr].height = block.number;
        delStartingInfo[valAddr][delAddr].previousPeriod = previousPeriod;
        uint256 stake = _tokenFromShare(valAddr, del.shares);
        delStartingInfo[valAddr][delAddr].stake = stake;
    }

    function _initializeValidator(address valAddr) private {
        valCurrentRewards[valAddr].period = 1;
        valCurrentRewards[valAddr].reward = 0;
        valAccumulatedCommission[valAddr] = 0;
    }

    function _beforeDelegationCreated(address valAddr) private {
        _incrementValidatorPeriod(valAddr);
    }

    function _beforeDelegationSharesModified(
        address valAddr,
        address payable delAddr
    ) private {
        _withdrawRewards(valAddr, delAddr);
    }

    function _withdrawRewards(address valAddr, address payable delAddr)
        private
    {
        uint256 endingPeriod = _incrementValidatorPeriod(valAddr);
        uint256 rewards = _calculateDelegationRewards(
            valAddr,
            delAddr,
            endingPeriod
        );
        _decrementReferenceCount(
            valAddr,
            delStartingInfo[valAddr][delAddr].previousPeriod
        );
        delete delStartingInfo[valAddr][delAddr];
        if (rewards > 0) {
            delAddr.transfer(rewards);
            emit WithdrawDelegationRewards(valAddr, delAddr, rewards);
        }
    }

    function withdrawReward(address valAddr) public {
        require(dels[valAddr].contains(msg.sender), "delegator not found");
        _withdrawRewards(valAddr, msg.sender);
        _initializeDelegation(valAddr, msg.sender);
    }

    function getDelegationRewards(address valAddr, address delAddr)
        public
        view
        returns (uint256)
    {
        require(dels[valAddr].contains(delAddr), "delegation not found");
        Validator memory val = valByAddr[valAddr];
        Delegation memory del = delByAddr[valAddr][delAddr];
        uint256 rewards = _calculateDelegationRewards(
            valAddr,
            delAddr,
            valCurrentRewards[valAddr].period - 1
        );

        uint256 currentReward = valCurrentRewards[valAddr].reward;
        if (currentReward > 0) {
            uint256 stake = _tokenFromShare(valAddr, del.shares);
            rewards += stake.mulTrun(currentReward.divTrun(val.tokens));
        }
        return rewards;
    }

    function _withdrawValidatorCommission(address payable valAddr) private {
        require(vals.contains(valAddr), "validator not found");
        uint256 commission = valAccumulatedCommission[valAddr];
        require(commission > 0, "no validator commission to reward");
        valAddr.transfer(commission);
        valAccumulatedCommission[valAddr] = 0;
        emit WithdrawCommissionReward(valAddr, commission);
    }

    function withdrawValidatorCommission() public {
        _withdrawValidatorCommission(msg.sender);
    }

    function getValidator(address valAddr)
        public
        view
        returns (
            uint256,
            uint256,
            bool,
            uint256,
            uint256,
            uint256
        )
    {
        require(vals.contains(valAddr), "validator not found");
        Validator memory val = valByAddr[valAddr];
        Commission memory commission = val.commission;

        return (
            val.tokens,
            val.delegationShares,
            val.jailed,
            commission.rate,
            commission.maxRate,
            commission.maxChangeRate
        );
    }

    function getDelegationsByValidator(address valAddr)
        public
        view
        returns (address[] memory, uint256[] memory)
    {
        require(vals.contains(valAddr), "validator not found");
        uint256 total = dels[valAddr].length();
        address[] memory delAddrs = new address[](total);
        uint256[] memory shares = new uint256[](total);
        for (uint256 i = 0; i < total; i++) {
            address delAddr = dels[valAddr].at(i);
            delAddrs[i] = delAddr;
            shares[i] = delByAddr[valAddr][delAddr].shares;
        }
        return (delAddrs, shares);
    }

    function getDelegation(address valAddr, address delAddr)
        public
        view
        returns (uint256)
    {
        require(dels[valAddr].contains(delAddr), "delegation not found");
        Delegation memory del = delByAddr[valAddr][delAddr];
        return (del.shares);
    }

    function getValidatorsByDelegator(address delAddr)
        public
        view
        returns (address[] memory)
    {
        uint256 total = delVals[delAddr].length();
        address[] memory addrs = new address[](total);
        for (uint256 i = 0; i < total; i++) {
            addrs[i] = delVals[delAddr].at(i);
        }

        return addrs;
    }

    function getValidatorCommission(address valAddr)
        public
        view
        returns (uint256)
    {
        return valAccumulatedCommission[valAddr];
    }

    function getAllDelegatorRewards(address delAddr)
        public
        view
        returns (uint256)
    {
        uint256 total = delVals[delAddr].length();
        uint256 rewards = 0;
        for (uint256 i = 0; i < total; i++) {
            address valAddr = delVals[delAddr].at(i);
            rewards += getDelegationRewards(valAddr, delAddr);
        }
        return rewards;
    }

    function getDelegatorStake(address valAddr, address delAddr)
        public
        view
        returns (uint256)
    {
        require(dels[valAddr].contains(delAddr), "delegation not found");
        Delegation memory del = delByAddr[valAddr][delAddr];
        return _tokenFromShare(valAddr, del.shares);
    }

    function getAllDelegatorStake(address delAddr)
        public
        view
        returns (uint256)
    {
        uint256 stake = 0;
        uint256 total = delVals[delAddr].length();
        for (uint256 i = 0; i < total; i++) {
            address valAddr = delVals[delAddr].at(i);
            stake += getDelegatorStake(valAddr, delAddr);
        }
        return stake;
    }

    function _tokenFromShare(address valAddr, uint256 shares)
        private
        view
        returns (uint256)
    {
        Validator memory val = valByAddr[valAddr];
        return shares.mul(val.tokens).div(val.delegationShares);
    }

    function _shareFromToken(address valAddr, uint256 amount)
        private
        view
        returns (uint256)
    {
        Validator memory val = valByAddr[valAddr];
        return val.delegationShares.mul(amount).div(val.tokens);
    }

    function getUBDEntries(address valAddr, address delAddr)
        public
        view
        returns (uint256[] memory, uint256[] memory)
    {
        uint256 total = ubdEntries[valAddr][delAddr].length;
        uint256[] memory balances = new uint256[](total);
        uint256[] memory completionTime = new uint256[](total);

        for (uint256 i = 0; i < total; i++) {
            completionTime[i] = ubdEntries[valAddr][delAddr][i].completionTime;
            balances[i] = ubdEntries[valAddr][delAddr][i].amount;
        }
        return (balances, completionTime);
    }

    function getValidatorSlashEvents(address valAddr)
        public
        view
        returns (uint256[] memory, uint256[] memory)
    {
        uint256 total = valByAddr[valAddr].slashEventCounter;
        uint256[] memory fraction = new uint256[](total);
        uint256[] memory height = new uint256[](total);
        for (uint256 i = 0; i < total; i++) {
            fraction[i] = valByAddr[valAddr].slashEvents[i].fraction;
            height[i] = valByAddr[valAddr].slashEvents[i].height;
        }
        return (height, fraction);
    }

    function _doubleSign(
        address valAddr,
        uint256 votingPower,
        uint256 distributionHeight
    ) private {
        if (!vals.contains(valAddr)) return;

        // reason: doubleSign
        emit Slashed(valAddr, votingPower, 2);

        _slash(
            valAddr,
            distributionHeight.sub(1),
            votingPower,
            _params.slashFractionDoubleSign
        );
        _jail(valAddr);
        // // (Dec 31, 9999 - 23:59:59 GMT).
        valSigningInfos[valAddr].jailedUntil = 253402300799;
        valSigningInfos[valAddr].tombstoned = true;
    }

    function doubleSign(
        address valAddr,
        uint256 votingPower,
        uint256 distributionHeight
    ) public {
        _doubleSign(valAddr, votingPower, distributionHeight);
    }

    function _validateSignature(
        address valAddr,
        uint256 votingPower,
        bool signed
    ) private {
        Validator storage val = valByAddr[valAddr];
        ValSigningInfo storage signInfo = valSigningInfos[valAddr];
        uint256 index = signInfo.indexOffset % _params.signedBlockWindow;
        signInfo.indexOffset++;
        bool previous = valByAddr[valAddr].missedBlock.items[index];
        bool missed = !signed;
        if (!previous && missed) {
            signInfo.missedBlockCounter++;
            valByAddr[valAddr].missedBlock.items[index] = true;
        } else if (previous && !missed) {
            signInfo.missedBlockCounter--;
            valByAddr[valAddr].missedBlock.items[index] = false;
        }

        if (missed) {
            emit Liveness(valAddr, signInfo.missedBlockCounter, block.number);
        }

        uint256 minHeight = signInfo.startHeight + _params.signedBlockWindow;

        uint256 minSignedPerWindow = _params.signedBlockWindow.mulTrun(
            _params.minSignedPerWindow
        );
        uint256 maxMissed = _params.signedBlockWindow - minSignedPerWindow;
        if (
            block.number > minHeight && signInfo.missedBlockCounter > maxMissed
        ) {
            if (!val.jailed) {
                // reason: missing signature
                emit Slashed(valAddr, votingPower, 1);

                _slash(
                    valAddr,
                    block.number - 2,
                    votingPower,
                    _params.slashFractionDowntime
                );
                _jail(valAddr);

                // solhint-disable-next-line not-rely-on-time
                signInfo.jailedUntil = block.timestamp.add(
                    _params.downtimeJailDuration
                );
                signInfo.missedBlockCounter = 0;
                signInfo.indexOffset = 0;
                delete valByAddr[valAddr].missedBlock;
            }
        }
    }

    function _allocateTokens(
        uint256 sumPreviousPrecommitPower,
        uint256 totalPreviousVotingPower,
        address previousProposer,
        address[] memory addrs,
        uint256[] memory powers
    ) private {
        uint256 previousFractionVotes = sumPreviousPrecommitPower.divTrun(
            totalPreviousVotingPower
        );
        uint256 proposerMultiplier = _params.baseProposerReward.add(
            _params.bonusProposerReward.mulTrun(previousFractionVotes)
        );
        uint256 proposerReward = _feesCollected.mulTrun(proposerMultiplier);
        _allocateTokensToValidator(previousProposer, proposerReward);

        uint256 voteMultiplier = oneDec;
        voteMultiplier = voteMultiplier.sub(proposerMultiplier);
        for (uint256 i = 0; i < addrs.length; i++) {
            uint256 powerFraction = powers[i].divTrun(totalPreviousVotingPower);
            uint256 rewards = _feesCollected.mulTrun(voteMultiplier).mulTrun(
                powerFraction
            );
            _allocateTokensToValidator(addrs[i], rewards);
        }
    }

    function _allocateTokensToValidator(address valAddr, uint256 rewards)
        private
    {
        uint256 commission = rewards.mulTrun(
            valByAddr[valAddr].commission.rate
        );
        uint256 shared = rewards.sub(commission);
        valAccumulatedCommission[valAddr] += commission;
        valCurrentRewards[valAddr].reward += shared;
    }

    function _finalizeCommit(
        address[] memory addrs,
        uint256[] memory powers,
        bool[] memory signed
    ) private {
        uint256 previousTotalPower = 0;
        uint256 sumPreviousPrecommitPower = 0;
        for (uint256 i = 0; i < powers.length; i++) {
            previousTotalPower += powers[i];
            if (signed[i]) {
                sumPreviousPrecommitPower += powers[i];
            }
        }
        if (block.number > 1) {
            _allocateTokens(
                sumPreviousPrecommitPower,
                previousTotalPower,
                _previousProposer,
                addrs,
                powers
            );
        }
        _previousProposer = block.coinbase;

        for (uint256 i = 0; i < powers.length; i++) {
            _validateSignature(addrs[i], powers[i], signed[i]);
        }
    }

    function finalizeCommit(
        address[] memory addrs,
        uint256[] memory powers,
        bool[] memory signed
    ) public onlyOwner {
        _finalizeCommit(addrs, powers, signed);
    }

    function setPreviousProposer(address previousProposer) public onlyOwner {
        _previousProposer = previousProposer;
    }

    function getValidators()
        public
        view
        returns (
            address[] memory,
            uint256[] memory,
            uint256[] memory
        )
    {
        uint256 total = vals.length();
        address[] memory valAddrs = new address[](total);
        uint256[] memory tokens = new uint256[](total);
        uint256[] memory delegationsShares = new uint256[](total);
        for (uint256 i = 0; i < total; i++) {
            address valAddr = vals.at(i);
            valAddrs[i] = valAddr;
            tokens[i] = valByAddr[valAddr].tokens;
            delegationsShares[i] = valByAddr[valAddr].delegationShares;
        }
        return (valAddrs, tokens, delegationsShares);
    }

    function getMissedBlock(address valAddr)
        public
        view
        returns (bool[] memory)
    {
        bool[] memory missedBlock = new bool[](_params.signedBlockWindow);
        for (uint256 i = 0; i < _params.signedBlockWindow; i++) {
            missedBlock[i] = valByAddr[valAddr].missedBlock.items[i];
        }

        return missedBlock;
    }

    // Mint
    //  --------------------------------------------------

    // Mints new tokens for the previous block. Returns fee collected
    function mint() public onlyOwner returns (uint256) {
        // recalculate inflation rate
        inflation = nextInflationRate();
        // recalculate annual provisions
        annualProvision = nextAnnualProvisions();
        // update fee collected
        _feesCollected = getBlockProvision();
        totalSupply += _feesCollected;
        emit Minted(_feesCollected);
        return _feesCollected;
    }

    function setInflation(uint256 _inflation) public onlyOwner {
        inflation = _inflation;
    }

    function nextInflationRate() public view returns (uint256) {
        uint256 bondedRatio = totalBonded.divTrun(totalSupply);
        uint256 inflationRateChangePerYear;
        uint256 inflationRateChange;
        uint256 inflationRate;
        if (bondedRatio < _params.goalBonded) {
            inflationRateChangePerYear = oneDec
                .sub(bondedRatio.divTrun(_params.goalBonded))
                .mulTrun(_params.inflationRateChange);
            inflationRateChange = inflationRateChangePerYear.div(
                _params.blocksPerYear
            );
            inflationRate = inflation.add(inflationRateChange);
        } else {
            inflationRateChangePerYear = bondedRatio
                .divTrun(_params.goalBonded)
                .sub(oneDec)
                .mulTrun(_params.inflationRateChange);
            inflationRateChange = inflationRateChangePerYear.div(
                _params.blocksPerYear
            );
            if (inflation > inflationRateChange) {
                inflationRate = inflation.sub(inflationRateChange);
            } else {
                inflationRate = 0;
            }
        }
        if (inflationRate > _params.inflationMax) {
            inflationRate = _params.inflationMax;
        }
        if (inflationRate < _params.inflationMin) {
            inflationRate = _params.inflationMin;
        }
        return inflationRate;
    }

    function nextAnnualProvisions() public view returns (uint256) {
        return inflation.mulTrun(totalSupply);
    }

    function setAnnualProvision(uint256 _annualProvision) public onlyOwner {
        annualProvision = _annualProvision;
    }

    function getBlockProvision() public view returns (uint256) {
        return annualProvision.div(_params.blocksPerYear);
    }

    function setTotalSupply(uint256 amount) public onlyOwner {
        totalSupply = amount;
    }

    function getInflation() public view returns (uint256) {
        return inflation;
    }

    // validator rank
    function addValidatorRank(address valAddr) private {
        uint256 idx = valRankIndexes[valAddr];
        uint256 valPower = getValidatorPower(valAddr);
        if (valPower == 0) return;
        if (idx == 0) {
            valRanks.push(valAddr);
            valRankIndexes[valAddr] = valRanks.length;
        }
        _needSort = true;
    }

    function removeValidatorRank(address valAddr) private {
        uint256 todDeleteIndex = valRankIndexes[valAddr];
        if (todDeleteIndex == 0) return;
        uint256 lastIndex = valRanks.length;
        address last = valRanks[lastIndex - 1];
        valRanks[todDeleteIndex - 1] = last;
        valRankIndexes[last] = todDeleteIndex;
        valRanks.pop();
        delete valRankIndexes[valAddr];
        _needSort = true;
    }

    function getValPowerByRank(uint256 rank) private view returns (uint256) {
        return getValidatorPower(valRanks[rank]);
    }

    function _sortValRank(int256 left, int256 right) internal {
        int256 i = left;
        int256 j = right;
        if (i == j) return;
        uint256 pivot = getValPowerByRank(uint256(left + (right - left) / 2));
        while (i <= j) {
            while (getValPowerByRank(uint256(i)) > pivot) i++;
            while (pivot > getValPowerByRank(uint256(j))) j--;
            if (i <= j) {
                address tmp = valRanks[uint256(i)];
                valRanks[uint256(i)] = valRanks[uint256(j)];
                valRanks[uint256(j)] = tmp;

                valRankIndexes[tmp] = uint256(j + 1);
                valRankIndexes[valRanks[uint256(i)]] = uint256(i + 1);

                i++;
                j--;
            }
        }
        if (left < j) _sortValRank(left, j);
        if (i < right) _sortValRank(i, right);
    }

    function _clearValRank() private {
        for (uint256 i = valRanks.length; i > 300; i--) {
            delete valRankIndexes[valRanks[i - 1]];
            valRanks.pop();
        }
    }

    function applyAndReturnValidatorSets()
        public
        onlyOwner
        returns (address[] memory, uint256[] memory)
    {
        if (_needSort && valRanks.length > 0) {
            _sortValRank(0, int256(valRanks.length - 1));
            _clearValRank();
            _needSort = false;
        }
        return getValidatorSets();
    }

    function getValidatorSets()
        public
        view
        returns (address[] memory, uint256[] memory)
    {
        uint256 maxVal = _params.maxValidators;
        if (maxVal > valRanks.length) {
            maxVal = valRanks.length;
        }
        address[] memory valAddrs = new address[](maxVal);
        uint256[] memory powers = new uint256[](maxVal);

        for (uint256 i = 0; i < maxVal; i++) {
            valAddrs[i] = valRanks[i];
            powers[i] = getValidatorPower(valRanks[i]);
        }
        return (valAddrs, powers);
    }

    function getValidatorPower(address valAddr) public view returns (uint256) {
        return valByAddr[valAddr].tokens.div(powerReduction);
    }

    // slashing
    function _unjail(address valAddr) private {
        require(vals.contains(valAddr), "validator not found");
        Validator storage val = valByAddr[valAddr];
        require(val.jailed, "validator not jailed");
        // cannot be unjailed if tombstoned
        require(
            valSigningInfos[valAddr].tombstoned == false,
            "validator jailed"
        );
        uint256 jailedUntil = valSigningInfos[valAddr].jailedUntil;
        // solhint-disable-next-line not-rely-on-time
        require(jailedUntil < block.timestamp, "validator jailed");
        Delegation storage del = delByAddr[valAddr][valAddr];
        uint256 tokens = _tokenFromShare(valAddr, del.shares);
        require(
            tokens > val.minSelfDelegation,
            "self delegation too low to unjail"
        );

        valSigningInfos[valAddr].jailedUntil = 0;
        val.jailed = false;
        addValidatorRank(valAddr);
    }

    function unjail() public {
        _unjail(msg.sender);
        emit UnJail(msg.sender);
    }
}
