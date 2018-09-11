package ethsmc

// Address of the deployed contract on Rinkeby.
var EthContractAddress = "0xa131f8ef263527892d1e2971efc8ced85537b068"

// ABI of the deployed Eth contract.
var EthExchangeAbi = `[
    {
        "constant": false,
        "inputs": [
            {
                "name": "ethReceiver",
                "type": "address"
            },
            {
                "name": "ethAmount",
                "type": "uint256"
            }
        ],
        "name": "release",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "id",
                "type": "uint256"
            },
            {
                "name": "matchedValue",
                "type": "uint256"
            }
        ],
        "name": "updateOnMatch",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "neoAddress",
                "type": "string"
            }
        ],
        "name": "deposit",
        "outputs": [],
        "payable": true,
        "stateMutability": "payable",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [
            {
                "name": "id",
                "type": "uint256"
            }
        ],
        "name": "getInfoById",
        "outputs": [
            {
                "name": "sender",
                "type": "address"
            },
            {
                "name": "receiver",
                "type": "string"
            },
            {
                "name": "amount",
                "type": "uint256"
            },
            {
                "name": "matchedValue",
                "type": "uint256"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "constructor"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": false,
                "name": "id",
                "type": "uint256"
            },
            {
                "indexed": false,
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": false,
                "name": "receiver",
                "type": "string"
            },
            {
                "indexed": false,
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "onDeposit",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": false,
                "name": "receiver",
                "type": "address"
            },
            {
                "indexed": false,
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "onRelease",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": false,
                "name": "id",
                "type": "uint256"
            },
            {
                "indexed": false,
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": false,
                "name": "matchedValue",
                "type": "uint256"
            }
        ],
        "name": "onMatch",
        "type": "event"
    }
]`
