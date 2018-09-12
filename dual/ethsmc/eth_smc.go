package ethsmc

import (
	"encoding/hex"
	"fmt"
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/kardiachain/go-kardia/abi"
	"math/big"
	"strings"
)

// Address of the deployed contract on Rinkeby.
var EthContractAddress = "0xffd56f189a9e67aeee5220f3b66146c63d7fcb10"

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
                "name": "matchedId",
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
                "name": "infoId",
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
        "constant": true,
        "inputs": [],
        "name": "id",
        "outputs": [
            {
                "name": "",
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
        "payable": true,
        "stateMutability": "payable",
        "type": "fallback"
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

var (
	EthAccountReleaseDestination = "0x3688Aad7025F17f64eAF8A8De250D3E67f60D9f7"
	EthAccountSign               = "0xff6781f2cc6f9b6b4a68a0afc3aae89133bbb236"
	EthAccountSignAddr           = "457D86F3AFAA8159D7C8356BF3F195CF7AED35AF84C7DC40C4D9AA27846ED9DC"
)

type EthSmc struct {
	ethABI ethabi.ABI
	kABI   abi.ABI
}

func NewEthSmc() *EthSmc {
	smc := &EthSmc{}
	eABI, err := ethabi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		panic(fmt.Sprintf("Geth ABI library fail to read abi def: %v", err))
	}
	smc.ethABI = eABI

	kABI, err := abi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		panic(fmt.Sprintf("Kardia ABI library fail to read abi def: %v", err))
	}
	smc.kABI = kABI

	return smc
}

func (e *EthSmc) etherABI() ethabi.ABI {
	return e.ethABI
}

func (e *EthSmc) InputMethodName(input []byte) (string, error) {
	method, err := e.ethABI.MethodById(input[0:4])
	if err != nil {
		return "", err
	}
	return method.Name, nil
}

func (e *EthSmc) UnpackDepositInput(input []byte) (string, error) {
	var param string

	if err := e.kABI.UnpackInput(&param, "deposit", input[4:]); err != nil {
		return "", err
	}
	return param, nil
}

func (e *EthSmc) packReleaseInput(amount *big.Int) []byte {
	releaseAddr := common.HexToAddress(EthAccountReleaseDestination)
	input, err := e.ethABI.Pack("release", releaseAddr, amount)
	if err != nil {
		panic(err)
	}

	return input
}

func (e *EthSmc) CreateEthReleaseTx(amount *big.Int, nonce uint64) *types.Transaction {
	contractAddr := common.HexToAddress(EthContractAddress)
	keyBytes, err := hex.DecodeString(EthAccountSignAddr)
	if err != nil {
		panic(err)
	}
	key := crypto.ToECDSAUnsafe(keyBytes)
	data := e.packReleaseInput(amount)
	gasLimit := uint64(40000)
	gasPrice := big.NewInt(5000000000) // 5gwei
	tx, err := types.SignTx(
		types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data),
		types.HomesteadSigner{},
		key)
	if err != nil {
		panic(err)
	}

	return tx
}
