package sample_kvm

import (
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
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

// Test behaviour of master exchange contract to exchange ETH <-> NEO
// Please find solidity source code in smc/Exchange.sol
func TestExecuteMasterExchangeContract(t *testing.T) {
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")

	// Add runtime_bytecode for Exchange.sol to execute the smc:
	var code = common.Hex2Bytes("6080604052600436106100775763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630a0306b1811461007c578063323a9243146100a357806344af18e8146100bd5780636e63987d146100d557806386dca334146100ed578063fa8513de14610105575b600080fd5b34801561008857600080fd5b5061009161011a565b60408051918252519081900360200190f35b3480156100af57600080fd5b506100bb600435610139565b005b3480156100c957600080fd5b506100bb600435610154565b3480156100e157600080fd5b506100bb60043561016f565b3480156100f957600080fd5b506100bb60043561017a565b34801561011157600080fd5b50610091610185565b600060015460005411156101315750600154610136565b506000545b90565b60015481111561014857600080fd5b60018054919091039055565b60005481111561016357600080fd5b60008054919091039055565b600180549091019055565b600080549091019055565b60008054600154111561019b5750600054610136565b50600154905600a165627a7a72305820f07bf8b0278729f61585fdeb608ea6ab12a34ae7871ea92bfd2f4199cc5bfd0d0029")
	state.SetCode(address, code)
	var definition = `[
						{"constant": false,"inputs": [{"name": "eth","type": "uint256"}],"name": "matchEth","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
						{"constant": false,"inputs": [{"name": "neo","type": "uint256"}],"name": "matchNeo","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
						{"constant": false,"inputs": [{"name": "eth","type": "uint256"}],"name": "removeEth","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
						{"constant": false,"inputs": [{"name": "neo","type": "uint256"}],"name": "removeNeo","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
						{"constant": true,"inputs": [],"name": "getEthToSend","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"},
						{"constant": true,"inputs": [],"name": "getNeoToSend","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"}
					]`
	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	// Deposit 10 ETH to exchange for 10 NEO
	matchEthInput, err := abi.Pack("matchEth", big.NewInt(10))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Call(address, matchEthInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	// Deposit 5 NEO to exchange for 5 ETH
	matchNeoInput, err := abi.Pack("matchNeo", big.NewInt(5))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Call(address, matchNeoInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	// Get number of matching ETH quantity
	getEthInput, err := abi.Pack("getEthToSend")
	result, _, err := Call(address, getEthInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	var ethAmount = new(big.Int).SetBytes(result)
	if ethAmount.Cmp(big.NewInt(5)) != 0 {
		t.Error("Expected 5, got", ethAmount)
	}

	// Assume ETH has been release successfully, update the order list
	removeEthInput, err := abi.Pack("removeEth", big.NewInt(5))
	_, _, err = Call(address, removeEthInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	removeNeoInput, err := abi.Pack("removeNeo", big.NewInt(5))
	_, _, err = Call(address, removeNeoInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	// Get number of matching ETH quantity again, it should be 0 because no more NEO order is match
	getEthInput, err = abi.Pack("getEthToSend")
	result, _, err = Call(address, getEthInput, &Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	ethAmount = new(big.Int).SetBytes(result)
	if ethAmount.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0, got", ethAmount)
	}
}

// Test call a contract from inside another contract
// The source code of 2 contracts are in smc/intersmc.
// Contract A is callee, B is caller
func TestExecuteInterContract(t *testing.T) {
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	addressA := common.HexToAddress("0x0a")
	addressB := common.HexToAddress("0x0b")
	// Add runtime_bytecode
	// Contract B
	var codeA = common.Hex2Bytes("60806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166373d4a13a8114604d578063da358a3c146071575b600080fd5b348015605857600080fd5b50605f6088565b60408051918252519081900360200190f35b348015607c57600080fd5b506086600435608e565b005b60005481565b6000555600a165627a7a72305820408349f58cb50ba37a5c1f89b5c4dacc1077449c09ab590360ea2866dcbc0a460029")
	state.SetCode(addressA, codeA)
	var definitionA = `[{"constant":true,"inputs":[],"name":"data","outputs":[{"name":"","type":"int256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_data","type":"int256"}],"name":"setData","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"}]`

	abiA, errParseA := abi.JSON(strings.NewReader(definitionA))
	if errParseA != nil {
		t.Fatal(errParseA)
	}
	// Contract B
	var codeB = common.Hex2Bytes("60806040526004361061004b5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416635adc75af8114610050578063d32fe93414610077575b600080fd5b34801561005c57600080fd5b506100656100aa565b60408051918252519081900360200190f35b34801561008357600080fd5b506100a873ffffffffffffffffffffffffffffffffffffffff600435166024356100b0565b005b60005481565b60008290508073ffffffffffffffffffffffffffffffffffffffff1663da358a3c836040518263ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180828152602001915050600060405180830381600087803b15801561012457600080fd5b505af1158015610138573d6000803e3d6000fd5b5050506000929092555050505600a165627a7a723058205824e91fcb7a1f7034282bc72a1641ff48abe2e8a99e0ef68c941da88fdc21a30029")
	state.SetCode(addressB, codeB)
	var definitionB = `[{"constant":true,"inputs":[],"name":"datab","outputs":[{"name":"","type":"int256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"aAddr","type":"address"},{"name":"_data","type":"int256"}],"name":"testData","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"}]`
	abiB, errParseB := abi.JSON(strings.NewReader(definitionB))
	if errParseB != nil {
		t.Fatal(errParseB)
	}

	// Add default data to A to be 100
	setData, errPackSetData := abiA.Pack("setData", big.NewInt(100))
	if errPackSetData != nil {
		t.Fatal(errPackSetData)
	}
	_, _, errCallSetData := Call(addressA, setData, &Config{State: state})
	if errCallSetData != nil {
		t.Error(errCallSetData)
	}

	getData, errPackGetData := abiA.Pack("data")

	if errPackGetData != nil {
		t.Fatal(errPackGetData)
	}

	rgetData, _, errCallGetData := Call(addressA, getData, &Config{State: state})
	if errCallSetData != nil {
		t.Error(errCallGetData)
	}

	getValue := new(big.Int).SetBytes(rgetData)
	// Check value of A to check whether it's 100
	if getValue.Cmp(big.NewInt(100)) != 0 {
		t.Error("Error get value, expected 100 got ", getValue)
	}

	// Try to test set Data to A from B, data is set to be 10
	testData, errTestData := abiB.Pack("testData", addressA, big.NewInt(10))
	if errTestData != nil {
		t.Fatal(errTestData)
	}

	_, _, errCallTestData := Call(addressB, testData, &Config{State: state})

	if errCallTestData != nil {
		t.Fatal(errCallTestData)
	}
	// Now we call getData from A again, to check whether it's set to 10
	rgetData2, _, errCallGetData := Call(addressA, getData, &Config{State: state})

	getValue = new(big.Int).SetBytes(rgetData2)
	if errCallGetData != nil {
		t.Fatal(errCallGetData)
	}
	// Data should be 10 after be set from B
	if getValue.Cmp(big.NewInt(10)) != 0 {
		t.Error("Error get value, expected 100 got ", getValue)
	}
}