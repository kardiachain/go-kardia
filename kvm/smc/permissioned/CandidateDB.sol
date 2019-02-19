pragma solidity ^0.4.24;

// CandidateDB stores info of all candidates that an organization has recorded
contract CandidateDB {
    struct Request {
        string email;
        string fromOrgId;
        string content;
        bool isComplete;
        uint blockAdded;
        uint blockCompleted;
    }

    struct Response {
        string email;
        string fromOrgId;
        string content;
        bool isSet;
        uint blockAdded;
    }


    Request[] listRequest;
    Response[] listResponse;

    event ExternalCandidateInfoRequested(string email, string fromOrgId, string toOrgId);
    event RequestCompleted(string email, string answer, string toOrgId);

    mapping(string=>Response) externalResponse;

    // requestCandidateInfo fires an event to be caught by Kardia dual node to request candidate info from another private blockchain
    function requestCandidateInfo(string _email, string _fromOrgId, string _toOrgId) public {
        emit ExternalCandidateInfoRequested(_email, _fromOrgId, _toOrgId);
    }

    //addRequest adds an external candidate info request into requestList
    function addRequest(string email, string fromOrgId) public {
        Request memory r = Request(email, fromOrgId, "", false, block.number, 0);
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
        // remove request from list
        listRequest[_requestID].isComplete = true;
        listRequest[_requestID].content = _content;
        listRequest[_requestID].blockCompleted = block.number;
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

    //getRequests returns list of requests in a comma-separated string
    function getCompletedRequests() public view returns (string requestList) {
        if (listRequest.length == 0) {
            return "";
        }
        string memory results = "";
        for (uint i=0; i < listRequest.length; i++) {
            if (listRequest[i].isComplete == true) {
                if (keccak256(abi.encodePacked(results)) != keccak256(abi.encodePacked(""))) {
                    results = concatStr(results, ",");
                }
                results = concatStr(results, requestToString(listRequest[i], i));
            }
        }
        return results;
    }

    //getResponses returns list of requests in a comma-separated string
    function getResponses() public view returns (string responseList) {
        if (listResponse.length == 0) {
            return "";
        }
        string memory results = "";
        for (uint i=0; i < listResponse.length; i++) {
            if (keccak256(abi.encodePacked(results)) != keccak256(abi.encodePacked(""))) {
                results = concatStr(results, ",");
            }
            if (listResponse[i].isSet == true) {
                results = concatStr(results, responseToString(listResponse[i], i));
            }
        }
        return results;
    }

    // addExternalResponse adds an external response for a candidate
    function addExternalResponse(string _email, string _fromOrgId, string _content) public {
        Response memory r = Response(_email, _fromOrgId, _content, true, block.number);
        listResponse.push(r);
    }

    // getExternalResponse returns an external response for a candidate from a specific orgID
    function getExternalResponse(string _email, string _fromOrgId) public view returns (string content) {
        for (uint i=0; i < listResponse.length; i++) {
            if (keccak256(abi.encodePacked(listResponse[i].email)) == keccak256(abi.encodePacked(_email))
                && keccak256(abi.encodePacked(listResponse[i].fromOrgId)) == keccak256(abi.encodePacked(_fromOrgId))) {
                return listResponse[i].content;
            }
        }
        return "";
    }

    function concatStr(string a, string b) internal pure returns (string) {
        return string(abi.encodePacked(a, b));
    }

    function requestToString(Request r, uint _id) internal pure returns (string) {
        string memory strId = uint2str(_id);
        if (r.isComplete) {
            return string(abi.encodePacked(strId, ":", r.email, ":", r.fromOrgId, ":", r.content, ":", uint2str(r.blockAdded), ":",uint2str(r.blockCompleted)));
        }
        return string(abi.encodePacked(strId, ":", r.email, ":", r.fromOrgId, ":", uint2str(r.blockAdded)));
    }

    function responseToString(Response r, uint _id) internal pure returns (string) {
        string memory strId = uint2str(_id);
        return string(abi.encodePacked(strId, ":", r.email, ":", r.fromOrgId, ":", r.content, ":", uint2str(r.blockAdded)));
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