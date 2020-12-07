pragma solidity ^0.5.0;

interface IMinter {
    function mint() external returns (uint64);
    function feesCollected() external view returns (uint256);
}