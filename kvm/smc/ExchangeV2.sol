pragma solidity ^0.4.24;

contract KardiaExchange {

    uint constant PENDING = 0;
    uint constant DONE = 1;
    uint constant REJECTED = 2; // This status will not be used now, just add it for future used
    address owner;
    uint constant UPDATE = 0;
    uint constant DELETE = 1;

    modifier onlyadmin {
        require(
            authorizedUsers[msg.sender] == true || msg.sender == owner,
            "Only admin can call this function."
        );
        _;
    }

    modifier isRoot {
        require(
            msg.sender == owner,
            "Only root can call this function."
        );
        _;
    }

    // setOwner if owner is empty, set sender as owner
    function setOwner() public {
        if (owner == 0x0) {
            owner = msg.sender;
        }
    }

    // authorizedUsers is a list of authorized senders that can trigger smc functions
    mapping(address=>bool) authorizedUsers;

    // availableTypes stores allowed type (NEO, TRX, etc.) as key and their target smart contract address as value.
    mapping(string=>string) availableTypes;

    struct Order {
        // fromType and toType is used to specified the rate
        string fromType;
        string toType;

        // fromAddress is address of sender is his chain (NEO or ETH, etc.)
        string fromAddress;

        // receiver is targeted address that assets are transferred to
        string receiver;

        // txId is the transactionId created from sender
        string txId;

        // txId created by kardia
        string kardiaTxId;

        // txIds of target chain
        string targetTxId;

        // amount is the number that is converted from the following syntax
        // amount divided by the rate of fromType and toType
        uint256 amount;

        // availableAmount is unmatched Amount, it will be paired from the order from toType
        uint256 availableAmount;

        // status of an order: 0 is pending, 1 is finish, 2 is rejected;
        uint done;

        // timestamp stores the time order is created. Note: backend should be able to get UTC-0 time.
        uint256 timestamp;

        // This field is only used to check if Order is added or not
        bool existed;
    }

    /*
        IndexInfo stores needed information to query Order in allOrders
    */
    struct IndexInfo {
        string _address;
        string fromType;
        string toType;
        uint index;
    }

    /*
    The first key is fromType, the second key is toType and the value is list of pendingOrders
    In checking matching of an Order. We will do the following steps:
    1. If an order (1) is in a reversed way then toType and fromType will be fromType and toType of others Orders.
    Then get toType and fromType from Order orderly as first key and second key to get list of pendingOrders
    2. Then every matched orders will be stored into a list (2)
    3. (2) will be updated into allOrders with:
        a. first key as fromAddress
        b. second is fromType
        c. third key is toType
    of (1)
    4. When looping and get (2), (1) also update its availableAmount. Then update (1) allOrders with:
        a. first key as fromAddress
        b. second is fromType
        c. third key is toType
    */
    mapping(string => mapping(string => Order[])) pendingOrders;

    // allOrders is used to track history of a specific address based on fromType and toType
    // first key is fromAddress, second key is fromType and third Key is toType
    mapping(string => mapping(string => mapping(string => Order[]))) allOrders;

    // rates is map of rate that first key is fromType, second key is toType
    mapping(string => mapping(string => Rate)) rates;

    // addedTxs stores all txids that are added to smart contract
    mapping(string => bool) addedTxs;

    // store info to query Order in allOrders with key is txid
    mapping(string => IndexInfo) indexes;

    /*
    matchingResult stores 4 elements which are toTypes, releasedAddresses, amounts, txs
    which is result from doMatchingOrder function.
    */
    mapping(string => string[]) matchingResult;

    struct Rate {
        uint256 fromAmount;
        uint256 receivedAmount;
    }

    // getOwner returns root address
    function getOwner() public view returns (address) {
        return owner;
    }

    // authorizedUser authorize an address to be admin
    function authorizedUser(address user) public isRoot {
        authorizedUsers[user] = true;
    }

    // deAuthorizedUser removes an address out of admin list
    function deAuthorizedUser(address user) public isRoot {
        authorizedUsers[user] = false;
    }

    function getRate(string fromType, string toType) public view returns (uint256 fromAmount, uint256 receivedAmount) {
        if (rates[fromType][toType].fromAmount != 0) {
            return (rates[fromType][toType].fromAmount, rates[fromType][toType].receivedAmount);
        }
        return (0, 0);
    }

    function updateRate(string fromType, string toType, uint256 fromAmount, uint256 receivedAmount) public onlyadmin {
        rates[fromType][toType] = Rate(fromAmount, receivedAmount);
    }

    // hasTxId returns true if order has been added to smart contract
    function hasTxId(string txId) public view returns (bool) {
        return addedTxs[txId];
    }

    // getAddressFromType gets smart contract address from type (NEO, TRX, etc.)
    function getAddressFromType(string _type) public view returns (string) {
        if (keccak256(abi.encodePacked(availableTypes[_type])) != keccak256(abi.encodePacked("")))
            return availableTypes[_type];
        return "";
    }

    // updateAvailableType updates address to type
    function updateAvailableType(string _type, string _address) public onlyadmin {
        availableTypes[_type] = _address;
    }

    function orderToString(Order order) internal pure returns (string) {
        // because delete function could leave the gap in a list therefore order might be empty
        if (!order.existed) return "";
        return string(abi.encodePacked(
                order.fromType, ";", order.toType, ";", order.fromAddress, ";", order.receiver, ";", uint2str(order.amount),
                ";", uint2str(order.availableAmount), ";", order.txId, ";", order.kardiaTxId, ";", order.targetTxId, ";",
                uint2str(order.timestamp), ";", uint2str(order.done)));
    }

    // ordersToString converts a list of orders to string
    function ordersToString(Order[] orders) internal pure returns (string) {
        if (orders.length > 0) {
            string memory result = "";
            for (uint i=0; i<orders.length; i++) {
                string memory order = orderToString(orders[i]);
                if (keccak256(abi.encodePacked(order)) != keccak256(abi.encodePacked(""))) {
                    result = concatStr(result, orderToString(orders[i]), "|");
                }
            }
            return result;
        }
        return "";
    }

    // pendingOrdersToString converts a list of pending orders to string
    function pendingOrdersToString(Order[] orders) internal pure returns (string) {
        if (orders.length > 0) {
            string memory result = "";
            for (uint i=0; i<orders.length; i++) {
                if (orders[i].availableAmount == 0) continue;
                string memory order = orderToString(orders[i]);
                if (keccak256(abi.encodePacked(order)) != keccak256(abi.encodePacked(""))) {
                    result = concatStr(result, orderToString(orders[i]), "|");
                }
            }
            return result;
        }
        return "";
    }

    // getPendingOrders return a list of orders, this function is used to get Buy/Sell Orders
    function getPendingOrders(string fromType, string toType) public view returns (string orders) {
        return pendingOrdersToString(pendingOrders[fromType][toType]);
    }

    // getAllOrders return a list of all orders an address did from fromType and toType
    function getAllOrders(string _address, string fromType, string toType) public view returns (string orders) {
        return ordersToString(allOrders[_address][fromType][toType]);
    }

    // getOrderByTxId query idx from indexes by txid and then use IndexInfo to query to allOrders
    function getOrderByTxId(string txId) internal view returns (Order) {
        if (!addedTxs[txId]) return;
        IndexInfo storage indexInfo = indexes[txId];
        Order storage order = allOrders[indexInfo._address][indexInfo.fromType][indexInfo.toType][indexInfo.index];
        return order;
    }

    // getOrderByTxIdPublic get order by txId and return to string. This function can be used by DApp
    function getOrderByTxIdPublic(string txId) public view returns (string strOrder) {
        Order memory order = getOrderByTxId(txId);
        if (!order.existed) return "";
        return orderToString(order);
    }

    // updateOrder updates order into allOrders by query indexInfo by txId
    function updateOrder(Order order) internal {
        IndexInfo storage indexInfo = indexes[order.txId];
        allOrders[indexInfo._address][indexInfo.fromType][indexInfo.toType][indexInfo.index] = order;
    }

    // updateKardiaTx updates kardiaTxId to current order by its txId
    function updateKardiaTx(string txId, string kardiaTxId) public onlyadmin {
        if (!hasTxId(txId)) return;
        Order memory order = getOrderByTxId(txId);
        order.kardiaTxId = kardiaTxId;
        updateOrder(order);
    }

    // updateTargetTx updates targetTxId to current order by its txId
    function updateTargetTx(string txId, string targetTxId) public onlyadmin {
        if (!hasTxId(txId)) return;
        Order memory order = getOrderByTxId(txId);
        order.targetTxId = concatStr(order.targetTxId, targetTxId, ",");
        updateOrder(order);
    }

    /*
        addOrder adds new order into pendingOrders and allOrders
        Note: before addOrder, backend should convert amount by rate.
    */
    function addOrder(string fromType, string toType, string fromAddress, string receiver, string txid, uint256 amount, uint256 timestamp) public onlyadmin {
        Order memory order = Order(fromType, toType, fromAddress, receiver, txid, "", "", amount, amount, PENDING, timestamp, true);

        // if txid has already existed do nothing.
        if (addedTxs[txid]) {
            return;
        }

        // add order to pendingOrders
        pendingOrders[fromType][toType].push(order);

        // Get length of orders
        uint len = allOrders[fromAddress][fromType][toType].length;

        // add to allOrders
        allOrders[fromAddress][fromType][toType].push(order);

        // Create IndexInfo and add to indexes by txid with len as index
        IndexInfo memory index;
        index = IndexInfo(fromAddress, fromType, toType, len);

        indexes[txid] = index;
        addedTxs[txid] = true;

        doMatchingOrder(order);
    }

    // Update pendingOrders
    function updatePendingOrder(Order order, uint _type) internal {
        Order[] storage orders = pendingOrders[order.fromType][order.toType];
        bytes32 txId = keccak256(abi.encodePacked(order.txId));
        for (uint i = 0; i < orders.length; i++) {
            if (txId == keccak256(abi.encodePacked(orders[i].txId))) {
                if (_type == UPDATE) {
                    pendingOrders[order.fromType][order.toType][i] = order;
                }
                // if found break after processed
                break;
            }
        }
    }

    function updateMatchingResult(string txid, string toType, string _address, uint256 amount, string _tx) public onlyadmin {
        string[] memory result = matchingResult[txid];
        if (result.length == 0) {
            result = new string[](4);
            result[0] = toType;
            result[1] = _address;
            result[2] = uint2str(amount);
            result[3] = _tx;
        } else {
            result[0] = concatStr(result[0], toType, ";");
            result[1] = concatStr(result[1], _address, ";");
            result[2] = concatStr(result[2], uint2str(amount), ";");
            result[3] = concatStr(result[3], _tx, ";");
        }
        matchingResult[txid] = result;
    }

    /*
        Start doing matching from a specific order. The flow is:
        1. Get orders from pendingOrders by using toType as fromType, fromType as toType
        2. Update orderly amount until amount = 0 or reach the length of pendingOrders list
        3. If a pendingOrder's availableAmount = 0 then:
        - remove it from pendingOrder,
        - mark it to done (1) in allOrders,
        - add it to release list.
        4. Update results (order or pendingOrders) to allOrders.
        5. Append results to _toTypes, releasedAddresses, amount, txs and return.
        Note: before releasing, backend should get amount/rate to get correct amount.
    */
    function doMatchingOrder(Order order) internal {

        uint256 releasedAmount = order.availableAmount;
        // get orders from pendingOrders
        Order[] storage pendingOrdersList = pendingOrders[order.toType][order.fromType];
        if (pendingOrdersList.length >= 0) {
            for (uint i=0; i < pendingOrdersList.length; i++) {

                // break if order.availableAmount = 0
                if (releasedAmount == 0) break;

                // continue loop if availableAmount = 0
                if (pendingOrdersList[i].availableAmount == 0) continue;

                // orderly match pendingOrder with availableAmount until availableAmount is 0
                if (pendingOrdersList[i].availableAmount >= releasedAmount) {
                    // append pendingOrder info to _toTypes, releasedAddresses, amounts and txs
                    updateMatchingResult(order.txId, pendingOrdersList[i].toType, pendingOrdersList[i].receiver, releasedAmount, pendingOrdersList[i].txId);
                    pendingOrdersList[i].availableAmount = pendingOrdersList[i].availableAmount - releasedAmount;

                    // update pendingOrder if its availableAmount = 0 and remove it from pendingOrders
                    if (pendingOrdersList[i].availableAmount == 0) {
                        pendingOrdersList[i].done = DONE;
                    }

                    // update result to pendingOrders and allOrders
                    pendingOrders[order.toType][order.fromType][i] = pendingOrdersList[i];
                    updateOrder(pendingOrdersList[i]);
                    releasedAmount = 0;
                    break;
                } else {
                    releasedAmount = releasedAmount - pendingOrdersList[i].availableAmount;
                    pendingOrdersList[i].availableAmount = 0;
                    pendingOrdersList[i].done = DONE;
                    pendingOrders[order.toType][order.fromType][i] = pendingOrdersList[i];
                    // append pendingOrder info to _toTypes, releasedAddresses, amounts and txs
                    updateMatchingResult(order.txId, pendingOrdersList[i].toType, pendingOrdersList[i].receiver, pendingOrdersList[i].amount, pendingOrdersList[i].txId);

                    // update pendingOrder to allOrders
                    updateOrder(pendingOrdersList[i]);
                }
            }

            if (order.availableAmount - releasedAmount > 0 || order.availableAmount - releasedAmount == order.availableAmount) {
                // append order info to _toTypes, releaseAddresses, amounts and txs
                uint256 amount = order.availableAmount - releasedAmount;
                if (amount == 0) {
                    order.done = DONE;
                    order.availableAmount = 0;
                    updatePendingOrder(order, UPDATE);
                    updateMatchingResult(order.txId, order.toType, order.receiver, releasedAmount, order.txId);
                } else {
                    order.availableAmount = releasedAmount;
                    updatePendingOrder(order, UPDATE);
                    updateMatchingResult(order.txId, order.toType, order.receiver, amount, order.txId);
                }
                updateOrder(order);
            }
        }
    }

    function getMatchingResult(string txid) public view returns (string results) {
        string[] memory result = matchingResult[txid];
        if (result.length == 0) return "";
        return string(abi.encodePacked(result[0], "|", result[1], "|", result[2], "|", result[3]));
    }

    // THIS IS UTILS PART

    // concatStr concat 2 string
    function concatStr(string currentStr, string newStr, string splitter) internal pure returns (string) {
        if (keccak256(abi.encodePacked(currentStr)) == keccak256(abi.encodePacked(""))) {
            return newStr;
        }
        return string(abi.encodePacked(currentStr, splitter, newStr));
    }

    // convert a uint type into string
    function uint2str(uint i) internal pure returns (string) {
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
