pragma solidity ^0.4.24;


// interface definition for validatorsContract which is used to check whether sender is validator.
contract ValidatorsContract {
    function isValidator(address sender) public pure returns (bool) {}
}

/**
 * Meta contract contains periods information of staking, voting and validators
 */
contract Meta {

    // stakingPeriod is a period that a staking process is valid.
    // when the time is expired, current staking process will be ended.
    // results will be used to create voting smart contract.
    // New staking smart contract will be created.
    uint64 stakingPeriod;

    // currentStaking stores the last key that staking smart contract is used.
    uint64 currentStaking;

    // stakingCounter counts the number of staking smart contracts were created so far.
    uint64 stakingCounter;

    // votingPeriod is a period that a period process is valid.
    // when the time is expired, voting results will be collected and
    // they are used to create validators smart contract.
    // New cresterd Validators smart contract then is added to a map that stores validators smart contract with validators counter as a key.
    uint64 votingPeriod;

    // currentVoting stores the last key that voting smart contract is used.
    uint64 currentVoting;

    // votingCounter counts the number of voting contracts were created so far.
    uint64 votingCounter;

    // validatorsPeriod is a period that validators won last voting process are valid.
    uint64 validatorsPeriod;

    // validatorsCounter counts the number of voting contracts were created so far.
    uint64 validatorsCounter;

    // lastValidators stores the last key that validators smart contract is used.
    uint64 currentValidators;

    // maxValidators indicates maximum number of validators allowed.
    uint maxValidators;

    uint256 minStakes;

    struct ContractInformation {
        address contractAddress;
        uint64 fromTime;
        uint64 toTime;
    }

    // stakings stores a map of created staking contract addresses. Key is the counter
    mapping(uint64=>ContractInformation) stakings;

    // votings stores a map of created voting contract addresses. Key is the counter
    mapping(uint64=>ContractInformation) votings;

    // validators stores a map of created validators contract addresses. Key is the counter
    mapping(uint64=>ContractInformation) validators;

    constructor(uint64 _stakingPeriod, uint64 _votingPeriod, uint64 _validatorsPeriod, uint _maxValidators, uint64 _minStakes, address initValidators, uint64 fromTime, uint64 toTime) public {
        validators[0] = ContractInformation(initValidators, fromTime, toTime);
        stakingPeriod = _stakingPeriod;
        votingPeriod = _votingPeriod;
        validatorsPeriod = _validatorsPeriod;
        currentStaking = 0;
        currentVoting = 0;
        currentValidators = 0;
        stakingCounter = 0;
        votingCounter = 0;
        validatorsCounter = 1;
        maxValidators = _maxValidators;
        minStakes = _minStakes;
    }

    // get current validators contract and check if sender is validator or not in `isValidator` function.
    modifier isValidator {
        address contractAddress = validators[currentValidators].contractAddress;
        ValidatorsContract validatorsContract = ValidatorsContract(contractAddress);
        require(
            validatorsContract.isValidator(msg.sender) == true,
            "Only validators can access this function"
        );
        _;
    }

    // setStaking sets contractAddress to stakings with given fromTime and toTime, after all increment stakingCounter by 1.
    function setStaking(address contractAddress, uint64 fromTime, uint64 toTime) public isValidator {
        ContractInformation memory stakeContractInformation;
        stakeContractInformation = ContractInformation(contractAddress, fromTime, toTime);
        stakings[stakingCounter] = stakeContractInformation;
        stakingCounter = stakingCounter + 1;
    }

    // setVoting sets contractAddress to votings, update currentStaking in order to get correct staking contractAddress lately, after all increment votingCounter by 1.
    function setVoting(address contractAddress, uint64 fromTime, uint64 toTime) public isValidator {
        currentStaking = currentStaking + 1;
        ContractInformation memory contractInformation;
        contractInformation = ContractInformation(contractAddress, fromTime, toTime);
        votings[votingCounter] = contractInformation;
        votingCounter = votingCounter + 1;
    }

    // setVoting sets contractAddress to votings, update currentVoting in order to get correct voting contractAddress lately, after all increment validatorsCounter by 1.
    function setValidators(address contractAddress, uint64 fromTime, uint64 toTime) public isValidator {
        currentVoting = currentVoting + 1;
        ContractInformation memory contractInformation;
        contractInformation = ContractInformation(contractAddress, fromTime, toTime);
        validators[validatorsCounter] = contractInformation;
        validatorsCounter = validatorsCounter + 1;
    }

    // updateValidatorsToKardia increment currentValidators by 1.
    function updateValidatorsToKardia() public isValidator {
        currentValidators = currentValidators + 1;
    }

    // getCurrentStakingInfo gets current used staking smart contract address and its valid date
    function getCurrentStakingInfo() public view returns (address,uint64,uint64) {
        ContractInformation memory contractInformation;
        contractInformation = stakings[currentStaking];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getStakingInfo gets staking smart contract address and its valid date based on index
    function getStakingInfo(uint64 index) public view returns (address, uint64, uint64) {
        ContractInformation memory contractInformation;
        contractInformation = stakings[index];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getCurrentStakingIndex gets current used staking index.
    function getCurrentStakingIndex() public view returns (uint64) {
        return currentStaking;
    }

    // getTotalStaking gets total stakings created so far.
    function getTotalStaking() public view returns (uint64) {
        return stakingCounter;
    }

    // getCurrentVotingInfo gets current used voting smart contract address and its valid date.
    function getCurrentVotingInfo() public view returns (address,uint64,uint64) {
        ContractInformation memory contractInformation;
        contractInformation = votings[currentVoting];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getVotingInfo gets voting smart contract address and its valid date based on index
    function getVotingInfo(uint64 index) public view returns (address, uint64, uint64) {
        ContractInformation memory contractInformation;
        contractInformation = votings[index];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getCurrentVotingIndex gets current used voting index.
    function getCurrentVotingIndex() public view returns (uint64) {
        return currentVoting;
    }

    // getTotalVoting gets total votings created so far.
    function getTotalVoting() public view returns (uint64) {
        return votingCounter;
    }

    // getValidatorsInfo gets current used validators smart contract address and its valid date.
    function getCurrentValidatorsInfo() public view returns (address,uint64,uint64) {
        ContractInformation memory contractInformation;
        contractInformation = validators[currentValidators];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getCurrentValidatorsInfo gets validators smart contract address and its valid date based on index.
    function getValidatorsInfo(uint64 index) public view returns (address, uint64, uint64) {
        ContractInformation memory contractInformation;
        contractInformation = validators[index];
        return (contractInformation.contractAddress, contractInformation.fromTime, contractInformation.toTime);
    }

    // getCurrentValidatorsIndex gets current used validators index.
    function getCurrentValidatorsIndex() public view returns (uint64) {
        return currentValidators;
    }

    // getTotalVoting gets total validators created so far.
    function getTotalValidators() public view returns (uint64) {
        return validatorsCounter;
    }

    function maxValidators() public view returns (uint) {
        return maxValidators;
    }

    function minStakes() public view returns (uint256) {
        return minStakes;
    }
}
