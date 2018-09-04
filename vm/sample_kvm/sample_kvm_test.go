package sample_kvm

import (
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/state"
	kaidb "github.com/kardiachain/go-kardia/storage"
	"github.com/kardiachain/go-kardia/vm"
)

func TestDefaults(t *testing.T) {
	cfg := new(Config)
	setDefaults(cfg)

	if cfg.Time == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.GasLimit == 0 {
		t.Error("didn't expect gaslimit to be zero")
	}
	if cfg.GasPrice == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.Value == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.GetHashFn == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.BlockHeight != 0 {
		t.Error("expected block number to be 0")
	}
}

func TestKVM(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("crashed with: %v", r)
		}
	}()

	Execute([]byte{
		byte(vm.TIMESTAMP),
		byte(vm.GASLIMIT),
		byte(vm.PUSH1),
		byte(vm.ORIGIN),
		byte(vm.BLOCKHASH),
		byte(vm.COINBASE),
	}, nil, nil)
}

func TestExecute(t *testing.T) {
	ret, _, err := Execute([]byte{
		byte(vm.PUSH1), 10,
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	}, nil, nil)
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

func TestCall(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")
	state.SetCode(address, []byte{
		byte(vm.PUSH1), 10,
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	})

	ret, _, err := Call(address, nil, &Config{State: state})
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

// Simple counter smart contract to be used for below tests:
/*
- counter.sol:
	pragma solidity ^0.4.24;
	contract Counter {
    	uint8 count;
    	function set(uint8 x) public {
        	count = x;
    	}
    	function get() public view returns (uint8) {
        	return count;
    	}
	}

- compiler: remix: 0.4.24+commit.e67f0147.Emscripten.clang
*/

// Test creating a simple smart contract on KVM.
// Note: Create uses the raw bytecode as generated from compiler
func TestCreateSimpleCounterSmc(t *testing.T) {
	// Add bytecode for counter.sol to create the smc:
	var input = common.Hex2Bytes("608060405234801561001057600080fd5b5060da8061001f6000396000f30060806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166324b8ba5f8114604d5780636d4ce63c146067575b600080fd5b348015605857600080fd5b50606560ff60043516608f565b005b348015607257600080fd5b50607960a5565b6040805160ff9092168252519081900360200190f35b6000805460ff191660ff92909216919091179055565b60005460ff16905600a165627a7a723058206cc1a54f543612d04d3f16b0bbb49e9ded9ccf6d47f7789fe3577260346ed44d0029")
	_, _, _, err := Create(input, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// Test executing the counter smart contract on KVM
// Note: Call uses the runtime_bytecode from the compiler, unlike the raw bytecode as in the previous unit test
func TestCallSimpleCounterSmc(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")

	// Add runtime_bytecode for counter.sol to execute the smc:
	var code = common.Hex2Bytes("60806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166324b8ba5f8114604d5780636d4ce63c146067575b600080fd5b348015605857600080fd5b50606560ff60043516608f565b005b348015607257600080fd5b50607960a5565b6040805160ff9092168252519081900360200190f35b6000805460ff191660ff92909216919091179055565b60005460ff16905600a165627a7a723058206cc1a54f543612d04d3f16b0bbb49e9ded9ccf6d47f7789fe3577260346ed44d0029")
	state.SetCode(address, code)
	var definition = `[
		{"constant":false,"inputs":[{"name":"x","type":"uint8"}],"name":"set","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
		{"constant":true,"inputs":[],"name":"get","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}
	]`

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	// Sets counter to 5
	set, err := abi.Pack("set", uint8(5))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Call(address, set, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	// Gets counter and verifies it is 5
	get, err := abi.Pack("get")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := Call(address, get, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(5)) != 0 {
		t.Error("Expected 5, got", num)
	}
}

func TestChangeBalance(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	var address = common.HexToAddress("0x0b")
	state.CreateAccount(address)
	state.AddBalance(address, big.NewInt(500))

	var balance = state.GetBalance(address)
	if balance.Cmp(big.NewInt(500)) != 0 {
		t.Error("error setting balance, expect 500, got", balance)
	}

	state.SubBalance(address, big.NewInt(100))
	balance = state.GetBalance(address)
	if balance.Cmp(big.NewInt(400)) != 0 {
		t.Error("error subtract balance, expect 400, got", balance)
	}
}

func TestCallSmcDeductBalance(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	var sender = common.HexToAddress("0x0b")
	state.CreateAccount(sender)
	state.AddBalance(sender, big.NewInt(500))

	address := common.HexToAddress("0x0a")

	state.SetCode(address, []byte{
		byte(vm.PUSH1), 10,
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	})
	ret, _, err := Call(address, nil, &Config{State: state, Origin: sender, Value: big.NewInt(50)})
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
	var sender_balance = state.GetBalance(sender)
	if sender_balance.Cmp(big.NewInt(450)) != 0 {
		t.Error("Invalid remaining balance, expect 450, got", sender_balance)
	}
	var contract_balance = state.GetBalance(address)
	if contract_balance.Cmp(big.NewInt(50)) != 0 {
		t.Error("Invalid contract balance, expect 50, got", contract_balance)
	}
}

// Simple voting smart contract to be used for below tests:
/*
- ballot.sol:
	pragma solidity ^0.4.0;
	contract Ballot {

		struct Voter {
			bool voted;
			uint8 vote;
		}
		struct Proposal {
			uint voteCount;
		}

		mapping(address => Voter) voters;
		Proposal[4] proposals;

		/// Give a single vote to proposal $(toProposal).
		function vote(uint8 toProposal) public {
			Voter storage sender = voters[msg.sender];
			if (sender.voted || toProposal >= proposals.length) return;
			sender.voted = true;
			sender.vote = toProposal;
			proposals[toProposal].voteCount += 1;
		}

		function getVote(uint8 toProposal) public view returns (uint) {
			if (toProposal >= proposals.length) return 0;
			return proposals[toProposal].voteCount;
		}

		function winningProposal() public view returns (uint8 _winningProposal) {
			uint256 winningVoteCount = 0;
			for (uint8 prop = 0; prop < proposals.length; prop++)
				if (proposals[prop].voteCount > winningVoteCount) {
					winningVoteCount = proposals[prop].voteCount;
					_winningProposal = prop;
				}
		}
	}

- compiler: remix: 0.4.24+commit.e67f0147.Emscripten.clang
*/

// Test executing the voting smart contract on KVM
// Note: Call uses the runtime_bytecode from the compiler, unlike the raw bytecode as in the previous unit test
func TestExecuteVoteSmc(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")

	// Add runtime_bytecode for ballot.sol to execute the smc:
	var code = common.Hex2Bytes("608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029")
	state.SetCode(address, code)
	var definition = `[
	{
		"constant": true,
		"inputs": [
			{
				"name": "toProposal",
				"type": "uint8"
			}
		],
		"name": "getVote",
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
		"name": "winningProposal",
		"outputs": [
			{
				"name": "_winningProposal",
				"type": "uint8"
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
				"name": "toProposal",
				"type": "uint8"
			}
		],
		"name": "vote",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	//get init winning proposal
	get, err := abi.Pack("winningProposal")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := Call(address, get, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0, got", num)
	}
	vote, err := abi.Pack("vote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, vote, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	result, _, err = Call(address, get, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got", num)
	}
}

// Test executing the voting smart contract on KVM using different senders
// Note: Call uses the runtime_bytecode from the compiler, unlike the raw bytecode as in the previous unit test
func TestExecuteVoteSmcMultipleTime(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")

	// Add runtime_bytecode for ballot.sol to execute the smc:
	var code = common.Hex2Bytes("608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029")
	state.SetCode(address, code)
	var definition = `[
	{
		"constant": true,
		"inputs": [
			{
				"name": "toProposal",
				"type": "uint8"
			}
		],
		"name": "getVote",
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
		"name": "winningProposal",
		"outputs": [
			{
				"name": "_winningProposal",
				"type": "uint8"
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
				"name": "toProposal",
				"type": "uint8"
			}
		],
		"name": "vote",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	//get init winning proposal, should be 0
	get, err := abi.Pack("winningProposal")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := Call(address, get, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0, got", num)
	}
	//create first vote for second candidate , should be successful
	vote, err := abi.Pack("vote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	var sender1 = common.HexToAddress("0x0b")
	state.CreateAccount(sender1)
	state.AddBalance(sender1, big.NewInt(500))
	result, _, err = Call(address, vote, &Config{State: state, Origin: sender1})
	if err != nil {
		t.Fatal(err)
	}

	// now we get count of second candidate , should be 1
	getProposal, err := abi.Pack("getVote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, getProposal, &Config{State: state, Origin: sender1})

	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got", num)
	}
	//create duplicate vote for 2nd candidate , should be no error
	vote, err = abi.Pack("vote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, vote, &Config{State: state, Origin: sender1})
	if err != nil {
		t.Fatal(err)
	}

	// now we get vote count of candidate 2th, should be 1 because latter vote was invalid
	getProposal, err = abi.Pack("getVote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, getProposal, &Config{State: state})

	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got", num)
	}

	// now we create 2 another accounts to vote for 3rd candidate
	var sender2 = common.HexToAddress("0x0c")
	state.CreateAccount(sender2)
	state.AddBalance(sender2, big.NewInt(500))
	var sender3 = common.HexToAddress("0x0d")
	state.CreateAccount(sender3)
	state.AddBalance(sender3, big.NewInt(500))
	vote, err = abi.Pack("vote", uint8(2))
	result, _, err = Call(address, vote, &Config{State: state, Origin: sender2})
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, vote, &Config{State: state, Origin: sender3})
	if err != nil {
		t.Fatal(err)
	}
	// now we get the winning candidate, it shoud be 3rd candidate
	result, _, err = Call(address, get, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(2)) != 0 {
		t.Error("Expected 2, got", num)
	}
	// get num of vote of 3rd candidate, should be 2 votes
	getProposal, err = abi.Pack("getVote", uint8(2))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = Call(address, getProposal, &Config{State: state})

	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(2)) != 0 {
		t.Error("Expected 2, got", num)
	}
}
