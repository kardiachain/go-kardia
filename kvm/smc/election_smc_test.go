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

// Runtime_bytecode for ./DPoS_Election.sol
var voting_smc_code = common.Hex2Bytes("608060405260043610610110576000357c010000000000000000000000000000000000000000000000000000000090048063b048e056116100a7578063ec69a0e011610076578063ec69a0e014610480578063edc15b6b146104e5578063f8b2cb4f14610560578063fcf58fa5146105c557610110565b8063b048e0561461032f578063b7b0422d146103aa578063c94ca9e3146103e5578063e35c0f7d1461041457610110565b806359f78468116100e357806359f784681461025f5780635d593f8d146102695780636dd7d8ea146102945780638da5cb5b146102d857610110565b80632ccb1b3014610115578063484da961146101635780634cb273b2146101c85780635216509a14610234575b600080fd5b6101616004803603604081101561012b57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610845565b005b34801561016f57600080fd5b506101b26004803603602081101561018657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506108fb565b6040518082815260200191505060405180910390f35b3480156101d457600080fd5b506101dd610947565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b83811015610220578082015181840152602081019050610205565b505050509050019250505060405180910390f35b34801561024057600080fd5b506102496109d5565b6040518082815260200191505060405180910390f35b6102676109db565b005b34801561027557600080fd5b5061027e611140565b6040518082815260200191505060405180910390f35b6102d6600480360360208110156102aa57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611146565b005b3480156102e457600080fd5b506102ed611581565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561033b57600080fd5b506103686004803603602081101561035257600080fd5b81019080803590602001909291905050506115a6565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156103b657600080fd5b506103e3600480360360208110156103cd57600080fd5b81019080803590602001909291905050506115e4565b005b3480156103f157600080fd5b506103fa6116f7565b604051808215151515815260200191505060405180910390f35b34801561042057600080fd5b5061042961170a565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561046c578082015181840152602081019050610451565b505050509050019250505060405180910390f35b34801561048c57600080fd5b506104cf600480360360208110156104a357600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611798565b6040518082815260200191505060405180910390f35b3480156104f157600080fd5b5061051e6004803603602081101561050857600080fd5b81019080803590602001909291905050506118c4565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561056c57600080fd5b506105af6004803603602081101561058357600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611902565b6040518082815260200191505060405180910390f35b610843600480360360808110156105db57600080fd5b81019080803590602001906401000000008111156105f857600080fd5b82018360208201111561060a57600080fd5b8035906020019184600183028401116401000000008311171561062c57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561068f57600080fd5b8201836020820111156106a157600080fd5b803590602001918460018302840111640100000000831117156106c357600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561072657600080fd5b82018360208201111561073857600080fd5b8035906020019184600183028401116401000000008311171561075a57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156107bd57600080fd5b8201836020820111156107cf57600080fd5b803590602001918460018302840111640100000000831117156107f157600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050611923565b005b8173ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801561088b573d6000803e3d6000fd5b507fbb28353e4598c3b9199101a66e0989549b659a59a54d2c27fbb183f1932c8e6d8282604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050565b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001549050919050565b606060048054806020026020016040519081016040528092919081815260200182805480156109cb57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610981575b5050505050905090565b60025481565b600060149054906101000a900460ff16151515610a60576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610b07576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001806122f86024913960400191505060405180910390fd5b6001600060146101000a81548160ff0219169083151502179055506000600190505b6001548111158015610b3f575060048054905081105b15610be557600481815481101515610b5357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600560018303815481101515610b9057fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508080600101915050610b29565b506000600180540190505b6002548111151561113d57610c03612115565b60036000600484815481101515610c1657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060c0604051908101604052908160008201548152602001600182015481526020016002820160009054906101000a900460ff161515151581526020016003820160a06040519081016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610d605780601f10610d3557610100808354040283529160200191610d60565b820191906000526020600020905b815481529060010190602001808311610d4357829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610e025780601f10610dd757610100808354040283529160200191610e02565b820191906000526020600020905b815481529060010190602001808311610de557829003601f168201915b5050505050815260200160028201548152602001600382018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610eae5780601f10610e8357610100808354040283529160200191610eae565b820191906000526020600020905b815481529060010190602001808311610e9157829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610f505780601f10610f2557610100808354040283529160200191610f50565b820191906000526020600020905b815481529060010190602001808311610f3357829003601f168201915b50505050508152505081526020016008820154815260200160098201805480602002602001604051908101604052809291908181526020016000905b8282101561101e57838290600052602060002090600202016040805190810160405290816000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200160018201548152505081526020019060010190610f8c565b5050505081525050905060008090505b816080015181101561112e576110828260a001518281518110151561104f57fe5b90602001906020020151600001518360a001518381518110151561106f57fe5b9060200190602002015160200151610845565b60006003600060048681548110151561109757fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206009018281548110151561110c57fe5b906000526020600020906002020160010181905550808060010191505061102e565b50508080600101915050610bf0565b50565b60015481565b600060149054906101000a900460ff161515156111cb576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b600034111515611243576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff161515611307576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520646f6573206e6f74206578697374000000000000000081525060200191505060405180910390fd5b34600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160008282540192505081905550600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060090160408051908101604052803373ffffffffffffffffffffffffffffffffffffffff168152602001348152509080600181540180825580915050906001820390600052602060002090600202016000909192909190915060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101555050506001600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600801600082825401925050819055506114df600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010154611db9565b7f484413c33463e7d976e3ded2a85b0c31610d0906977df2cd9cbd6d12a6872a84338234604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a150565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6005818154811015156115b557fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060146101000a81548160ff021916908315150217905550806001819055506001546040519080825280602002602001820160405280156116765781602001602082028038833980820191505090505b506005908051906020019061168c929190612155565b506004600090806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050565b600060149054906101000a900460ff1681565b6060600580548060200260200160405190810160405280929190818152602001828054801561178e57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311611744575b5050505050905090565b600080600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015490508273ffffffffffffffffffffffffffffffffffffffff1660048281548110151561180657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614151561185057fe5b7f6510d4dce8156a73fb1e59829fc119ba136d8fd146b5dbda51e017781c18dfca8382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a180915050919050565b6004818154811015156118d357fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008173ffffffffffffffffffffffffffffffffffffffff16319050919050565b600060149054906101000a900460ff161515156119a8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b600034111515611a20576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff1615611ae3576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520616c726561647920657869737473000000000000000081525060200191505060405180910390fd5b60026000815480929190600101919050555034600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000018190555060a06040519081016040528085815260200184815260200134815260200183815260200182815250600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206003016000820151816000019080519060200190611bc29291906121df565b506020820151816001019080519060200190611bdf9291906121df565b50604082015181600201556060820151816003019080519060200190611c069291906121df565b506080820151816004019080519060200190611c239291906121df565b509050506001600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160006101000a81548160ff02191690831515021790555060043390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160048054905003600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550611d48600160048054905003611db9565b7f6578ae22e2e9b39b41597df980dc4daa54b024101d07dd0fffc49b85372fa4293334604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a150505050565b60008190505b60018111156121115760036000600460018403815481101515611dde57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000015460036000600484815481101515611e5a57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000154111515611ecf57612111565b6001810360036000600484815481101515611ee657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101819055508060036000600460018503815481101515611f6957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101819055506000600482815481101515611fe657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905060046001830381548110151561202557fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1660048381548110151561205f57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550806004600184038154811015156120ba57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050808060019003915050611dbf565b5050565b61014060405190810160405280600081526020016000815260200160001515815260200161214161225f565b815260200160008152602001606081525090565b8280548282559060005260206000209081019282156121ce579160200282015b828111156121cd5782518260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555091602001919060010190612175565b5b5090506121db919061228f565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061222057805160ff191683800117855561224e565b8280016001018555821561224e579182015b8281111561224d578251825591602001919060010190612232565b5b50905061225b91906122d2565b5090565b60a06040519081016040528060608152602001606081526020016000815260200160608152602001606081525090565b6122cf91905b808211156122cb57600081816101000a81549073ffffffffffffffffffffffffffffffffffffffff021916905550600101612295565b5090565b90565b6122f491905b808211156122f05760008160009055506001016122d8565b5090565b9056fe4f6e6c7920746865206f776e65722063616e2063616c6c207468652066756e6374696f6ea165627a7a72305820c288f8567f2df5451e4f63b4ec1d52f1079c1d688ee678accee2f7cdbb54704c0029")
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
