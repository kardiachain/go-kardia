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

    struct Request {
        string email;
        string fromOrgId;
        bool isComplete;
    }

    struct Response {
        string fromOrgId;
        string content;
        bool isSet;
    }


    Request[] listRequest;

    event ExternalCandidateInfoRequested(string email, string fromOrgId, string toOrgId);
    event RequestCompleted(string email, string answer, string toOrgId);
    mapping(string=>CandidateInfo) candidateList;
    mapping(string=>Response) externalResponse;

    // getCandidateInfo returns info of a candidate specified by email,
    // in which add is address in source blockchain , isExternal indicates if candidate comes from external source,
    // source is symbol of the source blockchain that candidate comes from,
    // returns ("", "", 0, 0x, false, "") if not found
    function getCandidateInfo(string _email) public view returns (string name, string email, uint age, address addr, bool isExternal, string source) {
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

    //addRequest adds an external candidate info request into requestList
    function addRequest(string email, string fromOrgId) public {
        Request memory r = Request(email, fromOrgId, false);
        listRequest.push(r);
    }

    //completeRequest fire event to send info of requested candidate and removes request from list
    function completeRequest(uint _requestID, string _email, string _content, string _toOrgId) public returns (uint8) {
        if (keccak256(abi.encodePacked(listRequest[_requestID].email)) != keccak256(abi.encodePacked(_email))) {
            return 0;
        }
        if (listRequest[_requestID].isComplete) {
            return 0;
        }
        if (keccak256(abi.encodePacked(listRequest[_requestID].fromOrgId)) != keccak256(abi.encodePacked(_toOrgId)) ) {
            return 0;
        }
        if (!candidateList[_email].isSet) {
            return 0;
        }

        // remove request from list
        listRequest[_requestID].isComplete = true;

        emit RequestCompleted(_email, _content, listRequest[_requestID].fromOrgId);
        return 1;
    }

    //getRequests returns list of requests in a comma-separated string
    function getRequests() public view returns (string requestList) {
        if (listRequest.length == 0) {
            return "";
        }
        string memory results = "";
        for (uint i=0; i < listRequest.length; i++) {
            if (keccak256(abi.encodePacked(results)) != keccak256(abi.encodePacked(""))) {
                results = concatStr(results, ",");
            }
            if (listRequest[i].isComplete == false) {
                results = concatStr(results, requestToString(listRequest[i], i));
            }
        }
        return results;
    }

    // addExternalResponse adds an external response for a candidate
    function addExternalResponse(string _email, string _fromOrgId, string _content) public {
        Response memory r = Response(_fromOrgId, _content, true);
        externalResponse[_email] = r;
    }

    // getExternalResponse returns an external response for a candidate from a specific orgID
    function getExternalResponse(string _email, string _fromOrgId) public view returns (string content) {
        if (externalResponse[_email].isSet == false) {
            return "";
        }
        if (keccak256(abi.encodePacked(externalResponse[_email].fromOrgId)) != keccak256(abi.encodePacked(_fromOrgId))) {
            return "";
        }
        return externalResponse[_email].content;
    }
    function concatStr(string a, string b) internal pure returns (string) {
        return string(abi.encodePacked(a, b));
    }

    function requestToString(Request r, uint _id) internal pure returns (string) {
        string memory strId = uint2str(_id);
        return string(abi.encodePacked(strId, ":", r.email, ":", r.fromOrgId));
    }

    function uint2str(uint i) internal pure returns (string){
        if (i == 0) return "0";
        uint j = i;
        uint length;
        while (j != 0){
            length++;
            j /= 10;
        }
        bytes memory bstr = new bytes(length);
        uint k = length - 1;
        while (i != 0){
            bstr[k--] = byte(48 + i % 10);
            i /= 10;
        }
        return string(bstr);
    }
}

