pragma solidity ^0.5.8;

contract PoSHandler {
    function claimReward(address node, uint64 blockHeight) public {}
    function newConsensusPeriod(uint64 blockHeight) public {} // blockHeight is used to validate if sender is blockHeight's proposer
}