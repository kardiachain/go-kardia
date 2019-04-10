pragma solidity ^0.5.0;

contract Pos {
    /* TODO(huny): Add delegator next
    struct Delegator {
        address addr;
        uint amount;
    }
    */
    struct Candidate {
        uint totalStake;
        bool existed;
        /* TODO(huny):
        uint numDelegators;
        Delegator[] delegators;
        */
    }
 
    mapping(address => Candidate) public candidateMap;
    address[] public candidateList;
    
    address[10] public validatorList;
    
    function stake(address candidate) public payable {
        Candidate storage c = candidateMap[candidate];
        // Add this candidate to the list if it does not exist
        if (c.existed == false) {
            c.existed = true;
            candidateList.push(candidate); 
        }
        c.totalStake += msg.value;
        // TODO(huny): Renable this: c.delegators[c.numDelegators++] = Delegator({addr: msg.sender, amount: msg.value});
        updateValidatorList(candidate, c.totalStake);
    }

    // Update the validator list if this candidate has enough stake to join the list
    function updateValidatorList(address addr, uint currentStake) private {
        uint i = 0;
        // get the index of the current max element
        for(i; i < validatorList.length; i++) {
            if(candidateMap[validatorList[i]].totalStake < currentStake) {
                break;
            }
        }
        // shift the array of position (getting rid of the last element)
        for(uint j = validatorList.length - 1; j > i; j--) {
            validatorList[j] = validatorList[j - 1];
        }
        // update the new max element
        validatorList[i] = addr;
    }
     
    function getValidatorList() public view returns (address[10] memory) {
        address[10] memory valList = validatorList;
        return valList;
    }
    
    function getCandidateCount() public view returns (uint candidateCount) {
        return candidateList.length;
    }
}