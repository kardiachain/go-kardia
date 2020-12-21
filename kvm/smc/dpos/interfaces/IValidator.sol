// SPDX-License-Identifier: MIT
pragma solidity ^0.5.0;

interface IValidator {
    function initialize (
        bytes32 _name, 
        address _owner,
        uint256 _rate, 
        uint256 _maxRate, 
        uint256 _maxChangeRate, 
        uint256 _minSelfDelegation
    ) external;
    function update(bytes32 _name, uint256 _commissionRate, uint256 _minSelfDelegation) external;
    function unjail() external;
    function allocateToken(uint256 _rewards) external;
    function delegate() external payable;
    function withdrawRewards() external;
    function withdrawCommission()external;
    function withdraw() external;
    function undelegate() external;
    function undelegateWithAmount(uint256 _amount) external;
    function getCommissionRewards() external view returns (uint256);
    function getDelegationRewards(address _delAddr) external view returns (uint256);
    function getDelegations() external view returns (address[] memory, uint256[] memory);
    function validateSignature(uint256 _votingPower, bool _signed) external;
    function doubleSign(
        uint256 votingPower,
        uint256 distributionHeight
    ) external;
    function start() external;
    function stop() external;

    // @dev Emitted when validator is updated;
    event UpdateValidator(
        bytes32 _name,
        uint256 _commissionRate,
        uint256 _minSelfDelegation
    );

    // @dev Emitted when validater commission is withdraw
    event WithdrawCommissionReward(uint256 _rewards);

    // @dev Emitted when
    event Delegate(address _delAddr, uint256 _amount);

    event Undelegate(
        address _delAddr,
        uint256 _amount,
        uint256 _completionTime
    );

    event Withdraw(address _delAddr, uint256 _amount);
    event Slashed(uint256 _power, uint256 _reason);
    event Liveness(uint256 _missedBlocks, uint256 _blockHeight);
    event UpdatedSigner(address previousSigner, address newSigner);

    event Started();
    event Stopped();
}