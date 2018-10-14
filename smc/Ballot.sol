// Compiler: remix: 0.4.24+commit.e67f0147.Emscripten.clang

pragma solidity ^0.4.0;
contract Ballot {

    struct Voter {
        bool voted;
        uint8 vote;
    }
    struct Proposal {
        uint voteCount;
    }

    mapping(address => Voter) voters;
    Proposal[4] proposals;

    /// Give a single vote to proposal $(toProposal).
    function vote(uint8 toProposal) public {
        Voter storage sender = voters[msg.sender];
        if (sender.voted || toProposal >= proposals.length) return;
        sender.voted = true;
        sender.vote = toProposal;
        proposals[toProposal].voteCount += 1;
    }

    function getVote(uint8 toProposal) public view returns (uint) {
        if (toProposal >= proposals.length) return 0;
        return proposals[toProposal].voteCount;
    }

    function winningProposal() public view returns (uint8 _winningProposal) {
        uint256 winningVoteCount = 0;
        for (uint8 prop = 0; prop < proposals.length; prop++)
        if (proposals[prop].voteCount > winningVoteCount) {
            winningVoteCount = proposals[prop].voteCount;
            _winningProposal = prop;
        }
    }
}