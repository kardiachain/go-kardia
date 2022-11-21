/*
 *  Copyright 2019 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package tests

import (
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/kai/accounts/abi"
	"github.com/kardiachain/go-kardia/ksml"
	message "github.com/kardiachain/go-kardia/ksml/proto"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/stretchr/testify/require"
)

var (
	sampleCode1       = common.Hex2Bytes("608060405260043610603f576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680633bc5de30146044575b600080fd5b348015604f57600080fd5b5060566072565b604051808260ff1660ff16815260200191505060405180910390f35b6000809050905600a165627a7a72305820d1a94e87e80f645f0f381c5a92d9c5212efe1343f8f1c027eb119870576313440029")
	sampleDefinition1 = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getData",
		"outputs": [
			{
				"name": "data",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`
	sampleCode2       = common.Hex2Bytes("608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806308038b7c1461005c5780633a2350f11461008d57806397191cb2146100be575b600080fd5b34801561006857600080fd5b50610071610115565b604051808260ff1660ff16815260200191505060405180910390f35b34801561009957600080fd5b506100a261011e565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100ca57600080fd5b506100f9600480360381019080803560ff169060200190929190803560ff169060200190929190505050610127565b604051808260ff1660ff16815260200191505060405180910390f35b60006001905090565b60006002905090565b60008183019050929150505600a165627a7a72305820863a6a9ff2789069f376d82512183111067f27f38bb9e91b28ef34a176cee2530029")
	sampleDefinition2 = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getV1",
		"outputs": [
			{
				"name": "v1",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getV2",
		"outputs": [
			{
				"name": "v2",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "v1",
				"type": "uint8"
			},
			{
				"name": "v2",
				"type": "uint8"
			}
		],
		"name": "Calculate",
		"outputs": [
			{
				"name": "data",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	sampleCode3       = common.Hex2Bytes("6080604052600436106100d0576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630e4cdd15146100d55780633a6b097b146101995780633f65269d1461027257806346e468d41461039e5780634fbf2e76146105d15780636168d817146106a7578063781c6dbe146106ee57806382ca3ca4146107d0578063886d9ea8146108775780639c981fcb14610913578063ae22c57d146109f5578063ba9985dc14610a78578063d62f41c314610c51578063f1b3e2fc14610daa575b600080fd5b3480156100e157600080fd5b5061017b600480360381019080803566ffffffffffffff19169060200190929190803565ffffffffffff19169060200190929190803564ffffffffff19169060200190929190803563ffffffff19169060200190929190803562ffffff19169060200190929190803561ffff19169060200190929190803560ff191690602001909291908035600019169060200190929190505050610f35565b60405180826000191660001916815260200191505060405180910390f35b3480156101a557600080fd5b506102566004803603810190808035600c0b90602001909291908035600d0b90602001909291908035600e0b90602001909291908035600f0b9060200190929190803560100b9060200190929190803560110b9060200190929190803560120b9060200190929190803560130b9060200190929190803560140b9060200190929190803560150b9060200190929190803560160b9060200190929190803560170b9060200190929190505050610f46565b604051808260170b60170b815260200191505060405180910390f35b34801561027e57600080fd5b5061036c60048036038101908080359060200190929190803560ff169060200190929190803561ffff169060200190929190803563ffffffff169060200190929190803564ffffffffff169060200190929190803565ffffffffffff169060200190929190803566ffffffffffffff169060200190929190803567ffffffffffffffff169060200190929190803568ffffffffffffffffff169060200190929190803569ffffffffffffffffffff16906020019092919080356affffffffffffffffffffff16906020019092919080356bffffffffffffffffffffffff169060200190929190505050610f5b565b60405180826bffffffffffffffffffffffff166bffffffffffffffffffffffff16815260200191505060405180910390f35b3480156103aa57600080fd5b5061058d60048036038101908080357effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916906020019092919080357dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916906020019092919080357cffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916906020019092919080357bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916906020019092919080357affffffffffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803579ffffffffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803578ffffffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803577ffffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803576ffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803575ffffffffffffffffffffffffffffffffffffffffffff19169060200190929190803574ffffffffffffffffffffffffffffffffffffffffff19169060200190929190803573ffffffffffffffffffffffffffffffffffffffff19169060200190929190505050610f70565b604051808273ffffffffffffffffffffffffffffffffffffffff191673ffffffffffffffffffffffffffffffffffffffff1916815260200191505060405180910390f35b3480156105dd57600080fd5b5061068b60048036038101908080359060200190929190803560000b9060200190929190803560010b9060200190929190803560030b9060200190929190803560040b9060200190929190803560050b9060200190929190803560060b9060200190929190803560070b9060200190929190803560080b9060200190929190803560090b90602001909291908035600a0b90602001909291908035600b0b9060200190929190505050610f85565b6040518082600b0b600b0b815260200191505060405180910390f35b3480156106b357600080fd5b506106d4600480360381019080803515159060200190929190505050610f9a565b604051808215151515815260200191505060405180910390f35b3480156106fa57600080fd5b50610755600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610fa4565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561079557808201518184015260208101905061077a565b50505050905090810190601f1680156107c25780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156107dc57600080fd5b5061081d60048036038101908080357effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff19169060200190929190505050610fae565b60405180827effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff19167effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916815260200191505060405180910390f35b34801561088357600080fd5b506108fd600480360381019080803560180b9060200190929190803560190b90602001909291908035601a0b90602001909291908035601b0b90602001909291908035601c0b90602001909291908035601d0b90602001909291908035601e0b906020019092919080359060200190929190505050610fb8565b6040518082815260200191505060405180910390f35b34801561091f57600080fd5b5061097a600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610fc9565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156109ba57808201518184015260208101905061099f565b50505050905090810190601f1680156109e75780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b348015610a0157600080fd5b50610a36600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610fd3565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b348015610a8457600080fd5b50610c0760048036038101908080356cffffffffffffffffffffffffff16906020019092919080356dffffffffffffffffffffffffffff16906020019092919080356effffffffffffffffffffffffffffff16906020019092919080356fffffffffffffffffffffffffffffffff169060200190929190803570ffffffffffffffffffffffffffffffffff169060200190929190803571ffffffffffffffffffffffffffffffffffff169060200190929190803572ffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803574ffffffffffffffffffffffffffffffffffffffffff169060200190929190803575ffffffffffffffffffffffffffffffffffffffffffff169060200190929190803576ffffffffffffffffffffffffffffffffffffffffffffff169060200190929190803577ffffffffffffffffffffffffffffffffffffffffffffffff169060200190929190505050610fdd565b604051808277ffffffffffffffffffffffffffffffffffffffffffffffff1677ffffffffffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b348015610c5d57600080fd5b50610d94600480360381019080803578ffffffffffffffffffffffffffffffffffffffffffffffffff169060200190929190803579ffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080357affffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080357bffffffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080357cffffffffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080357dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080357effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610ff2565b6040518082815260200191505060405180910390f35b348015610db657600080fd5b50610f09600480360381019080803572ffffffffffffffffffffffffffffffffffffff19169060200190929190803571ffffffffffffffffffffffffffffffffffff19169060200190929190803570ffffffffffffffffffffffffffffffffff1916906020019092919080356fffffffffffffffffffffffffffffffff1916906020019092919080356effffffffffffffffffffffffffffff1916906020019092919080356dffffffffffffffffffffffffffff1916906020019092919080356cffffffffffffffffffffffffff1916906020019092919080356bffffffffffffffffffffffff1916906020019092919080356affffffffffffffffffffff19169060200190929190803569ffffffffffffffffffff19169060200190929190803568ffffffffffffffffff19169060200190929190803567ffffffffffffffff19169060200190929190505050611003565b604051808267ffffffffffffffff191667ffffffffffffffff1916815260200191505060405180910390f35b600081905098975050505050505050565b60008190509c9b505050505050505050505050565b60008190509c9b505050505050505050505050565b60008190509c9b505050505050505050505050565b60008190509c9b505050505050505050505050565b6000819050919050565b6060819050919050565b6000819050919050565b600081905098975050505050505050565b6060819050919050565b6000819050919050565b60008190509c9b505050505050505050505050565b600081905098975050505050505050565b60008190509c9b5050505050505050505050505600a165627a7a72305820cde91fa34b2c99e6b6f250d31c9d4a65b2c36674687946f88ed021da600b5b930029")
	sampleDefinition3 = `[
	{
		"constant": true,
		"inputs": [
			{
				"name": "t25",
				"type": "bytes25"
			},
			{
				"name": "t26",
				"type": "bytes26"
			},
			{
				"name": "t27",
				"type": "bytes27"
			},
			{
				"name": "t28",
				"type": "bytes28"
			},
			{
				"name": "t29",
				"type": "bytes29"
			},
			{
				"name": "t30",
				"type": "bytes30"
			},
			{
				"name": "t31",
				"type": "bytes31"
			},
			{
				"name": "t32",
				"type": "bytes32"
			}
		],
		"name": "getLast8Bytes",
		"outputs": [
			{
				"name": "result",
				"type": "bytes32"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "int104"
			},
			{
				"name": "t2",
				"type": "int112"
			},
			{
				"name": "t3",
				"type": "int120"
			},
			{
				"name": "t4",
				"type": "int128"
			},
			{
				"name": "t5",
				"type": "int136"
			},
			{
				"name": "t6",
				"type": "int144"
			},
			{
				"name": "t7",
				"type": "int152"
			},
			{
				"name": "t8",
				"type": "int160"
			},
			{
				"name": "t9",
				"type": "int168"
			},
			{
				"name": "t10",
				"type": "int176"
			},
			{
				"name": "t11",
				"type": "int184"
			},
			{
				"name": "t12",
				"type": "int192"
			}
		],
		"name": "getNext12Int",
		"outputs": [
			{
				"name": "result",
				"type": "int192"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "uint256"
			},
			{
				"name": "t2",
				"type": "uint8"
			},
			{
				"name": "t3",
				"type": "uint16"
			},
			{
				"name": "t4",
				"type": "uint32"
			},
			{
				"name": "t5",
				"type": "uint40"
			},
			{
				"name": "t6",
				"type": "uint48"
			},
			{
				"name": "t7",
				"type": "uint56"
			},
			{
				"name": "t8",
				"type": "uint64"
			},
			{
				"name": "t9",
				"type": "uint72"
			},
			{
				"name": "t10",
				"type": "uint80"
			},
			{
				"name": "t11",
				"type": "uint88"
			},
			{
				"name": "t12",
				"type": "uint96"
			}
		],
		"name": "getFirst12UInt",
		"outputs": [
			{
				"name": "result",
				"type": "uint96"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "bytes1"
			},
			{
				"name": "t2",
				"type": "bytes2"
			},
			{
				"name": "t3",
				"type": "bytes3"
			},
			{
				"name": "t4",
				"type": "bytes4"
			},
			{
				"name": "t5",
				"type": "bytes5"
			},
			{
				"name": "t6",
				"type": "bytes6"
			},
			{
				"name": "t7",
				"type": "bytes7"
			},
			{
				"name": "t8",
				"type": "bytes8"
			},
			{
				"name": "t9",
				"type": "bytes9"
			},
			{
				"name": "t10",
				"type": "bytes10"
			},
			{
				"name": "t11",
				"type": "bytes11"
			},
			{
				"name": "t12",
				"type": "bytes12"
			}
		],
		"name": "getFirst12Bytes",
		"outputs": [
			{
				"name": "result",
				"type": "bytes12"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "int256"
			},
			{
				"name": "t2",
				"type": "int8"
			},
			{
				"name": "t3",
				"type": "int16"
			},
			{
				"name": "t4",
				"type": "int32"
			},
			{
				"name": "t5",
				"type": "int40"
			},
			{
				"name": "t6",
				"type": "int48"
			},
			{
				"name": "t7",
				"type": "int56"
			},
			{
				"name": "t8",
				"type": "int64"
			},
			{
				"name": "t9",
				"type": "int72"
			},
			{
				"name": "t10",
				"type": "int80"
			},
			{
				"name": "t11",
				"type": "int88"
			},
			{
				"name": "t12",
				"type": "int96"
			}
		],
		"name": "getFirst12Int",
		"outputs": [
			{
				"name": "result",
				"type": "int96"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "bool"
			}
		],
		"name": "getBool",
		"outputs": [
			{
				"name": "result",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "bytes"
			}
		],
		"name": "getBytes",
		"outputs": [
			{
				"name": "result",
				"type": "bytes"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "bytes1"
			}
		],
		"name": "getByte",
		"outputs": [
			{
				"name": "result",
				"type": "bytes1"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "int200"
			},
			{
				"name": "t2",
				"type": "int208"
			},
			{
				"name": "t3",
				"type": "int216"
			},
			{
				"name": "t4",
				"type": "int224"
			},
			{
				"name": "t5",
				"type": "int232"
			},
			{
				"name": "t6",
				"type": "int240"
			},
			{
				"name": "t7",
				"type": "int248"
			},
			{
				"name": "t8",
				"type": "int256"
			}
		],
		"name": "getLast8Int",
		"outputs": [
			{
				"name": "result",
				"type": "int256"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "string"
			}
		],
		"name": "getString",
		"outputs": [
			{
				"name": "result",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "address"
			}
		],
		"name": "getAddress",
		"outputs": [
			{
				"name": "result",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "uint104"
			},
			{
				"name": "t2",
				"type": "uint112"
			},
			{
				"name": "t3",
				"type": "uint120"
			},
			{
				"name": "t4",
				"type": "uint128"
			},
			{
				"name": "t5",
				"type": "uint136"
			},
			{
				"name": "t6",
				"type": "uint144"
			},
			{
				"name": "t7",
				"type": "uint152"
			},
			{
				"name": "t8",
				"type": "uint160"
			},
			{
				"name": "t9",
				"type": "uint168"
			},
			{
				"name": "t10",
				"type": "uint176"
			},
			{
				"name": "t11",
				"type": "uint184"
			},
			{
				"name": "t12",
				"type": "uint192"
			}
		],
		"name": "getNext12UInt",
		"outputs": [
			{
				"name": "result",
				"type": "uint192"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t1",
				"type": "uint200"
			},
			{
				"name": "t2",
				"type": "uint208"
			},
			{
				"name": "t3",
				"type": "uint216"
			},
			{
				"name": "t4",
				"type": "uint224"
			},
			{
				"name": "t5",
				"type": "uint232"
			},
			{
				"name": "t6",
				"type": "uint240"
			},
			{
				"name": "t7",
				"type": "uint248"
			},
			{
				"name": "t8",
				"type": "uint256"
			}
		],
		"name": "getLast8UInt",
		"outputs": [
			{
				"name": "result",
				"type": "uint256"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "t13",
				"type": "bytes13"
			},
			{
				"name": "t14",
				"type": "bytes14"
			},
			{
				"name": "t15",
				"type": "bytes15"
			},
			{
				"name": "t16",
				"type": "bytes16"
			},
			{
				"name": "t17",
				"type": "bytes17"
			},
			{
				"name": "t18",
				"type": "bytes18"
			},
			{
				"name": "t19",
				"type": "bytes19"
			},
			{
				"name": "t20",
				"type": "bytes20"
			},
			{
				"name": "t21",
				"type": "bytes21"
			},
			{
				"name": "t22",
				"type": "bytes22"
			},
			{
				"name": "t23",
				"type": "bytes23"
			},
			{
				"name": "t24",
				"type": "bytes24"
			}
		],
		"name": "getNext12Bytes",
		"outputs": [
			{
				"name": "result",
				"type": "bytes24"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	sampleCode4       = common.Hex2Bytes("608060405260043610610062576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631f7c38d8146100675780632bfc4c69146100985780633b693e301461012857806366ce82cb1461016f575b600080fd5b34801561007357600080fd5b5061007c6101ad565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100a457600080fd5b506100ad6101b6565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156100ed5780820151818401526020810190506100d2565b50505050905090810190601f16801561011a5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561013457600080fd5b506101556004803603810190808035151590602001909291905050506101f3565b604051808215151515815260200191505060405180910390f35b34801561017b57600080fd5b506101846101fe565b604051808360ff1660ff1681526020018260ff1660ff1681526020019250505060405180910390f35b60006001905090565b60606040805190810160405280600581526020017f68656c6c6f000000000000000000000000000000000000000000000000000000815250905090565b600081159050919050565b600080600260038191508090509150915090915600a165627a7a72305820e00df01b154b34f0906610ccc0b2875c26c78f2845d85966a64ce7c67e015c250029")
	sampleDefinition4 = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getSingleUintValue",
		"outputs": [
			{
				"name": "single",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getStringValue",
		"outputs": [
			{
				"name": "single",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "val",
				"type": "bool"
			}
		],
		"name": "getBoolValue",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "get2UintValue",
		"outputs": [
			{
				"name": "first",
				"type": "uint8"
			},
			{
				"name": "second",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	sampleCode5       = common.Hex2Bytes("608060405260043610603f576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680638f755479146044575b600080fd5b348015604f57600080fd5b50606f600480360381019080803560ff1690602001909291905050506071565b005b806000806101000a81548160ff021916908360ff160217905550505600a165627a7a72305820c25cbeac5f2b9728ac00bf7844ddc3122d94a4acfa1b1bcecef1f69df50e17f70029")
	sampleDefinition5 = `[
	{
		"constant": false,
		"inputs": [
			{
				"name": "d",
				"type": "uint8"
			}
		],
		"name": "setData",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
	sampleCode6       = common.Hex2Bytes("608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680633bc5de3014610046575b600080fd5b34801561005257600080fd5b5061005b6100d6565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561009b578082015181840152602081019050610080565b50505050905090810190601f1680156100c85780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b60606040805190810160405280600581526020017f68656c6c6f0000000000000000000000000000000000000000000000000000008152509050905600a165627a7a72305820a7650f38e073e17ffa40d3832012f03e6cbfd523c624bd33f8cede24b4b3a7a40029")
	sampleDefinition6 = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getData",
		"outputs": [
			{
				"name": "data",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
)

func TestApplyBuiltInFunc(t *testing.T) {
	out, err := ksml.BuiltInFuncMap["ping"](nil, nil)
	require.NoError(t, err)
	require.Equal(t, out, []interface{}{"pong"})
}

func TestGetDataFromSmc(t *testing.T) {
	patterns := make([]string, 0)
	parser, err := setup(sampleCode1, sampleDefinition1, patterns, nil)
	require.NoError(t, err)
	method := "getData"
	params := []interface{}{method}
	val, err := ksml.GetDataFromSmc(parser, params...)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint8(0)}, val)
}

func TestAddVar(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2,
		[]string{
			"${fn:var(testVar, bigInt, 1)}",
		},
		&message.EventMessage{
			Params: []string{"1", "2"},
		},
	)
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)

	expected := map[string]interface{}{
		"testVar": big.NewInt(int64(1)),
	}
	require.Equal(t, expected, parser.UserDefinedVariables)
}

func TestReadVarInPattern(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2,
		[]string{
			"${fn:var(testVar, uint64, 1)}",
			"${testVar==uint(1)}",
		},
		&message.EventMessage{
			Params: []string{"1", "2"},
		},
	)
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)

	expected := []interface{}{true}
	require.Equal(t, expected, parser.GetGlobalParams())
}

func TestReadVarInPattern_withList(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2,
		[]string{
			"${uint(1) + uint(2)}",
			"${params[0]==uint(3)}",
			"${fn:var(testVar, list, params)}",
			"${size(testVar)==int(2)}",
		},
		&message.EventMessage{
			Params: []string{"1", "2"},
		},
	)
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)

	expected := []interface{}{uint64(3), true, true}
	require.Equal(t, expected, parser.GetGlobalParams())
}

func TestGetDataFromSmc_WithCELParams(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{"message.params[0]", "message.params[1]"}, &message.EventMessage{
		Params: []string{"1", "2"},
	})
	require.NoError(t, err)
	method := "Calculate"
	params := []interface{}{method, "message.params[0]", "message.params[1]"}
	val, err := ksml.GetDataFromSmc(parser, params...)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint8(3)}, val)
}

func TestConvertParams_getFirst12Int(t *testing.T) {
	parser, err := setup(
		sampleCode1,
		sampleDefinition1,
		[]string{
			"message.params[0]",
			"message.params[1]",
			"message.params[2]",
			"message.params[3]",
			"message.params[4]",
			"message.params[5]",
			"message.params[6]",
			"message.params[7]",
			"message.params[8]",
			"message.params[9]",
			"message.params[10]",
			"message.params[11]",
		},
		&message.EventMessage{
			Params: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
		},
	)
	require.NoError(t, err)
	kAbi, err := abi.JSON(strings.NewReader(sampleDefinition3))
	require.NoError(t, err)
	method := "getFirst12Int"
	args := kAbi.Methods[method].Inputs
	results, err := ksml.ConvertParams(parser, args, parser.GlobalPatterns)
	require.NoError(t, err)

	expectedResult := []interface{}{
		big.NewInt(1),
		int8(2),
		int16(3),
		int32(4),
		big.NewInt(5),
		big.NewInt(6),
		big.NewInt(7),
		int64(8),
		big.NewInt(9),
		big.NewInt(10),
		big.NewInt(11),
		big.NewInt(12),
	}
	require.Equal(t, expectedResult, results)
}

func TestConvertParams_getBool(t *testing.T) {
	parser := &ksml.Parser{
		GlobalMessage: &message.EventMessage{
			Params: []string{"true"},
		},
		GlobalPatterns: []string{
			"message.params[0]",
		},
		GlobalParams: []interface{}{0},
	}
	kAbi, err := abi.JSON(strings.NewReader(sampleDefinition3))
	require.NoError(t, err)
	method := "getBool"
	args := kAbi.Methods[method].Inputs
	results, err := ksml.ConvertParams(parser, args, parser.GlobalPatterns)
	require.NoError(t, err)

	expectedResult := []interface{}{true}
	require.Equal(t, expectedResult, results)
}

func TestConvertParams_getString(t *testing.T) {
	parser := &ksml.Parser{
		GlobalMessage: &message.EventMessage{
			Params: []string{"hello"},
		},
		GlobalPatterns: []string{
			"message.params[0]",
		},
		GlobalParams: []interface{}{0},
	}
	kAbi, err := abi.JSON(strings.NewReader(sampleDefinition3))
	require.NoError(t, err)
	method := "getString"
	args := kAbi.Methods[method].Inputs
	results, err := ksml.ConvertParams(parser, args, parser.GlobalPatterns)
	require.NoError(t, err)

	expectedResult := []interface{}{"hello"}
	require.Equal(t, expectedResult, results)
}

func TestConvertParams_getAddress(t *testing.T) {
	parser := &ksml.Parser{
		GlobalMessage: &message.EventMessage{
			Params: []string{"0x0A"},
		},
		GlobalPatterns: []string{
			"message.params[0]",
		},
		GlobalParams: []interface{}{0},
	}
	kAbi, err := abi.JSON(strings.NewReader(sampleDefinition3))
	require.NoError(t, err)
	method := "getAddress"
	args := kAbi.Methods[method].Inputs
	results, err := ksml.ConvertParams(parser, args, parser.GlobalPatterns)
	require.NoError(t, err)

	expectedResult := []interface{}{common.HexToAddress("0x0A")}
	require.Equal(t, expectedResult, results)
}

func TestExecuteIfElse(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${message.params[0]}",
		"${fn:if(name1,uint(message.params[1])==uint(3))}",
		"${uint(message.params[0])+uint(message.params[1])}",
		"${fn:elif(name1,uint(message.params[1])==uint(2))}",
		"${uint(message.params[2])==uint(2)}",
		"${fn:else(name1)}",
		"${message.params[3]}",
		"${fn:endif(name1)}",
		"${uint(message.params[3])+uint(1)}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedParams := []interface{}{"1", false, uint64(5)}
	require.Equal(t, expectedParams, parser.GetGlobalParams())
}

func TestExecuteIfElse_callElse(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${message.params[0]}",
		"${fn:if(name1,uint(message.params[1])==uint(3))}",
		"${uint(message.params[0])+uint(message.params[1])}",
		"${fn:elif(name1,uint(message.params[1])==uint(3))}",
		"${uint(message.params[2])==uint(2)}",
		"${fn:else(name1)}",
		"${message.params[3]}",
		"${fn:endif(name1)}",
		"${uint(message.params[3])+uint(1)}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedParams := []interface{}{"1", "4", uint64(5)}
	require.Equal(t, expectedParams, parser.GetGlobalParams())
}

//func TestExecuteIfElse_nestedIf(t *testing.T) {
//	parser, err := setup(sampleCode2, sampleDefinition2, []string{
//		"${message.params[0]}",
//		"${fn:if(name1,uint(message.params[1])==uint(3))}",
//		"${uint(message.params[0])+uint(message.params[1])}",
//		"${fn:elif(name1,uint(message.params[1])==uint(3))}",
//		"${uint(message.params[2])==uint(2)}",
//		"${fn:else(name1)}",
//		"${message.params[3]}",
//		"${fn:endif(name1)}",
//		"${uint(message.params[3])+uint(1)}",
//	}, &message.EventMessage{
//		Params: []string{"1", "2", "3", "4"},
//	})
//	require.NoError(t, err)
//
//	err = parser.ParseParams()
//	require.NoError(t, err)
//
//	expectedParams := []interface{}{"1","4",uint64(5)}
//	require.Equal(t, expectedParams, parser.GetGlobalParams())
//}

func TestExecuteIfElse_overwriteVar(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:var(testVar,uint64,1)}",
		"${message.params[0]}",
		"${fn:if(name1,uint(message.params[1])==uint(3))}",
		"${uint(message.params[0])+uint(message.params[1])}",
		"${fn:elif(name1,uint(message.params[1])==uint(3))}",
		"${uint(message.params[2])==uint(2)}",
		"${fn:else(name1)}",
		"${message.params[3]}",
		"${fn:var(testVar,uint64,2)}",
		"${fn:var(newVar,uint64,3)}",
		"${fn:endif(name1)}",
		"${uint(message.params[3])+uint(1)}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedParams := []interface{}{"1", "4", uint64(5)}
	require.Equal(t, expectedParams, parser.GetGlobalParams())

	expectedDefinedVar := map[string]interface{}{
		"testVar": uint64(2),
	}
	require.Equal(t, expectedDefinedVar, parser.UserDefinedVariables)
}

func TestForEach(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:var(testVar,uint64,1)}",
		"${fn:forEach(name1,message.params,index)}",
		"${fn:var(msgParam,uint64,message.params[index])}",
		"${fn:var(testVar,uint64,testVar+msgParam)}",
		"${fn:endForEach(name1)}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedDefinedVar := map[string]interface{}{
		"testVar": uint64(11),
	}
	require.Equal(t, expectedDefinedVar, parser.UserDefinedVariables)
}

func TestForEach1(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:var(l1,list,fn:split(message.params[0],';'))}",
		"${fn:var(l2,list,fn:split(message.params[1],';'))}",
		"${fn:var(l3,list,fn:split(message.params[2],';'))}",
		"${fn:var(l4,list,fn:split(message.params[3],';'))}",
		"${fn:var(testVar,uint64,1)}",
		"${fn:forEach(name1,l1,i)}",
		"${fn:var(el1,int,l1[i])}",
		"${fn:validate(el1==int(1),SIGNAL_CONTINUE,SIGNAL_RETURN)}",
		"${fn:endForEach(name1)}",
		"hello",
	}, &message.EventMessage{
		Params: []string{"1;2;3", "4;5;6", "7;8;9", "10;11;12"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedParams := []interface{}{"hello"}
	require.Equal(t, expectedParams, parser.GlobalParams)
}

func TestSplit(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:split(message.params[0],';')}",
	}, &message.EventMessage{
		Params: []string{"1;2;3;4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedParams := []interface{}{[]string{
		"1", "2", "3", "4",
	}}
	require.Equal(t, expectedParams, parser.GetGlobalParams())
}

func TestDefineFunc(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:defineFunc(testVar,params1,param2)}",
		"${uint(params1)+uint(params2)}",
		"${fn:endDefineFunc(testVar)}",
		"${message.params[1]}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
	require.Len(t, parser.GlobalPatterns, 1)
}

func TestDefine2Functions(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:defineFunc(testVar,params1,param2)}",
		"${uint(params1)+uint(params2)}",
		"${fn:endDefineFunc(testVar)}",
		"${fn:defineFunc(testVar1,params1,param2)}",
		"${uint(params1)-uint(params2)}",
		"${fn:endDefineFunc(testVar1)}",
		"${message.params[1]}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
	require.Len(t, parser.GlobalPatterns, 1)
}

func TestCallFunc(t *testing.T) {
	parser, err := setup(sampleCode2, sampleDefinition2, []string{
		"${fn:defineFunc(testVar,param1,param2)}",
		"${uint(param1)+uint(param2)}",
		"${fn:endDefineFunc(testVar)}",
		"${fn:call(testVar,message.params[0],message.params[1])}",
	}, &message.EventMessage{
		Params: []string{"1", "2", "3", "4"},
	})
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
	require.Len(t, parser.GlobalPatterns, 1)

	expectedParams := []interface{}{uint64(3)}
	require.Equal(t, expectedParams, parser.GetGlobalParams())
}

// @todo thangn test failed
// func TestTriggerSmc(t *testing.T) {
// 	parser, err := setup(sampleCode5, sampleDefinition5, []string{
// 		"${smc:trigger(setData, message.params[0])}",
// 	}, &message.EventMessage{
// 		Params: []string{"1"},
// 	})
// 	require.NoError(t, err)
// 	err = parser.ParseParams()
// 	require.NoError(t, err)

// 	expectedPoolLen := 1
// 	require.Equal(t, int(parser.TxPool.PendingSize()), expectedPoolLen)
// 	require.Equal(t, uint64(4), parser.Nonce)
// }

func TestPublishMessage(t *testing.T) {
	parser, err := setup(sampleCode5, sampleDefinition5, []string{
		"${['41d69c86eeaa2c1ea0e2db91a64c2ed5814ec66470','deposit',[message.params[0],message.params[1]]]}",
		"${fn:var(triggerMessage,list,params[0])}",
		"${['contractAddress','updateKardiaTx',[message.params[2]]]}",
		"${fn:var(cb1,list,params[1])}",
		"${[cb1]}",
		"${fn:var(callbacks,list,params[2])}",
		"${fn:publish(triggerMessage[0],triggerMessage[1],triggerMessage[2],callbacks)}",
	}, &message.EventMessage{
		Params: []string{"0x123", "10", "0x456"},
	})
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)
}

func TestGenerateOutputStruct(t *testing.T) {
	parser, err := setup(sampleCode6, sampleDefinition6, []string{
		"${smc:getData(getData)}",
	}, &message.EventMessage{
		Params: []string{"0x123", "10", "0x456"},
	})
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)
}

func TestReplaceFunction(t *testing.T) {
	parser, err := setup(sampleCode6, sampleDefinition6, []string{
		"${fn:var(testReplace,string,fn:replace('0xhelloWorld','0x',''))}",
	}, &message.EventMessage{})
	require.NoError(t, err)
	err = parser.ParseParams()
	require.NoError(t, err)
	expectedResult := "helloWorld"
	require.Equal(t, expectedResult, parser.UserDefinedVariables["testReplace"])
}
