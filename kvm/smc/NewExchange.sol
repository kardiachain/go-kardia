pragma solidity ^0.4.24;
contract KardiaExchange {
    struct ExchangeRequest {
        string pair;
        string fromAddress;
        string toAddress;
        uint256 sellAmount;
        uint256 receiveAmount;
        uint256 matchedRequestId;
        uint done;
    }
    struct Rate {
        uint sellAmount;
        uint receiveAmount;
    }
    mapping (uint256 => ExchangeRequest) listRequests;
    mapping (string => uint256[]) requestIDsByPairs;
    mapping (string => Rate) rates;
    uint256 counter;
    event Release(
        string indexed pair,
        string indexed addr,
        uint256 matchRequestId,
        uint256 _value
);

    // if 1 eth = 10 neo, then sale_amount = 1, receive_amount = 10
    function addRate(string pair, uint sale_amount, uint receiveAmount) public {
        rates[pair] = Rate(sale_amount, receiveAmount);
    }
    // pair should be "ETH-NEO" for order from ETH to NEO and vice versa
    function getRate(string pair) internal view returns (Rate) {
        return rates[pair];
    }

    // pair should be "ETH-NEO" for order from ETH to NEO and vice versa
    function getRatePublic(string pair) public view returns (uint sale, uint receive) {
        return (rates[pair].sellAmount, rates[pair].receiveAmount);
    }

    // create an order with source - dest address, source - dest pair and amount
    // order will be stored with returned ID in smc
    // order from ETH from NEO should be "ETH-NEO", "NEO-ETH"
    function matchRequest(string srcPair, string destPair, string srcAddress, string destAddress, uint256 amount) public returns (uint256) {
        Rate memory r = getRate(srcPair);
        if (r.receiveAmount == 0 || r.sellAmount == 0 ) {
            return 0;
        }
        ++counter;
        uint256 receiveAmount = amount * r.receiveAmount / r.sellAmount;
        uint256 matchRequestId = findMatchingRequest(destPair, receiveAmount);
        ExchangeRequest memory request = ExchangeRequest(srcPair, srcAddress, destAddress, amount, receiveAmount, matchRequestId, 0);
        listRequests[counter] = request;
        requestIDsByPairs[srcPair].push(counter);
        if (matchRequestId != 0) {
            listRequests[matchRequestId].matchedRequestId = counter;
            emit Release(destPair, listRequests[matchRequestId].toAddress, matchRequestId, listRequests[matchRequestId].receiveAmount);
        }
        return counter;
    }

    // find matching request for targeted pair with specific amount, return global request ID
    function findMatchingRequest(string destPair, uint256 receiveAmount) internal view returns (uint256) {
        uint256[] memory ids = requestIDsByPairs[destPair];
        for (uint256 i = 0; i < ids.length; i++) {
            ExchangeRequest memory request = listRequests[ids[i]];
            if (request.sellAmount == receiveAmount && request.matchedRequestId == 0 && request.done == 0) {
                return ids[i];
            }
        }
        // no matching request ID found
        return 0;
    }

    // Get request id from its details:
    function getRequestId(string sourcePair, string fromAddress, string toAddress, uint256 amount) internal view returns (uint256) {
        uint256[] memory ids = requestIDsByPairs[sourcePair];
        for (uint256 i = 0; i < ids.length; i++) {
            ExchangeRequest memory request = listRequests[ids[i]];
            if (keccak256(abi.encodePacked(request.fromAddress)) == keccak256(abi.encodePacked(fromAddress)) && keccak256(abi.encodePacked(request.toAddress)) == keccak256(abi.encodePacked(toAddress)) && request.sellAmount == amount) {
                return ids[i];
            }
        }
        // no request found
        return 0;
    }

    // Get matched request detail for a specific request
    function getMatchingRequestInfo(string sourcePair, string interestedPair, string fromAddress, string toAddress, uint256 amount) public view returns (uint256 matchedRequestID, string destAddress, uint256 sendAmount) {
        uint256 requestID = getRequestId(sourcePair, fromAddress, toAddress, amount);
        if (requestID == 0) {
            return (0, "", 0);
        }
        if (keccak256(abi.encodePacked(interestedPair)) == keccak256(abi.encodePacked(listRequests[requestID].pair))
            && listRequests[requestID].matchedRequestId != 0) {
            return (requestID, listRequests[requestID].toAddress, listRequests[requestID].receiveAmount);
        }
        uint256 matchedId = listRequests[requestID].matchedRequestId;
        if (matchedId != 0) {
            ExchangeRequest memory matchedRequest = listRequests[matchedId];
            return (matchedId, matchedRequest.toAddress, matchedRequest.receiveAmount);

        }
        // no matching request found
        return (0, "", 0);
    }

    // Complete a request indicates that the request with requestID has been release successfully
    // TODO(@sontranrad): implements retry logic in case release is failed
    function completeRequest(uint256 requestID, string pair) public returns (uint success){
        ExchangeRequest memory request = listRequests[requestID];
        if (keccak256(abi.encodePacked(request.pair)) != keccak256(abi.encodePacked(pair)) ) {
            return 0;
        }
        if (listRequests[requestID].done != 0) {
            return 0;
        }
        listRequests[requestID].done = 1;
        uint256 matchRequestID = listRequests[requestID].matchedRequestId;
        removeRequest(requestID, matchRequestID, listRequests[requestID].pair);
        return 1;
    }

    // Get exchangeable amount of each pair
    function getAvailableAmountByPair(string pair) public view returns (uint256 amount) {
        amount = 0;
        uint256[] memory ids = requestIDsByPairs[pair];
        for (uint i = 0; i < ids.length; i++) {
            if (listRequests[ids[i]].matchedRequestId == 0)
                amount += listRequests[ids[i]].receiveAmount;
        }
        return amount;
    }

    // Get 10 matchable amount by pair
    function getMatchableAmount(string pair) public view returns (uint256[] amounts) {
        uint256[] memory ids = requestIDsByPairs[pair];
        amounts = new uint256[](10);
        for (uint i = 0; i < ids.length; i++) {
            if (i < 10) {
                amounts[i] = listRequests[ids[i]].receiveAmount;
            }
        }
        return amounts;
    }

    // Remove a specific request
    function removeRequest(uint256 requestID, uint256 matchedRequestID, string pair) internal {
        uint256[] memory ids = requestIDsByPairs[pair];
        uint requestToRemove = ids.length;
        for (uint i = 0; i < ids.length; i++) {
            if (ids[i] == requestID) {
                requestToRemove = i;
                break;
            }
        }
        if (requestToRemove < ids.length) {
            requestIDsByPairs[pair][requestToRemove] == requestIDsByPairs[pair][ids.length - 1];
            requestIDsByPairs[pair].length--;
        }
        if (listRequests[requestID].done == 1 && listRequests[matchedRequestID].done == 1) {
            delete listRequests[requestID];
            delete listRequests[matchedRequestID];
        }

    }

    // Get the opposite uncompleted request of a compelete request
    function getUncompletedMatchingRequest(uint256 requestID) public view returns (uint256 matchedRequestID, string destAddress, uint256 sendAmount) {
        uint256 oppositeID = listRequests[requestID].matchedRequestId;
        if (oppositeID == 0) {
            return (0, "", 0);
        }
        ExchangeRequest memory oppositeRequest = listRequests[oppositeID];
        if (oppositeRequest.done == 0) {
            return (oppositeID, oppositeRequest.toAddress, oppositeRequest.receiveAmount);
        }
    }
}
