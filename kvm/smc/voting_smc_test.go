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
	// "bytes"
	// "encoding/json"
)

// Runtime_bytecode for ./Voting.sol
var voting_smc_code = common.Hex2Bytes("6080604052600436106100ca576000357c0100000000000000000000000000000000000000000000000000000000900480630deeca6c146100cf5780631e526e45146100fe578063484da961146101295780634cb273b21461018e5780636dd7d8ea146101fa5780638da5cb5b1461023e578063b048e05614610295578063b7b0422d14610310578063b92239461461034b578063e35c0f7d14610355578063ec69a0e0146103c1578063edc15b6b14610426578063f8b2cb4f146104a1578063fcf58fa514610506575b600080fd5b3480156100db57600080fd5b506100e4610786565b604051808215151515815260200191505060405180910390f35b34801561010a57600080fd5b50610113610799565b6040518082815260200191505060405180910390f35b34801561013557600080fd5b506101786004803603602081101561014c57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061079f565b6040518082815260200191505060405180910390f35b34801561019a57600080fd5b506101a36107eb565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b838110156101e65780820151818401526020810190506101cb565b505050509050019250505060405180910390f35b61023c6004803603602081101561021057600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610879565b005b34801561024a57600080fd5b50610253610c01565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156102a157600080fd5b506102ce600480360360208110156102b857600080fd5b8101908080359060200190929190505050610c26565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561031c57600080fd5b506103496004803603602081101561033357600080fd5b8101908080359060200190929190505050610c64565b005b610353610d40565b005b34801561036157600080fd5b5061036a61144f565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b838110156103ad578082015181840152602081019050610392565b505050509050019250505060405180910390f35b3480156103cd57600080fd5b50610410600480360360208110156103e457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506114dd565b6040518082815260200191505060405180910390f35b34801561043257600080fd5b5061045f6004803603602081101561044957600080fd5b8101908080359060200190929190505050611609565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156104ad57600080fd5b506104f0600480360360208110156104c457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611647565b6040518082815260200191505060405180910390f35b6107846004803603608081101561051c57600080fd5b810190808035906020019064010000000081111561053957600080fd5b82018360208201111561054b57600080fd5b8035906020019184600183028401116401000000008311171561056d57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156105d057600080fd5b8201836020820111156105e257600080fd5b8035906020019184600183028401116401000000008311171561060457600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192908035906020019064010000000081111561066757600080fd5b82018360208201111561067957600080fd5b8035906020019184600183028401116401000000008311171561069b57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156106fe57600080fd5b82018360208201111561071057600080fd5b8035906020019184600183028401116401000000008311171561073257600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050611668565b005b600060149054906101000a900460ff1681565b60015481565b6000600260008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001549050919050565b6060600380548060200260200160405190810160405280929190818152602001828054801561086f57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610825575b5050505050905090565b600060149054906101000a900460ff1615151561089557600080fd5b6000341115156108a457600080fd5b600260008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff1615156108ff57600080fd5b6000600260008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090503481600001600082825401925050819055508060080160408051908101604052803373ffffffffffffffffffffffffffffffffffffffff168152602001348152509080600181540180825580915050906001820390600052602060002090600202016000909192909190915060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060208201518160010155505050610a0f81600101546119bd565b80600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201548160000155600182015481600101556002820160009054906101000a900460ff168160020160006101000a81548160ff021916908315150217905550600382018160030160008201816000019080546001816001161561010002031660029004610abf929190611d19565b5060018201816001019080546001816001161561010002031660029004610ae7929190611d19565b506002820154816002015560038201816003019080546001816001161561010002031660029004610b19929190611d19565b5060048201816004019080546001816001161561010002031660029004610b41929190611d19565b5050506008820181600801908054610b5a929190611da0565b509050507f484413c33463e7d976e3ded2a85b0c31610d0906977df2cd9cbd6d12a6872a84338334604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a15050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600481815481101515610c3557fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060146101000a81548160ff02191690831515021790555080600181905550600154600481610cd59190611e69565b506003600090806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050565b600060149054906101000a900460ff16151515610d5c57600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610db757600080fd5b6001600060146101000a81548160ff0219169083151502179055506000600180540190505b60038054905081101561138857610df1611e95565b60026000600384815481101515610e0457fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060a0604051908101604052908160008201548152602001600182015481526020016002820160009054906101000a900460ff161515151581526020016003820160a06040519081016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610f4e5780601f10610f2357610100808354040283529160200191610f4e565b820191906000526020600020905b815481529060010190602001808311610f3157829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610ff05780601f10610fc557610100808354040283529160200191610ff0565b820191906000526020600020905b815481529060010190602001808311610fd357829003601f168201915b5050505050815260200160028201548152602001600382018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561109c5780601f106110715761010080835404028352916020019161109c565b820191906000526020600020905b81548152906001019060200180831161107f57829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561113e5780601f106111135761010080835404028352916020019161113e565b820191906000526020600020905b81548152906001019060200180831161112157829003601f168201915b505050505081525050815260200160088201805480602002602001604051908101604052809291908181526020016000905b8282101561120257838290600052602060002090600202016040805190810160405290816000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200160018201548152505081526020019060010190611170565b5050505081525050905060008090505b8160800151518310156113795781608001518181518110151561123157fe5b906020019060200201516000015173ffffffffffffffffffffffffffffffffffffffff166108fc83608001518381518110151561126a57fe5b90602001906020020151602001519081150290604051600060405180830381858888f19350505050151561129d57600080fd5b60008260800151828151811015156112b157fe5b9060200190602002015160200181815250507fbb28353e4598c3b9199101a66e0989549b659a59a54d2c27fbb183f1932c8e6d8260800151828151811015156112f657fe5b906020019060200201516000015183608001518381518110151561131657fe5b9060200190602002015160200151604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a18280600101935050611212565b50508080600101915050610ddc565b506000600190505b60015481111580156113a6575060038054905081105b1561144c576003818154811015156113ba57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166004600183038154811015156113f757fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508080600101915050611390565b50565b606060048054806020026020016040519081016040528092919081815260200182805480156114d357602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311611489575b5050505050905090565b600080600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001015490508273ffffffffffffffffffffffffffffffffffffffff1660038281548110151561154b57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614151561159557fe5b7f6510d4dce8156a73fb1e59829fc119ba136d8fd146b5dbda51e017781c18dfca8382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a180915050919050565b60038181548110151561161857fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008173ffffffffffffffffffffffffffffffffffffffff16319050919050565b600060149054906101000a900460ff1615151561168457600080fd5b60003411151561169357600080fd5b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160009054906101000a900460ff16156116ed57600080fd5b6116f5611ece565b60a06040519081016040528086815260200185815260200134815260200184815260200183815250905034600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000018190555080600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060030160008201518160000190805190602001906117c5929190611efe565b5060208201518160010190805190602001906117e2929190611efe565b50604082015181600201556060820151816003019080519060200190611809929190611efe565b506080820151816004019080519060200190611826929190611efe565b509050506001600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060020160006101000a81548160ff02191690831515021790555060033390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160038054905003600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018190555061194b6001600380549050036119bd565b7f6578ae22e2e9b39b41597df980dc4daa54b024101d07dd0fffc49b85372fa4293334604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a15050505050565b60008190505b6001811115611d1557600260006003600184038154811015156119e257fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000015460026000600384815481101515611a5e57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000154111515611ad357611d15565b6001810360026000600384815481101515611aea57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101819055508060026000600360018503815481101515611b6d57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101819055506000600382815481101515611bea57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600360018303815481101515611c2957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600383815481101515611c6357fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600360018403815481101515611cbe57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550508080600190039150506119c3565b5050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10611d525780548555611d8f565b82800160010185558215611d8f57600052602060002091601f016020900482015b82811115611d8e578254825591600101919060010190611d73565b5b509050611d9c9190611f7e565b5090565b828054828255906000526020600020906002028101928215611e585760005260206000209160020282015b82811115611e575782826000820160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060018201548160010155505091600201919060020190611dcb565b5b509050611e659190611fa3565b5090565b815481835581811115611e9057818360005260206000209182019101611e8f9190611f7e565b5b505050565b610120604051908101604052806000815260200160008152602001600015158152602001611ec1611ff1565b8152602001606081525090565b60a06040519081016040528060608152602001606081526020016000815260200160608152602001606081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10611f3f57805160ff1916838001178555611f6d565b82800160010185558215611f6d579182015b82811115611f6c578251825591602001919060010190611f51565b5b509050611f7a9190611f7e565b5090565b611fa091905b80821115611f9c576000816000905550600101611f84565b5090565b90565b611fee91905b80821115611fea57600080820160006101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055600182016000905550600201611fa9565b5090565b90565b60a0604051908101604052806060815260200160608152602001600081526020016060815260200160608152509056fea165627a7a7230582032870ff86b649dd03d0f38291b8f94361abac3b74033bc24ec588c40816b570e0029")
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
	t.Error("Before:", balanceBefore)

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
	_, _, err = sample_kvm.Call(address, endVote, &sample_kvm.Config{State: state, Value: big.NewInt(100000000000), Origin: owner})
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
	t.Error("After:", balanceAfter)
	amount := big.NewInt(0)
	amount = amount.Sub(balanceAfter, balanceBefore)
	if amount.Cmp(big.NewInt(100)) != 0 {
		t.Error("Expected 100 coins, got", amount)
	}

}
