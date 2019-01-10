pragma solidity ^0.4.24;

// CandidateDB stores info of all candidates that an organization has recorded
contract CandidateDB {
    struct CandidateInfo {
        string name;
        string email;
        uint8 age;
        address addr;
        bool isExternal;
        string source;
        bool isSet;
    }
    event ExternalCandidateInfoRequested(string email, string fromOrgId, string toOrgId);
    mapping(string=>CandidateInfo) candidateList;

    // getCandidateInfo returns info of a candidate specified by address,
    // in which add is address in source blockchain , isExternal indicates if candidate comes from external source,
    // source is symbol of the source blockchain that candidate comes from,
    // returns ("", "", 0, 0x, false, "") if not found
    function getCandidateInfo(string _email) public view returns (string name, string email, uint age, address add, bool isExternal, string source) {
        if (candidateList[_email].isSet) {
            return (candidateList[_email].name, _email, candidateList[_email].age, candidateList[_email].addr,
                candidateList[_email].isExternal, candidateList[_email].source);
        }
        return ("", "", 0, 0x0, false, "");
    }

    // requestCandidateInfo fires an event to be caught by Kardia dual node to request candidate info from another private blockchain
    function requestCandidateInfo(string _email, string _fromOrgId, string _toOrgId) public {
        emit ExternalCandidateInfoRequested(_email, _fromOrgId, _toOrgId);
    }

    // updateCandidateInfo adds info of a candidate from current private blockchain
    function updateCandidateInfo(string _name, string _email, uint8 _age, address _addr, string source) public {
        CandidateInfo memory info = CandidateInfo(_name, _email, _age, _addr, false, source, true);
        candidateList[_email] = info;
    }

    // updateCandidateInfo stores candidateInfo from external blockchain submitted by Kardia dual node
    // we separate this from previous function for adding authorization later
    function updateCandidateInfoFromExternal(string _name, string _email, uint8 _age, address _addr, string source) public {
        CandidateInfo memory info = CandidateInfo(_name, _email, _age, _addr, true, source, true);
        candidateList[_email] = info;
    }
}
