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
func TestExecuteSimpleCounterSmc(t *testing.T) {
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
