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
var voting_smc_code = common.Hex2Bytes("6080604052600436106100f35760003560e01c8063b048e0561161008a578063ec69a0e011610059578063ec69a0e014610463578063edc15b6b146104c8578063f8b2cb4f14610543578063fcf58fa5146105a8576100f3565b8063b048e05614610337578063b7b0422d146103b2578063b9223946146103ed578063e35c0f7d146103f7576100f3565b80634cb273b2116100c65780634cb273b21461020557806364f62cc0146102715780636dd7d8ea1461029c5780638da5cb5b146102e0576100f3565b80630deeca6c146100f85780631e526e45146101275780632ccb1b3014610152578063484da961146101a0575b600080fd5b34801561010457600080fd5b5061010d610828565b604051808215151515815260200191505060405180910390f35b34801561013357600080fd5b5061013c61083b565b6040518082815260200191505060405180910390f35b61019e6004803603604081101561016857600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610841565b005b3480156101ac57600080fd5b506101ef600480360360208110156101c357600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506108f7565b6040518082815260200191505060405180910390f35b34801561021157600080fd5b5061021a610943565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561025d578082015181840152602081019050610242565b505050509050019250505060405180910390f35b34801561027d57600080fd5b506102866109d1565b6040518082815260200191505060405180910390f35b6102de600480360360208110156102b257600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506109d7565b005b3480156102ec57600080fd5b506102f5610eb1565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561034357600080fd5b506103706004803603602081101561035a57600080fd5b8101908080359060200190929190505050610ed6565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156103be57600080fd5b506103eb600480360360208110156103d557600080fd5b8101908080359060200190929190505050610f12565b005b6103f5611025565b005b34801561040357600080fd5b5061040c611770565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561044f578082015181840152602081019050610434565b505050509050019250505060405180910390f35b34801561046f57600080fd5b506104b26004803603602081101561048657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506117fe565b6040518082815260200191505060405180910390f35b3480156104d457600080fd5b50610501600480360360208110156104eb57600080fd5b8101908080359060200190929190505050611926565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561054f57600080fd5b506105926004803603602081101561056657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611962565b6040518082815260200191505060405180910390f35b610826600480360360808110156105be57600080fd5b81019080803590602001906401000000008111156105db57600080fd5b8201836020820111156105ed57600080fd5b8035906020019184600183028401116401000000008311171561060f57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561067257600080fd5b82018360208201111561068457600080fd5b803590602001918460018302840111640100000000831117156106a657600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561070957600080fd5b82018360208201111561071b57600080fd5b8035906020019184600183028401116401000000008311171561073d57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156107a057600080fd5b8201836020820111156107b257600080fd5b803590602001918460018302840111640100000000831117156107d457600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050611983565b005b600060149054906101000a900460ff1681565b60015481565b8173ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050158015610887573d6000803e3d6000fd5b507fbb28353e4598c3b9199101a66e0989549b659a59a54d2c27fbb183f1932c8e6d8282604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050565b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001549050919050565b606060048054806020026020016040519081016040528092919081815260200182805480156109c757602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001906001019080831161097d575b5050505050905090565b60025481565b600060149054906101000a900460ff1615610a5a576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b60003411610ad0576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff16610b92576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520646f6573206e6f74206578697374000000000000000081525060200191505060405180910390fd5b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090503481600001600082825401925050819055508060090160405180604001604052803373ffffffffffffffffffffffffffffffffffffffff168152602001348152509080600181540180825580915050906001820390600052602060002090600202016000909192909190915060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506020820151816001015550505060018160080160008282540192505081905550610cb58160010154611e20565b80600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201548160000155600182015481600101556002820160009054906101000a900460ff168160020160006101000a81548160ff021916908315150217905550600382018160030160008201816000019080546001816001161561010002031660029004610d6592919061216a565b5060018201816001019080546001816001161561010002031660029004610d8d92919061216a565b506002820154816002015560038201816003019080546001816001161561010002031660029004610dbf92919061216a565b5060048201816004019080546001816001161561010002031660029004610de792919061216a565b505050600882015481600801556009820181600901908054610e0a9291906121f1565b509050507f484413c33463e7d976e3ded2a85b0c31610d0906977df2cd9cbd6d12a6872a84338334604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a15050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60058181548110610ee357fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060146101000a81548160ff02191690831515021790555080600181905550600154604051908082528060200260200182016040528015610fa45781602001602082028038833980820191505090505b5060059080519060200190610fba9291906122ba565b506004600090806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050565b600060149054906101000a900460ff16156110a8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161461114d576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260248152602001806125186024913960400191505060405180910390fd5b6001600060146101000a81548160ff0219169083151502179055506000600190505b6001548111158015611185575060048054905081105b15611227576004818154811061119757fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600560018303815481106111d257fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550808060010191505061116f565b506000600180540190505b600254811161176d57611243612344565b600360006004848154811061125457fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206040518060c001604052908160008201548152602001600182015481526020016002820160009054906101000a900460ff16151515158152602001600382016040518060a0016040529081600082018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561139c5780601f106113715761010080835404028352916020019161139c565b820191906000526020600020905b81548152906001019060200180831161137f57829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561143e5780601f106114135761010080835404028352916020019161143e565b820191906000526020600020905b81548152906001019060200180831161142157829003601f168201915b5050505050815260200160028201548152602001600382018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156114ea5780601f106114bf576101008083540402835291602001916114ea565b820191906000526020600020905b8154815290600101906020018083116114cd57829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561158c5780601f106115615761010080835404028352916020019161158c565b820191906000526020600020905b81548152906001019060200180831161156f57829003601f168201915b50505050508152505081526020016008820154815260200160098201805480602002602001604051908101604052809291908181526020016000905b8282101561165a57838290600052602060002090600202016040518060400160405290816000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001600182015481525050815260200190600101906115c8565b5050505081525050905060008090505b816080015181101561175e576116b68260a00151828151811061168957fe5b6020026020010151600001518360a0015183815181106116a557fe5b602002602001015160200151610841565b600060036000600486815481106116c957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600901828154811061173c57fe5b906000526020600020906002020160010181905550808060010191505061166a565b50508080600101915050611232565b50565b606060058054806020026020016040519081016040528092919081815260200182805480156117f457602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190600101908083116117aa575b5050505050905090565b600080600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015490508273ffffffffffffffffffffffffffffffffffffffff166004828154811061186a57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16146118b257fe5b7f6510d4dce8156a73fb1e59829fc119ba136d8fd146b5dbda51e017781c18dfca8382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a180915050919050565b6004818154811061193357fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008173ffffffffffffffffffffffffffffffffffffffff16319050919050565b600060149054906101000a900460ff1615611a06576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f54686520766f7465206973206e6f7420656e646564207965740000000000000081525060200191505060405180910390fd5b60003411611a7c576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f56616c7565206f66207374616b65206d75737420626520706f7369746976650081525060200191505060405180910390fd5b600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff1615611b3f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260188152602001807f43616e64696461746520616c726561647920657869737473000000000000000081525060200191505060405180910390fd5b600260008154809291906001019190505550611b59612383565b6040518060a0016040528086815260200185815260200134815260200184815260200183815250905034600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000018190555080600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206003016000820151816000019080519060200190611c289291906123b2565b506020820151816001019080519060200190611c459291906123b2565b50604082015181600201556060820151816003019080519060200190611c6c9291906123b2565b506080820151816004019080519060200190611c899291906123b2565b509050506001600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160006101000a81548160ff02191690831515021790555060043390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160048054905003600360003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550611dae600160048054905003611e20565b7f6578ae22e2e9b39b41597df980dc4daa54b024101d07dd0fffc49b85372fa4293334604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050505050565b60008190505b6001811115612166576003600060046001840381548110611e4357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001546003600060048481548110611ebd57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000015411611f3057612166565b600181036003600060048481548110611f4557fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010181905550806003600060046001850381548110611fc657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018190555060006004828154811061204157fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690506004600183038154811061207e57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600483815481106120b657fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550806004600184038154811061210f57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050808060019003915050611e26565b5050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106121a357805485556121e0565b828001600101855582156121e057600052602060002091601f016020900482015b828111156121df5782548255916001019190600101906121c4565b5b5090506121ed9190612432565b5090565b8280548282559060005260206000209060020281019282156122a95760005260206000209160020282015b828111156122a85782826000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506001820154816001015550509160020191906002019061221c565b5b5090506122b69190612457565b5090565b828054828255906000526020600020908101928215612333579160200282015b828111156123325782518260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550916020019190600101906122da565b5b50905061234091906124a5565b5090565b604051806101400160405280600081526020016000815260200160001515815260200161236f6124e8565b815260200160008152602001606081525090565b6040518060a0016040528060608152602001606081526020016000815260200160608152602001606081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106123f357805160ff1916838001178555612421565b82800160010185558215612421579182015b82811115612420578251825591602001919060010190612405565b5b50905061242e9190612432565b5090565b61245491905b80821115612450576000816000905550600101612438565b5090565b90565b6124a291905b8082111561249e57600080820160006101000a81549073ffffffffffffffffffffffffffffffffffffffff021916905560018201600090555060020161245d565b5090565b90565b6124e591905b808211156124e157600081816101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055506001016124ab565b5090565b90565b6040518060a001604052806060815260200160608152602001600081526020016060815260200160608152509056fe4f6e6c7920746865206f776e65722063616e2063616c6c207468652066756e6374696f6ea265627a7a7230582081785052a7a72991ad9bd56b2fabd83ee703ee49ac42390cd66ce20f612d5a9f64736f6c63430005090032")
var voting_smc_definition = `[
	{
		"constant": true,
		"inputs": [],
		"name": "voteEnded",
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
		"name": "numOfValidators",
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
		"name": "numOfCandidates",
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
		"constant": false,
		"inputs": [],
		"name": "endVote",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
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

func TestVoting (t *testing.T) {
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

	// check that numOfValidators == 3
	getNumOfValidators, err := abi.Pack("numOfValidators")
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

	// get balance of sender2 before the vote ends
	getBalance, err := abi.Pack("getBalance", sender2)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getBalance, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	balanceBefore := new(big.Int).SetBytes(result)

	// check that the vote is not ended
	voteEnded, err := abi.Pack("voteEnded")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, voteEnded, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	if !(result[len(result) - 1] == 0) {
		t.Error("Expected true, got ", result)
	}

	// now we end the vote
	endVote, err := abi.Pack("endVote")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, endVote, &sample_kvm.Config{State: state, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// check that the vote is ended
	voteEnded, err = abi.Pack("voteEnded")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, voteEnded, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	if !(result[len(result) - 1] == 1) {
		t.Error("Expected true, got ", result)
	}

	// candidate 1, 4, 2 were in top 3 so they are in validator list after the vote ended
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

	// // check current validator list
	// getValList, err = abi.Pack("getValidatorList")
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// result, _, err = sample_kvm.Call(address, getValList, &sample_kvm.Config{State: state})
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// t.Error(result[:]) //appear to have 2 chunk elems at front
	//
	// // Solidity memory layout is with 32-byte boundary so although address is 20-byte, it will return 32-byte.
	// addr = common.BytesToAddress(result[12:32])
	// if !addr.Equal(candidate1) {
	// 	t.Error("Expected candidate1, got ", addr.String())
	// }

	// sender2 should be refunded since candidate3 failed to be elected
	// get balance of sender2 after the vote ends
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
