pragma solidity ^0.5.0;

interface IParams {
    function updateMaxValidator(uint256 _maxValidators) external;
    function updateBaseReward(uint256 _baseProposerReward, uint256 _bonusProposerReward) external;
    function updateValidatorParams(uint256 _downtimeJailDuration,
        uint256 _slashFractionDowntime,
        uint256 _unbondingTime,
        uint256 _slashFractionDoubleSign,
        uint256 _signedBlockWindow,
        uint256 _minSignedPerWindow,
        uint256 _minStake,
        uint256 _minValidatorBalance,
        uint256 _minAmountChangeName,
        uint256 _minSelfDelegation) external;
    function updateMintParams(uint256 _inflationRateChange,
        uint256 _goalBonded,
        uint256 _blocksPerYear,
        uint256 _inflationMax,
        uint256 _inflationMin) external;
    function getBaseProposerReward() external view returns (uint256);
    function getBonusProposerReward() external view returns (uint256);
    function getMaxProposers() external view returns (uint256);
    function getInflationRateChange() external view returns (uint256);
    function getGoalBonded() external view returns (uint256);
    function getBlocksPerYear() external view returns (uint256);
    function getInflationMax() external view returns (uint256);
    function getInflationMin() external view returns (uint256);
    function getDowntimeJailDuration() external view returns (uint256);
    function getSlashFractionDowntime() external view returns (uint256);
    function getUnbondingTime() external view returns (uint256);
    function getSlashFractionDoubleSign() external view returns (uint256);
    function getSignedBlockWindow() external view returns (uint256);
    function getMinSignedPerWindow() external view returns (uint256);
    function getMinStake() external view returns (uint256);
    function getMinValidatorStake() external view returns (uint256);
    function getMinAmountChangeName() external view returns (uint256);
    function getMinSelfDelegation() external view returns (uint256);

}