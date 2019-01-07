pragma solidity ^0.4.24;
contract Permission {
    
    uint constant INITIAL_NODES_LENGTH = 13;

    struct NodeInfo {
        address nodeAddress;
        uint votingPower;
        uint nodeType;
        bool added;
        string listenAddress;
    }
    mapping(string => NodeInfo) nodes;

    function getInitialNodePubKeyList() internal pure returns (string[]) {
        string[] memory pubkey = new string[](INITIAL_NODES_LENGTH);
        pubkey[0] = "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860";
        pubkey[1] = "660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0";
        pubkey[2] = "2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da";
        pubkey[3] = "fc41a71d7a74d8665dbcc0f48c9a601e30b714ed50647669ef52c03f7123f2ae078dcaa36389e2636e1055f5f60fdf38d89a226ee84234f006b333cad2d2bcee";
        pubkey[4] = "ebf46faca754fc90716d665e6c6feb206ca437c9e5f16690e690513b302935053a9d722b88d2ab0b972f46448f3a53378bf5cfe01b8373af2e54197b17617e1c";
        pubkey[5] = "80c4fbf65122d817d3808afcb683fc66d9f9e19b476ea0ee3f757dca5cd18316ecb8999bfea4e9a5acc9968504cb919997a5c1ab623c5c533cb662291149b0a3";
        pubkey[6] = "5d7ed8131916b10ea545a559abe464307109a3d62ddbe19c368988bbdb1dd2330b6f3bbb479d0bdd79ef360d7d9175008d90f7d51122969210793e8a752cecd6";
        pubkey[7] = "7ecd4ea1bf4efa34dac41a16d7ccd14e23d3993dd3f0a54d722ee76d170718adba7f246c082bada922c875ffaaa4618e307b68e44c2847d4f4e3b767884c02b7";
        pubkey[8] = "4857f792ef779c511f6d7643f0991409f77e41124ced14385217535641315f5dc9927e7301ffd7afc7ae8025663e17f593306adf7d3ffac7c6aa625c250de0d5";
        pubkey[9] = "ad67c2502fc2723f2dcf25a140744382eb3e4e50d7e4dd910c423f7aa4fe0fbbcc2207d22ef6edf469dd6fbea73efa8d87b4b876a0d6e386c4e00b6a51c2a3f8";
        pubkey[10]= "43692b6f72370a32ab9fc5477ac3d560e45d29db0e6edc2e195cc74843b8fadf3259e5784c48bf4001a956b57c4e868f6d8d392083953009da114e382b67b326";
        pubkey[11]= "e376664d17fa55d1b94061cd486a6f8d642cf43ff63f2555744df668c85f86e781b026d6300564dde234e19dd69d2abedc18a53db0d6f1e4a4c1bf3b27f7d848";
        pubkey[12]= "2c29f2ce64dc90f1538bde02f666664a6c14832576086a019aa6672afb8e6299066b9c59c8c511e90c08b781e2051de6fefbde602dd745b5dc851972f3a34574";
        return pubkey;
    }

    function getInitialNodesIndexByPubKey(string _pubkey) internal pure returns (uint) {
        string[] memory pubkey = getInitialNodePubKeyList();
        for (uint i = 0; i < INITIAL_NODES_LENGTH; i ++) {
            if (keccak256(abi.encodePacked(_pubkey)) == keccak256(abi.encodePacked(pubkey[i]))) {
                return i;
            }
        }
        // return an index exceeding quantity of initial nodes
        return INITIAL_NODES_LENGTH;
    }

    // getInitialNodeAddressByIndex returns list of initial nodes' addresses
    function getInitialNodeAddresses() internal pure returns (address[]) {
        address[] memory listAddress = new address[](INITIAL_NODES_LENGTH);
        listAddress[0] = 0xc1fe56E3F58D3244F606306611a5d10c8333f1f6;
        listAddress[1] = 0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5;
        listAddress[2] = 0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd;
        listAddress[3] = 0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28;
        listAddress[4] = 0x94FD535AAB6C01302147Be7819D07817647f7B63;
        listAddress[5] = 0xa8073C95521a6Db54f4b5ca31a04773B093e9274;
        listAddress[6] = 0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547;
        listAddress[7] = 0xBA30505351c17F4c818d94a990eDeD95e166474b;
        listAddress[8] = 0x212a83C0D7Db5C526303f873D9CeaA32382b55D0;
        listAddress[9] = 0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4;
        listAddress[10]= 0x66BAB3F68Ff0822B7bA568a58A5CB619C4825Ce5;
        listAddress[11]= 0x88e1B4289b639C3b7b97899Be32627DCd3e81b7e;
        listAddress[12]= 0xCE61E95666737E46B2453717Fe1ba0d9A85B9d3E;
        return listAddress;
    }

    function getInitialNodeListenAddresses() internal pure returns (string[]) {
        string[] memory listAddress = new string[](INITIAL_NODES_LENGTH);
        listAddress[0] = "[::]:3000";
        listAddress[1] = "[::]:3001";
        listAddress[2] = "[::]:3002";
        listAddress[3] = "[::]:3003";
        listAddress[4] = "[::]:3004";
        listAddress[5] = "[::]:3005";
        listAddress[6] = "[::]:3006";
        listAddress[7] = "[::]:3007";
        listAddress[8] = "[::]:3008";
        listAddress[9] = "[::]:3009";
        listAddress[10]= "[::]:3010";
        listAddress[11]= "[::]:3011";
        listAddress[12]= "[::]:3012";
        return listAddress;
    }

    // getInitialNodeVotingPowerByIndex, always returns 100 as initial voting power
    function getInitialNodeVotingPower() internal pure returns (uint) {
        return 100;
    }

    // getInitialNodeType always return node 1 as initial node type
    function getInitialNodeType() internal pure returns (uint) {
        return 1;
    }

    modifier onlyOwner()
    {
        require(isOwner(msg.sender) == true);
        _;
    }

    // isValidInitialNode checks if given key with given nodeType exists in initial nodes list
    function isValidInitialNode(string _key, uint256 _type) internal pure returns (bool) {
        if (getInitialNodesIndexByPubKey(_key) < INITIAL_NODES_LENGTH && _type == 1) {
            return true;
        }
        return false;
    }

    function getInitialNodeInfo(string _key) internal pure returns (address _add, uint nodeType, uint votingPower, bool added, string listenAddress) {
        uint index = getInitialNodesIndexByPubKey(_key);
        if (index < INITIAL_NODES_LENGTH) {
            return (getInitialNodeAddresses()[index], 1, 100, true, getInitialNodeListenAddresses()[index]);
        }
        return (0x0, 0, 0, false, "");
    }

    // isInitialValidator checks if given key exists in initial nodes list and has votingPower
    function isInitialValidator(string _key) internal pure returns (bool) {
        uint index = getInitialNodesIndexByPubKey(_key);
        if (index < INITIAL_NODES_LENGTH) {
            return true;
        }
        return false;
    }

    // isOwner checks if address is in inital owners list
    function isOwner(address _addr) internal pure returns (bool) {
        address[] memory list = getInitialNodeAddresses();
        if (list.length == 0) {
            return false;
        }
        for (uint i = 0; i < list.length; i++) {
            if (list[i] == _addr) {
                return true;
            }
        }
        return false;
    }

    // getNodeInfo returns address, votingPower and nodeType of node specified by pubkey
    // return (0x0, 0, 0) if pubkey is not found
    function getNodeInfo(string _pubkey) public view returns (address addr, uint votingPower, uint nodeType, string listenAddress) {
        uint initialIndex = getInitialNodesIndexByPubKey(_pubkey);
        if (initialIndex < INITIAL_NODES_LENGTH) {
            return (getInitialNodeAddresses()[initialIndex], 100, 1, getInitialNodeListenAddresses()[initialIndex]);
        }
        if (nodes[_pubkey].added == false) {
            return (0x0, 0, 0, "");
        }
        return (nodes[_pubkey].nodeAddress, nodes[_pubkey].votingPower, nodes[_pubkey].nodeType, nodes[_pubkey].listenAddress);
    }

    // addNode add a node into lists of node including pubkey, wallet address, nodeType and votingPower
    // return 1 on success, raise error if called from non-owner accounts
    function addNode(string _pubkey, address _addr, uint _nodeType, uint _votingPower, string _listenAddress) public onlyOwner returns (uint)  {
        nodes[_pubkey] = NodeInfo(_addr, _votingPower, _nodeType, true, _listenAddress);
        return 1;
    }

    // isValidNode checks whether node specified by pubkey has specified nodeType or not
    // returns 1 if true, return 0 if false or pubkey is not found
    function isValidNode(string _pubkey, uint _nodeType) public view returns (uint) {
        if (isValidInitialNode(_pubkey, _nodeType)) {
            return 1;
        }
        if (nodes[_pubkey].added == false) {
            return 0;
        }
        if (nodes[_pubkey].nodeType == _nodeType) {
            return 1;
        }
        return 0;
    }

    // isValidator checks whether node specified by pubkey is a isValidator
    // returns 1 if true, return 0 if false or pubkey is not found
    function isValidator(string _pubkey) public view returns (uint) {
        if (isInitialValidator(_pubkey)) {
            return 1;
        }
        if (nodes[_pubkey].added == false) {
            return 0;
        }
        if (nodes[_pubkey].votingPower == 0) {
            return 0;
        }
        return 1;
    }

    // removeNode remove a node with a given node type,
        // return 1 if success, return 0 if false of pubkey is not found
        function removeNode(string _pubkey) public onlyOwner returns (uint) {
            // we don't allow removal of initial nodes for security reasons
            if (getInitialNodesIndexByPubKey(_pubkey) < INITIAL_NODES_LENGTH) {
                return 0;
            }
            // key has not been added , return false
            if (nodes[_pubkey].added == false) {
                return 0;
            }
            delete nodes[_pubkey];
            return 1;
        }

    // getInitialNodeByIndex returns publickey, addr, listenAddr, votingPower and nodeType of initial node specified by index
    // return ("", 0x0, "", 0, 0) if index > INITIAL_NODES_LENGTH
    function getInitialNodeByIndex(uint index) public pure returns (string publickey, address addr, string listenAddr, uint votingPower, uint nodeType) {
        if (index >= INITIAL_NODES_LENGTH) {
            return ("", 0x0, "", 0, 0);
        }
        string[] memory pubkey = getInitialNodePubKeyList();
        return (pubkey[index], getInitialNodeAddresses()[index],  getInitialNodeListenAddresses()[index], getInitialNodeVotingPower(), getInitialNodeType());
    }
}