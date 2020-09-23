
pragma solidity  ^0.6.0;


interface IStaking {

    // @dev Emitted when validator is created;
    event CreateValidator(
        address payable valAddr,
        uint256 amount,
        uint256 commissionRate,
        uint256 commissionMaxRate,
        uint256 commissionMaxChangeRate,
        uint256 minSelfDelegation
    );

    // @dev Emitted when validator is updated;
    event UpdateValidator(address valAddr, uint256 commissionRate, uint256 minSelfDelegation);

    // @dev Emitted when delegation rewards is withdraw
    event WithdrawDelegationRewards(
        address valAddr,
        address delAddr,
        uint256 rewards
    );

    // @dev Emitted when validater commission is withdraw
    event WithdrawCommissionReward(
        address valAddr,
        uint256 rewards
    );

    // @dev Emitted when
    event Delegate(
        address valAddr,
        address delAddr,
        uint256 amount
    );

    event Undelegate(
        address valAddr,
        address delAddr,
        uint256 amount,
        uint256 completionTime
    );

    event Withdraw(
        address valAddr,
        address delAddr,
        uint256 amount
    );


    event Minted(uint256 amount);
    event Burn(uint256 amount);
    event Slashed(address valAddr, uint256 power, uint256 reason);
    event UnJail(address valAddr);
    event Liveness(address valAddr, uint256 missedBlocks, uint256 blockHeight);
}
