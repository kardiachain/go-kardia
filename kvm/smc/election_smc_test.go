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

package kvm

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/kvm/sample_kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
)

// Runtime_bytecode for ./DPoS_Election.sol
var election_smc_code = common.Hex2Bytes("60806040526004361061011b576000357c010000000000000000000000000000000000000000000000000000000090048063b048e056116100b2578063e35c0f7d11610081578063e35c0f7d1461044e578063ec69a0e0146104ba578063edc15b6b1461051f578063f8b2cb4f1461059a578063fcf58fa5146105ff5761011b565b8063b048e0561461033a578063b7b0422d146103b5578063c94ca9e3146103f0578063d1f9e4091461041f5761011b565b806359f78468116100ee57806359f784681461026a5780635d593f8d146102745780636dd7d8ea1461029f5780638da5cb5b146102e35761011b565b80632ccb1b3014610120578063484da9611461016e5780634cb273b2146101d35780635216509a1461023f575b600080fd5b61016c6004803603604081101561013657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061087f565b005b34801561017a57600080fd5b506101bd6004803603602081101561019157600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610935565b6040518082815260200191505060405180910390f35b3480156101df57600080fd5b506101e8610981565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561022b578082015181840152602081019050610210565b505050509050019250505060405180910390f35b34801561024b57600080fd5b50610254610a0f565b6040518082815260200191505060405180910390f35b610272610a15565b005b34801561028057600080fd5b506102896111fe565b6040518082815260200191505060405180910390f35b6102e1600480360360208110156102b557600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611204565b005b3480156102ef57600080fd5b506102f86116c3565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561034657600080fd5b506103736004803603602081101561035d57600080fd5b81019080803590602001909291905050506116e8565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156103c157600080fd5b506103ee600480360360208110156103d857600080fd5b8101908080359060200190929190505050611726565b005b3480156103fc57600080fd5b50610405611934565b604051808215151515815260200191505060405180910390f35b34801561042b57600080fd5b50610434611947565b604051808215151515815260200191505060405180910390f35b34801561045a57600080fd5b5061046361195a565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b838110156104a657808201518184015260208101905061048b565b505050509050019250505060405180910390f35b3480156104c657600080fd5b50610509600480360360208110156104dd57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506119e8565b6040518082815260200191505060405180910390f35b34801561052b57600080fd5b506105586004803603602081101561054257600080fd5b8101908080359060200190929190505050611b14565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156105a657600080fd5b506105e9600480360360208110156105bd57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611b52565b6040518082815260200191505060405180910390f35b61087d6004803603608081101561061557600080fd5b810190808035906020019064010000000081111561063257600080fd5b82018360208201111561064457600080fd5b8035906020019184600183028401116401000000008311171561066657600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156106c957600080fd5b8201836020820111156106db57600080fd5b803590602001918460018302840111640100000000831117156106fd57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561076057600080fd5b82018360208201111561077257600080fd5b8035906020019184600183028401116401000000008311171561079457600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156107f757600080fd5b82018360208201111561080957600080fd5b8035906020019184600183028401116401000000008311171561082b57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050611b73565b005b8173ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f193505050501580156108c5573d6000803e3d6000fd5b507fbb28353e4598c3b9199101a66e0989549b659a59a54d2c27fbb183f1932c8e6d8282604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050565b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001549050919050565b60606004805480602002602001604051908101604052809291908181526020018280548015610a0557602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190600101908083116109bb575b5050505050905090565b60025481565b600060149054906101000a900460ff161515610a99576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f54686520636f6e7472616374206d75737420626520696e697469616c697a656481525060200191505060405180910390fd5b600060159054906101000a900460ff16151515610b1e576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610bc5576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001806125cc6024913960400191505060405180910390fd5b6001600060156101000a81548160ff0219169083151502179055506000600190505b6001548111158015610bfd575060048054905081105b15610ca357600481815481101515610c1157fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600560018303815481101515610c4e57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508080600101915050610be7565b506000600180540190505b600254811115156111fb57610cc16123e9565b60036000600484815481101515610cd457fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060c0604051908101604052908160008201548152602001600182015481526020016002820160009054906101000a900460ff161515151581526020016003820160a06040519081016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610e1e5780601f10610df357610100808354040283529160200191610e1e565b820191906000526020600020905b815481529060010190602001808311610e0157829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610ec05780601f10610e9557610100808354040283529160200191610ec0565b820191906000526020600020905b815481529060010190602001808311610ea357829003601f168201915b5050505050815260200160028201548152602001600382018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610f6c5780601f10610f4157610100808354040283529160200191610f6c565b820191906000526020600020905b815481529060010190602001808311610f4f57829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561100e5780601f10610fe35761010080835404028352916020019161100e565b820191906000526020600020905b815481529060010190602001808311610ff157829003601f168201915b50505050508152505081526020016008820154815260200160098201805480602002602001604051908101604052809291908181526020016000905b828210156110dc57838290600052602060002090600202016040805190810160405290816000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020016001820154815250508152602001906001019061104a565b5050505081525050905060008090505b81608001518110156111ec576111408260a001518281518110151561110d57fe5b90602001906020020151600001518360a001518381518110151561112d57fe5b906020019060200201516020015161087f565b60006003600060048681548110151561115557fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600901828154811015156111ca57fe5b90600052602060002090600202016001018190555080806001019150506110ec565b50508080600101915050610cae565b50565b60015481565b600060149054906101000a900460ff161515611288576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f54686520636f6e7472616374206d75737420626520696e697469616c697a656481525060200191505060405180910390fd5b600060159054906101000a900460ff1615151561130d576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b600034111515611385576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff161515611449576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520646f6573206e6f74206578697374000000000000000081525060200191505060405180910390fd5b34600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160008282540192505081905550600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060090160408051908101604052803373ffffffffffffffffffffffffffffffffffffffff168152602001348152509080600181540180825580915050906001820390600052602060002090600202016000909192909190915060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101555050506001600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060080160008282540192505081905550611621600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015461208d565b7f484413c33463e7d976e3ded2a85b0c31610d0906977df2cd9cbd6d12a6872a84338234604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a150565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6005818154811015156116f757fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600060149054906101000a900460ff161515156117ab576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601c8152602001807f496e69742063616e206f6e6c792062652063616c6c6564206f6e63650000000081525060200191505060405180910390fd5b600081111515611806576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260258152602001806125f06025913960400191505060405180910390fd5b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060156101000a81548160ff021916908315150217905550806001819055506001546040519080825280602002602001820160405280156118985781602001602082028038833980820191505090505b50600590805190602001906118ae929190612429565b506004600090806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550506001600060146101000a81548160ff02191690831515021790555050565b600060159054906101000a900460ff1681565b600060149054906101000a900460ff1681565b606060058054806020026020016040519081016040528092919081815260200182805480156119de57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311611994575b5050505050905090565b600080600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015490508273ffffffffffffffffffffffffffffffffffffffff16600482815481101515611a5657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141515611aa057fe5b7f6510d4dce8156a73fb1e59829fc119ba136d8fd146b5dbda51e017781c18dfca8382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a180915050919050565b600481815481101515611b2357fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008173ffffffffffffffffffffffffffffffffffffffff16319050919050565b600060149054906101000a900460ff161515611bf7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f54686520636f6e7472616374206d75737420626520696e697469616c697a656481525060200191505060405180910390fd5b600060159054906101000a900460ff16151515611c7c576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b600034111515611cf4576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff1615611db7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520616c726561647920657869737473000000000000000081525060200191505060405180910390fd5b60026000815480929190600101919050555034600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000018190555060a06040519081016040528085815260200184815260200134815260200183815260200182815250600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206003016000820151816000019080519060200190611e969291906124b3565b506020820151816001019080519060200190611eb39291906124b3565b50604082015181600201556060820151816003019080519060200190611eda9291906124b3565b506080820151816004019080519060200190611ef79291906124b3565b509050506001600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160006101000a81548160ff02191690831515021790555060043390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160048054905003600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018190555061201c60016004805490500361208d565b7f6578ae22e2e9b39b41597df980dc4daa54b024101d07dd0fffc49b85372fa4293334604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a150505050565b60008190505b60018111156123e557600360006004600184038154811015156120b257fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001546003600060048481548110151561212e57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001541115156121a3576123e5565b60018103600360006004848154811015156121ba57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550806003600060046001850381548110151561223d57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018190555060006004828154811015156122ba57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690506004600183038154811015156122f957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1660048381548110151561233357fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508060046001840381548110151561238e57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050808060019003915050612093565b5050565b610140604051908101604052806000815260200160008152602001600015158152602001612415612533565b815260200160008152602001606081525090565b8280548282559060005260206000209081019282156124a2579160200282015b828111156124a15782518260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555091602001919060010190612449565b5b5090506124af9190612563565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106124f457805160ff1916838001178555612522565b82800160010185558215612522579182015b82811115612521578251825591602001919060010190612506565b5b50905061252f91906125a6565b5090565b60a06040519081016040528060608152602001606081526020016000815260200160608152602001606081525090565b6125a391905b8082111561259f57600081816101000a81549073ffffffffffffffffffffffffffffffffffffffff021916905550600101612569565b5090565b90565b6125c891905b808211156125c45760008160009055506001016125ac565b5090565b9056fe4f6e6c7920746865206f776e65722063616e2063616c6c207468652066756e6374696f6e4e756d626572206f662076616c696461746f7273206d75737420626520706f736974697665a165627a7a72305820d725e26977f4cadc9b672497a56fee1dc02969b2e8ff7d1f1d0dd8fd9532abd50029")
var election_smc_definition = `[
	{
		"constant": false,
		"inputs": [
			{
				"name": "recipient",
				"type": "address"
			},
			{
				"name": "amount",
				"type": "uint256"
			}
		],
		"name": "transferTo",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "candAddress",
				"type": "address"
			}
		],
		"name": "getCandidateStake",
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
		"constant": true,
		"inputs": [],
		"name": "getCurrentRankings",
		"outputs": [
			{
				"name": "",
				"type": "address[]"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "numCandidates",
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
		"constant": false,
		"inputs": [],
		"name": "endElection",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "numValidators",
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
		"constant": false,
		"inputs": [
			{
				"name": "candAddress",
				"type": "address"
			}
		],
		"name": "vote",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "owner",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "",
				"type": "uint256"
			}
		],
		"name": "validatorList",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "n",
				"type": "uint256"
			}
		],
		"name": "init",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "electionEnded",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "initCalled",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getValidatorList",
		"outputs": [
			{
				"name": "",
				"type": "address[]"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "candAddress",
				"type": "address"
			}
		],
		"name": "getCandidateRanking",
		"outputs": [
			{
				"name": "",
				"type": "uint256"
			}
		],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "",
				"type": "uint256"
			}
		],
		"name": "rankings",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "addr",
				"type": "address"
			}
		],
		"name": "getBalance",
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
		"constant": false,
		"inputs": [
			{
				"name": "pubKey",
				"type": "string"
			},
			{
				"name": "name",
				"type": "string"
			},
			{
				"name": "ratio",
				"type": "string"
			},
			{
				"name": "description",
				"type": "string"
			}
		],
		"name": "signup",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "candidate",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "stakes",
				"type": "uint256"
			}
		],
		"name": "Signup",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "voter",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "candidate",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "stakes",
				"type": "uint256"
			}
		],
		"name": "VoteCast",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "candidate",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "position",
				"type": "uint256"
			}
		],
		"name": "CandidateRanking",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "voter",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "value",
				"type": "uint256"
			}
		],
		"name": "Refund",
		"type": "event"
	}
]`

func SetupBlockchain() (*blockchain.BlockChain, error) {

	initValue := genesis.ToCell(int64(math.Pow10(6)))

	var genesisAccounts = map[string]*big.Int{
		"0x1111": initValue,
		"0x2222": initValue,
		"0x3333": initValue,
		"0x4444": initValue,
		"0x1234": initValue,
		"0x5678": initValue,
		"0xabcd": initValue,
	}
	blockDB := memorydb.New()
	storeDB := kvstore.NewStoreDB(blockDB)
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	chainConfig, _, genesisErr := setupGenesis(g, storeDB)
	if genesisErr != nil {
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, true)
	return bc, err
}

// Test_Failed_Init tests for the cases when function init fails
// Init function can only be called once and the argument passed in must be positive
func Test_Failed_Init(t *testing.T) {
	bc, err := SetupBlockchain()
	if err != nil {
		t.Fatal(err)
	}
	state, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}
	// Setup contract code into genesis state
	address := common.HexToAddress("0x0a")
	state.SetCode(address, election_smc_code)
	abi, err := abi.JSON(strings.NewReader(election_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// Failed when initialize with a negative number of validators
	owner := common.HexToAddress("0x1234")
	init, err := abi.Pack("init", big.NewInt(-5))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, init, &sample_kvm.Config{State: state, Origin: owner})
	if err == nil {
		t.Fatal(err)
	}

	// Successfully init the first time
	init, err = abi.Pack("init", big.NewInt(5))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, init, &sample_kvm.Config{State: state, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// Failed when calling init more than once
	init, err = abi.Pack("init", big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, init, &sample_kvm.Config{State: state, Origin: owner})
	if err == nil {
		t.Fatal(err)
	}
}

// Test_Failed_Init tests for the cases when function signup() and vote() fail
// Candidates can only sign up once, voters can only vote for existing candidates and the value passed in the functions must be positive
func Test_Bad_Sender(t *testing.T) {
	bc, err := SetupBlockchain()
	if err != nil {
		t.Fatal(err)
	}
	state, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}
	// Setup contract code into genesis state
	address := common.HexToAddress("0x0a")
	state.SetCode(address, election_smc_code)
	abi, err := abi.JSON(strings.NewReader(election_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// Try signing up before initialized, should be reverted
	candidate1 := common.HexToAddress("0x1111")
	signup, err := abi.Pack("signup", "pubKey1", "name1", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: candidate1})
	if err == nil {
		t.Fatal(err)
	}

	// initialize
	owner := common.HexToAddress("0x1234")
	init, err := abi.Pack("init", big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, init, &sample_kvm.Config{State: state, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// candidate 1 signs up with 100 kai
	signup, err = abi.Pack("signup", "pubKey1", "name1", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: candidate1})
	if err != nil {
		t.Fatal(err)
	}
	// Candidate 1 fails to sign up again
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: candidate1})
	if err == nil {
		t.Fatal(err)
	}

	// Candidate 2 fails to sign up with 0 kai
	candidate2 := common.HexToAddress("0x2222")
	signup, err = abi.Pack("signup", "pubKey1", "name1", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(0), Origin: candidate2})
	if err == nil {
		t.Fatal(err)
	}

	// Failed to vote for non-existing candidate 2
	voter1 := common.HexToAddress("0xabcd")
	vote, err := abi.Pack("vote", candidate2)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Value: big.NewInt(600), Origin: voter1})
	if err == nil {
		t.Fatal(err)
	}

	// Failed to delegate -600 coins to candidate 1
	vote, err = abi.Pack("vote", candidate2)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Value: big.NewInt(-600), Origin: voter1})
	if err == nil {
		t.Fatal(err)
	}
}

// Test_Successful_Election tests DPoS_Election smc with multiple candidates and voters
// After the election ends, verify the validatorList and check that voters are refunded if needed
func Test_Successful_Election(t *testing.T) {
	bc, err := SetupBlockchain()
	if err != nil {
		t.Fatal(err)
	}
	state, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}
	// Setup contract code into genesis state
	address := common.HexToAddress("0x0a")
	state.SetCode(address, election_smc_code)
	abi, err := abi.JSON(strings.NewReader(election_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// initialize
	owner := common.HexToAddress("0x1234")
	init, err := abi.Pack("init", big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, init, &sample_kvm.Config{State: state, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// check that number of validators == 3
	getNumOfValidators, err := abi.Pack("numValidators")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := sample_kvm.Call(address, getNumOfValidators, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(3)) != 0 {
		t.Error("Expected 3, got ", num)
	}
	// candidate 1 signs up with 100 kai
	candidate1 := common.HexToAddress("0x1111")
	signup, err := abi.Pack("signup", "pubKey1", "name1", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: candidate1})
	if err != nil {
		t.Fatal(err)
	}

	// candidate 2 signs up with 500 kai
	candidate2 := common.HexToAddress("0x2222")
	signup, err = abi.Pack("signup", "pubKey2", "name2", "25/75", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(500), Origin: candidate2})
	if err != nil {
		t.Fatal(err)
	}

	// candidate 3 signs up with 200 kai
	candidate3 := common.HexToAddress("0x3333")
	signup, err = abi.Pack("signup", "pubKey3", "name3", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(200), Origin: candidate3})
	if err != nil {
		t.Fatal(err)
	}

	// verify that candidate 2 ranks 1st
	getCandidateRanking, err := abi.Pack("getCandidateRanking", candidate2)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getCandidateRanking, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected candidate 2 to rank #1, got #", num)
	}

	// vote and add 600 coins for candidate 1
	sender1 := common.HexToAddress("0xabcd")
	vote, err := abi.Pack("vote", candidate1)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Value: big.NewInt(600), Origin: sender1})
	if err != nil {
		t.Fatal(err)
	}

	// verify that candidate 1 now has 700 coins
	getCandidateStake, err := abi.Pack("getCandidateStake", candidate1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getCandidateStake, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(700)) != 0 {
		t.Error("Expected 700 stake, got", num)
	}

	// verify that candidate 2 now ranks 1st
	getCandidateRanking, err = abi.Pack("getCandidateRanking", candidate1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getCandidateRanking, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected candidate 2 to rank #1, got #", num)
	}

	// candidate 4 signs up with 600 kai
	candidate4 := common.HexToAddress("0x4444")
	signup, err = abi.Pack("signup", "pubKey4", "name4", "30/70", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(600), Origin: candidate4})
	if err != nil {
		t.Fatal(err)
	}

	// vote and add 100 coins for candidate 3
	sender2 := common.HexToAddress("0x5678")
	vote, err = abi.Pack("vote", candidate3)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: sender2})
	if err != nil {
		t.Fatal(err)
	}

	// get balance of sender2 before the election ends
	getBalance, err := abi.Pack("getBalance", sender2)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getBalance, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	balanceBefore := new(big.Int).SetBytes(result)

	// check that the election is not ended
	electionEnded, err := abi.Pack("electionEnded")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, electionEnded, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	if !(result[len(result)-1] == 0) {
		t.Error("Expected true, got ", result)
	}

	// now we end the election
	endElection, err := abi.Pack("endElection")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, endElection, &sample_kvm.Config{State: state, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// check that the election is ended
	electionEnded, err = abi.Pack("electionEnded")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, electionEnded, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	if !(result[len(result)-1] == 1) {
		t.Error("Expected true, got ", result)
	}

	// candidate 1, 4, 2 were in top 3 so they are in validator list after the election ended
	// check current validator list
	getValList, err := abi.Pack("validatorList", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getValList, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	// Solidity memory layout is with 32-byte boundary so although address is 20-byte, it will return 32-byte.
	addr := common.BytesToAddress(result)
	if !addr.Equal(candidate4) {
		t.Error("Expected candidate4 at #2, got ", addr.String())
	}

	// check current validator list
	getValList, err = abi.Pack("getValidatorList")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getValList, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	result = result[64:]

	// Solidity memory layout is with 32-byte boundary so although address is 20-byte, it will return 32-byte.
	addr = common.BytesToAddress(result[12:32])
	if !addr.Equal(candidate1) {
		t.Error("Expected candidate1, got ", addr.String())
	}

	// sender2 should be refunded since candidate3 failed to be elected
	// get balance of sender2 after the election ends
	getBalance, err = abi.Pack("getBalance", sender2)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getBalance, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	balanceAfter := new(big.Int).SetBytes(result)
	amount := big.NewInt(0)
	amount = amount.Sub(balanceAfter, balanceBefore)
	if amount.Cmp(big.NewInt(100)) != 0 {
		t.Error("Expected 100 coins, got", amount)
	}
}
