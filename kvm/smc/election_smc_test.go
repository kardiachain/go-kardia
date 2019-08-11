package kvm

import (
	kaidb "github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/kvm/sample_kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"math"
	"math/big"
	"strings"
	"testing"
)

// Runtime_bytecode for ./Voting.sol
var voting_smc_code = common.Hex2Bytes("6080604052600436106100f35760003560e01c8063b048e0561161008a578063ec69a0e011610059578063ec69a0e014610463578063edc15b6b146104c8578063f8b2cb4f14610543578063fcf58fa5146105a8576100f3565b8063b048e05614610312578063b7b0422d1461038d578063c94ca9e3146103c8578063e35c0f7d146103f7576100f3565b806359f78468116100c657806359f78468146102425780635d593f8d1461024c5780636dd7d8ea146102775780638da5cb5b146102bb576100f3565b80632ccb1b30146100f8578063484da961146101465780634cb273b2146101ab5780635216509a14610217575b600080fd5b6101446004803603604081101561010e57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610828565b005b34801561015257600080fd5b506101956004803603602081101561016957600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506108de565b6040518082815260200191505060405180910390f35b3480156101b757600080fd5b506101c061092a565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b838110156102035780820151818401526020810190506101e8565b505050509050019250505060405180910390f35b34801561022357600080fd5b5061022c6109b8565b6040518082815260200191505060405180910390f35b61024a6109be565b005b34801561025857600080fd5b50610261611109565b6040518082815260200191505060405180910390f35b6102b96004803603602081101561028d57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061110f565b005b3480156102c757600080fd5b506102d06115e9565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561031e57600080fd5b5061034b6004803603602081101561033557600080fd5b810190808035906020019092919050505061160e565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561039957600080fd5b506103c6600480360360208110156103b057600080fd5b810190808035906020019092919050505061164a565b005b3480156103d457600080fd5b506103dd61175d565b604051808215151515815260200191505060405180910390f35b34801561040357600080fd5b5061040c611770565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561044f578082015181840152602081019050610434565b505050509050019250505060405180910390f35b34801561046f57600080fd5b506104b26004803603602081101561048657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506117fe565b6040518082815260200191505060405180910390f35b3480156104d457600080fd5b50610501600480360360208110156104eb57600080fd5b8101908080359060200190929190505050611926565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561054f57600080fd5b506105926004803603602081101561056657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611962565b6040518082815260200191505060405180910390f35b610826600480360360808110156105be57600080fd5b81019080803590602001906401000000008111156105db57600080fd5b8201836020820111156105ed57600080fd5b8035906020019184600183028401116401000000008311171561060f57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561067257600080fd5b82018360208201111561068457600080fd5b803590602001918460018302840111640100000000831117156106a657600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561070957600080fd5b82018360208201111561071b57600080fd5b8035906020019184600183028401116401000000008311171561073d57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156107a057600080fd5b8201836020820111156107b257600080fd5b803590602001918460018302840111640100000000831117156107d457600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050611983565b005b8173ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801561086e573d6000803e3d6000fd5b507fbb28353e4598c3b9199101a66e0989549b659a59a54d2c27fbb183f1932c8e6d8282604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050565b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001549050919050565b606060048054806020026020016040519081016040528092919081815260200182805480156109ae57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610964575b5050505050905090565b60025481565b600060149054906101000a900460ff1615610a41576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614610ae6576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001806125186024913960400191505060405180910390fd5b6001600060146101000a81548160ff0219169083151502179055506000600190505b6001548111158015610b1e575060048054905081105b15610bc05760048181548110610b3057fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1660056001830381548110610b6b57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508080600101915050610b08565b506000600180540190505b600254811161110657610bdc61216a565b6003600060048481548110610bed57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206040518060c001604052908160008201548152602001600182015481526020016002820160009054906101000a900460ff16151515158152602001600382016040518060a0016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610d355780601f10610d0a57610100808354040283529160200191610d35565b820191906000526020600020905b815481529060010190602001808311610d1857829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610dd75780601f10610dac57610100808354040283529160200191610dd7565b820191906000526020600020905b815481529060010190602001808311610dba57829003601f168201915b5050505050815260200160028201548152602001600382018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610e835780601f10610e5857610100808354040283529160200191610e83565b820191906000526020600020905b815481529060010190602001808311610e6657829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610f255780601f10610efa57610100808354040283529160200191610f25565b820191906000526020600020905b815481529060010190602001808311610f0857829003601f168201915b50505050508152505081526020016008820154815260200160098201805480602002602001604051908101604052809291908181526020016000905b82821015610ff357838290600052602060002090600202016040518060400160405290816000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200160018201548152505081526020019060010190610f61565b5050505081525050905060008090505b81608001518110156110f75761104f8260a00151828151811061102257fe5b6020026020010151600001518360a00151838151811061103e57fe5b602002602001015160200151610828565b6000600360006004868154811061106257fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060090182815481106110d557fe5b9060005260206000209060020201600101819055508080600101915050611003565b50508080600101915050610bcb565b50565b60015481565b600060149054906101000a900460ff1615611192576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b60003411611208576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff166112ca576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520646f6573206e6f74206578697374000000000000000081525060200191505060405180910390fd5b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090503481600001600082825401925050819055508060090160405180604001604052803373ffffffffffffffffffffffffffffffffffffffff168152602001348152509080600181540180825580915050906001820390600052602060002090600202016000909192909190915060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060208201518160010155505050600181600801600082825401925050819055506113ed8160010154611e20565b80600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201548160000155600182015481600101556002820160009054906101000a900460ff168160020160006101000a81548160ff02191690831515021790555060038201816003016000820181600001908054600181600116156101000203166002900461149d9291906121a9565b50600182018160010190805460018160011615610100020316600290046114c59291906121a9565b5060028201548160020155600382018160030190805460018160011615610100020316600290046114f79291906121a9565b506004820181600401908054600181600116156101000203166002900461151f9291906121a9565b505050600882015481600801556009820181600901908054611542929190612230565b509050507f484413c33463e7d976e3ded2a85b0c31610d0906977df2cd9cbd6d12a6872a84338334604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a15050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6005818154811061161b57fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060146101000a81548160ff021916908315150217905550806001819055506001546040519080825280602002602001820160405280156116dc5781602001602082028038833980820191505090505b50600590805190602001906116f29291906122f9565b506004600090806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050565b600060149054906101000a900460ff1681565b606060058054806020026020016040519081016040528092919081815260200182805480156117f457602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190600101908083116117aa575b5050505050905090565b600080600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015490508273ffffffffffffffffffffffffffffffffffffffff166004828154811061186a57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16146118b257fe5b7f6510d4dce8156a73fb1e59829fc119ba136d8fd146b5dbda51e017781c18dfca8382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a180915050919050565b6004818154811061193357fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008173ffffffffffffffffffffffffffffffffffffffff16319050919050565b600060149054906101000a900460ff1615611a06576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b60003411611a7c576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff1615611b3f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520616c726561647920657869737473000000000000000081525060200191505060405180910390fd5b600260008154809291906001019190505550611b59612383565b6040518060a0016040528086815260200185815260200134815260200184815260200183815250905034600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000018190555080600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206003016000820151816000019080519060200190611c289291906123b2565b506020820151816001019080519060200190611c459291906123b2565b50604082015181600201556060820151816003019080519060200190611c6c9291906123b2565b506080820151816004019080519060200190611c899291906123b2565b509050506001600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160006101000a81548160ff02191690831515021790555060043390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160048054905003600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550611dae600160048054905003611e20565b7f6578ae22e2e9b39b41597df980dc4daa54b024101d07dd0fffc49b85372fa4293334604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050505050565b60008190505b6001811115612166576003600060046001840381548110611e4357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001546003600060048481548110611ebd57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000015411611f3057612166565b600181036003600060048481548110611f4557fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550806003600060046001850381548110611fc657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018190555060006004828154811061204157fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690506004600183038154811061207e57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600483815481106120b657fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550806004600184038154811061210f57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050808060019003915050611e26565b5050565b6040518061014001604052806000815260200160008152602001600015158152602001612195612432565b815260200160008152602001606081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106121e2578054855561221f565b8280016001018555821561221f57600052602060002091601f016020900482015b8281111561221e578254825591600101919060010190612203565b5b50905061222c9190612461565b5090565b8280548282559060005260206000209060020281019282156122e85760005260206000209160020282015b828111156122e75782826000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506001820154816001015550509160020191906002019061225b565b5b5090506122f59190612486565b5090565b828054828255906000526020600020908101928215612372579160200282015b828111156123715782518260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555091602001919060010190612319565b5b50905061237f91906124d4565b5090565b6040518060a0016040528060608152602001606081526020016000815260200160608152602001606081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106123f357805160ff1916838001178555612421565b82800160010185558215612421579182015b82811115612420578251825591602001919060010190612405565b5b50905061242e9190612461565b5090565b6040518060a0016040528060608152602001606081526020016000815260200160608152602001606081525090565b61248391905b8082111561247f576000816000905550600101612467565b5090565b90565b6124d191905b808211156124cd57600080820160006101000a81549073ffffffffffffffffffffffffffffffffffffffff021916905560018201600090555060020161248c565b5090565b90565b61251491905b8082111561251057600081816101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055506001016124da565b5090565b9056fe4f6e6c7920746865206f776e65722063616e2063616c6c207468652066756e6374696f6ea165627a7a7230582086c2b22052a71f59fa66b711286295865e47e53f98f2490491f9586a1915e7cc0029")
var voting_smc_definition = `[
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

func SetupBlockchainForTesting() (*blockchain.BlockChain, error) {

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
	kaiDb := kaidb.NewMemStore()
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), kaiDb, g)
	if genesisErr != nil {
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, true)
	return bc, err
}

// Test DPoS_Election smc with multiple candidates and voters
// After the election ends, verify the validatorList and check that voters are refunded if needed
func TestElection (t *testing.T) {
	bc, err := SetupBlockchainForTesting()
	if err != nil {
		t.Fatal(err)
	}
	state, err := bc.State()

	if err != nil {
		t.Fatal(err)
	}

	// Setup contract code into genesis state
	address := common.HexToAddress("0x0a")
	state.SetCode(address, voting_smc_code)
	abi, err := abi.JSON(strings.NewReader(voting_smc_definition))
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
	// candidate 1 signs up again, should be reverted
	candidate1 = common.HexToAddress("0x1111")
	signup, err = abi.Pack("signup", "pubKey1", "name1", "40/60", "description")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, signup, &sample_kvm.Config{State: state, Value: big.NewInt(100), Origin: candidate1})
	if err == nil {
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

	// verify that candidate 1 now ranks 1st
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
	if !(result[len(result) - 1] == 0) {
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
	if !(result[len(result) - 1] == 1) {
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
