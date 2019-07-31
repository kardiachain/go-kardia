pragma solidity >=0.4.22 <0.6.0;

contract DPOS_Voting {

    /** The validator applicant needs to provide his info in order to participant in the network:
     * - Validator's PubKey
     * - Validator's name
     * - Validator's website (Optional) / Validator's description (Optional)
     * - Initial self-bond amount: how much node is staking for itself
     * - Dividend ratio: the ratio in which it is sharing the profit with its stakers
     **/
    struct ValidatorInfo {
        string pubKey;
        string name;
        uint selfStake;
        string dividendRatio; // eg. "40/60"
        string description;
    }

    struct Candidate {
        uint stakes;
        uint ranking;
        bool exist;
        ValidatorInfo info;
        uint numOfVoters;
        Voter[] myVoters; //map voter address to staked amount
        /* - If the nodes they delegated become the validators, these nodes will start paying them a proportional rewards
           - If the nodes they delegated failed to be elected, their delegated KAI will be returned.
        */
    }

    struct Voter {
        address payable voter;
        uint stakedAmount;
    }

    // State variables
    address public owner;
    bool public voteEnded;
    uint public numOfValidators;
    mapping (address => Candidate) candidates;
    address[] public rankings; // keeps track of candidate rankings, position 1 is the highest
    address[] public validatorList; // stores list of validators after vote ended

    // Events
    event Signup(address candidate, uint stakes);
    event VoteCast(address voter, address candidate, uint stakes);
    event CandidateRanking(address candidate, uint position);
    event Refund(address voter, uint value);


    // Modifiers
    modifier onlyOwner{
        require(msg.sender == owner);
        _;
    }
    modifier checkValue {
        require(msg.value > 0);
        _;
    }
    modifier voteNotEnded {
        require(!voteEnded);
        _;
    }


    // Functions
    function init(uint n) public {
        owner = msg.sender;
        voteEnded = false;
        numOfValidators = n;
        validatorList.length = numOfValidators;
        rankings.push(address(0x0)); // dummy addr at position 0
    }

    function signup(string memory pubKey, string memory name, string memory ratio, string memory description)
        public payable voteNotEnded checkValue {
        if (candidates[msg.sender].exist) {
            revert();
        }
        ValidatorInfo memory newInfo = ValidatorInfo(pubKey, name, msg.value, ratio, description);
        candidates[msg.sender].stakes = msg.value;
        candidates[msg.sender].info = newInfo;
        candidates[msg.sender].exist = true;

        //Ranking
        rankings.push(msg.sender);
        candidates[msg.sender].ranking = rankings.length-1;
        sortByRanking(rankings.length-1);
        emit Signup(msg.sender, msg.value);
    }

    function vote(address candAddress) public payable voteNotEnded checkValue{
        if (!candidates[candAddress].exist) {
            revert();
        }
        // Update candidate votes
        Candidate storage c = candidates[candAddress];
        c.stakes += msg.value;

        // Update voter staked amount
        c.myVoters.push(Voter(msg.sender, msg.value));
        c.numOfVoters += 1;

        // Ranking
        sortByRanking(c.ranking);
        candidates[candAddress] = c;
        emit VoteCast(msg.sender, candAddress, msg.value);
    }

    /** Sorts the rankings array assuming the rest of the array is already sorted
     *  and the position of a candidate can only move up to higher rank.
     *  Returns the candidate's position after the sort.
     **/
    function sortByRanking(uint index) private {
        for (uint i = index; i > 1; i--) {
            if (candidates[rankings[i]].stakes <= candidates[rankings[i-1]].stakes) {
                break;
            }
            // Swap the candidate up one rank
            candidates[rankings[i]].ranking = i-1;
            candidates[rankings[i-1]].ranking = i;

            address temp = rankings[i];
            rankings[i] = rankings[i-1];
            rankings[i-1] = temp;
        }
    }

    /** Returns a candidate
     **/
    function getCandidateStake(address candAddress) public view returns (uint){
        return candidates[candAddress].stakes;
    }

    /** Returns the current ranking of a candidate
     **/
    function getCandidateRanking(address candAddress) public returns (uint){
        uint position = candidates[candAddress].ranking;
        assert(rankings[position] == candAddress);
        emit CandidateRanking(candAddress, position);
        return position;
    }

    /** Returns the current list of all rankings
     **/
    function getCurrentRankings() public view returns (address[] memory) {
        return rankings;
    }

    /** Returns the list of result after vote ended
     **/
    function getValidatorList() public view returns (address[] memory) {
        return validatorList;
    }

    /** Ends the vote and copies final result to validatorList and
     *  refund for voters whose candidates failed to be elected
     **/
    function endVote() public payable voteNotEnded onlyOwner{
        voteEnded = true;

        // Copy final result to validatorList
        for (uint i = 1; i <= numOfValidators && i < rankings.length; i++) {
            validatorList[i-1] = rankings[i];
        }

        // Refund to voters
        for (uint i = numOfValidators + 1; i < rankings.length; i++) {
            Candidate memory c = candidates[rankings[i]];
            for (uint k = 0; i < c.myVoters.length; i++) {
                c.myVoters[k].voter.transfer(c.myVoters[k].stakedAmount);
                candidates[rankings[i]].myVoters[k].stakedAmount = 0;
                emit Refund(c.myVoters[k].voter, c.myVoters[k].stakedAmount);
            }
        }
    }

    /** Returns the current balance of the address
     **/
    function getBalance(address addr) view public returns(uint) {
      return addr.balance;
    }

}
