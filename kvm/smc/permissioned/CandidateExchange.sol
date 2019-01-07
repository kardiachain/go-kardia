pragma solidity ^0.4.24;

// CandidateExchange forward candidate info requests and results to targeted private chains
contract CandidateExchange {
    event ExternalCandidateInfoRequested(string email, string fromOrgID, string toOrgID);
    event ExternalCandidateInfoFulfilled(string email, string name, uint8 age, address addr, string source, string fromOrgID, string toOrgID);

    // This function is used to notify chain B a request come from chain A by using event
    // ExternalCandidateInfoRequested. Dual node Kardia - B will catch this event and response
    function RequestCandidateInfo(string _email, string _fromOrgID, string _toOrgID) public {
        emit ExternalCandidateInfoRequested(_email, _fromOrgID, _toOrgID);
    }

    // This function is used to notify chain A a candidate info responded by chainB by using event
    // ExternalCandidateInfoFulfilled. Dual node Kardia - A will catch this event and store it into
    // CandidateDB of chain A
    function FulfilledCandidateInfo(string _email, string _name, uint8 _age, address _addr, string _source, string _fromOrgID, string _toOrgID) {
        emit ExternalCandidateInfoFulfilled(_email, _name, _age, _addr, _source, _fromOrgID, _toOrgID);
    }
}
