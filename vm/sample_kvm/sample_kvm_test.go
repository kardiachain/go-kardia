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

func BenchmarkCall(b *testing.B) {
	var definition = `[{"constant":true,"inputs":[],"name":"seller","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"abort","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"value","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[],"name":"refund","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"buyer","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmReceived","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"state","outputs":[{"name":"","type":"uint8"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmPurchase","outputs":[],"type":"function"},{"inputs":[],"type":"constructor"},{"anonymous":false,"inputs":[],"name":"Aborted","type":"event"},{"anonymous":false,"inputs":[],"name":"PurchaseConfirmed","type":"event"},{"anonymous":false,"inputs":[],"name":"ItemReceived","type":"event"},{"anonymous":false,"inputs":[],"name":"Refunded","type":"event"}]`

	var code = common.Hex2Bytes("6060604052361561006c5760e060020a600035046308551a53811461007457806335a063b4146100865780633fa4f245146100a6578063590e1ae3146100af5780637150d8ae146100cf57806373fac6f0146100e1578063c19d93fb146100fe578063d696069714610112575b610131610002565b610133600154600160a060020a031681565b610131600154600160a060020a0390811633919091161461015057610002565b61014660005481565b610131600154600160a060020a039081163391909116146102d557610002565b610133600254600160a060020a031681565b610131600254600160a060020a0333811691161461023757610002565b61014660025460ff60a060020a9091041681565b61013160025460009060ff60a060020a9091041681146101cc57610002565b005b600160a060020a03166060908152602090f35b6060908152602090f35b60025460009060a060020a900460ff16811461016b57610002565b600154600160a060020a03908116908290301631606082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f72c874aeff0b183a56e2b79c71b46e1aed4dee5e09862134b8821ba2fddbf8bf9250a150565b80546002023414806101dd57610002565b6002805460a060020a60ff021973ffffffffffffffffffffffffffffffffffffffff1990911633171660a060020a1790557fd5d55c8a68912e9a110618df8d5e2e83b8d83211c57a8ddd1203df92885dc881826060a15050565b60025460019060a060020a900460ff16811461025257610002565b60025460008054600160a060020a0390921691606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517fe89152acd703c9d8c7d28829d443260b411454d45394e7995815140c8cbcbcf79250a150565b60025460019060a060020a900460ff1681146102f057610002565b6002805460008054600160a060020a0390921692909102606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f8616bbbbad963e4e65b1366f1d75dfb63f9e9704bbbf91fb01bec70849906cf79250a15056")

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		b.Fatal(err)
	}

	cpurchase, err := abi.Pack("confirmPurchase")
	if err != nil {
		b.Fatal(err)
	}
	creceived, err := abi.Pack("confirmReceived")
	if err != nil {
		b.Fatal(err)
	}
	refund, err := abi.Pack("refund")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 400; j++ {
			Execute(code, cpurchase, nil)
			Execute(code, creceived, nil)
			Execute(code, refund, nil)
		}
	}
}

// testing call a simple smart contract return static value
func TestSimpleCalcContract(t *testing.T) {
	var definition = `[
	{
		"constant": true,
		"inputs": [],
		"name": "dm",
		"outputs": [
			{
				"name": "",
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
				"name": "x",
				"type": "uint8"
			},
			{
				"name": "y",
				"type": "uint8"
			}
		],
		"name": "plus",
		"outputs": [
			{
				"name": "",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	var code = common.Hex2Bytes("60806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416636f98b63c8114604d578063916f4029146075575b600080fd5b348015605857600080fd5b50605f6093565b6040805160ff9092168252519081900360200190f35b348015608057600080fd5b50605f60ff600435811690602435166098565b600a90565b01905600a165627a7a7230582042b9a30f60b4653c09c79c16d1976b003e7f6965ee65d924893fe488d87234c10029")
	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	// cplus, err := abi.Pack("plus", uint8(5), uint8(6))
	cplus, err := abi.Pack("dm")
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Execute(code, cplus, nil)

	if err != nil {
		t.Fatal(err)
	}
	//print(uint8(ret))cplus, err := abi.Pack("plus", uint8(5), uint8(6))

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}

}

// testing call a simple smart contract return sum of 2 parameters
func TestSimpleCalcContract1(t *testing.T) {
	var definition = `[
	{
		"constant": true,
		"inputs": [],
		"name": "dm",
		"outputs": [
			{
				"name": "",
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
				"name": "x",
				"type": "uint8"
			},
			{
				"name": "y",
				"type": "uint8"
			}
		],
		"name": "plus",
		"outputs": [
			{
				"name": "",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	var code = common.Hex2Bytes("60806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416636f98b63c8114604d578063916f4029146075575b600080fd5b348015605857600080fd5b50605f6093565b6040805160ff9092168252519081900360200190f35b348015608057600080fd5b50605f60ff600435811690602435166098565b600a90565b01905600a165627a7a7230582042b9a30f60b4653c09c79c16d1976b003e7f6965ee65d924893fe488d87234c10029")
	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	// cplus, err := abi.Pack("plus", uint8(5), uint8(6))
	cplus, err := abi.Pack("plus", uint8(1), uint8(2))
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Execute(code, cplus, nil)

	if err != nil {
		t.Fatal(err)
	}
	//print(uint8(ret))cplus, err := abi.Pack("plus", uint8(5), uint8(6))

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(3)) != 0 {
		t.Error("Expected 3, got", num)
	}
}

// the following test case fails now
// as when the contract get executed, it stops at REVERT opcodes
/*
func TestCreateContract(t *testing.T){
	var input = common.Hex2Bytes("60806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416636f98b63c8114604d578063916f4029146075575b600080fd5b348015605857600080fd5b50605f6093565b6040805160ff9092168252519081900360200190f35b348015608057600080fd5b50605f60ff600435811690602435166098565b600a90565b01905600a165627a7a7230582042b9a30f60b4653c09c79c16d1976b003e7f6965ee65d924893fe488d87234c10029")
	var code, add, _, err = Create(input, nil)
	if nil != err {
		t.Fatal(err)
	}
	print("code", code)
	print("add", add.String())
}
*/

// test create smart contract by specifying code at an address,
// call contract address and get return value
func TestCreateContractWithInput(t *testing.T){
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")
	var code = common.Hex2Bytes("60806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416636f98b63c8114604d578063916f4029146075575b600080fd5b348015605857600080fd5b50605f6093565b6040805160ff9092168252519081900360200190f35b348015608057600080fd5b50605f60ff600435811690602435166098565b600a90565b01905600a165627a7a7230582042b9a30f60b4653c09c79c16d1976b003e7f6965ee65d924893fe488d87234c10029")
	state.SetCode(address, code)
	var definition = `[
	{
		"constant": true,
		"inputs": [],
		"name": "dm",
		"outputs": [
			{
				"name": "",
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
				"name": "x",
				"type": "uint8"
			},
			{
				"name": "y",
				"type": "uint8"
			}
		],
		"name": "plus",
		"outputs": [
			{
				"name": "",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	// cplus, err := abi.Pack("plus", uint8(5), uint8(6))
	cplus, err := abi.Pack("plus", uint8(5), uint8(2))
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Call(address, cplus, &Config{State: state})

	if err != nil {
		t.Fatal(err)
	}
	//print(uint8(ret))cplus, err := abi.Pack("plus", uint8(5), uint8(6))

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(7)) != 0 {
		t.Error("Expected 7, got", num)
	}
}

// the following test case is failing because invalid RETURN opcode
// https://github.com/kardiachain/go-kardia/issues/63
/*func TestCreateContractWithStorage(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0b")
	var code = common.Hex2Bytes("60806040526000805460ff1916600517905534801561001d57600080fd5b5060a08061002c6000396000f300608060405260043610603e5763ffffffff7c0100000000000000000000000000000000000000000000000000000000600035041663de29278981146043575b600080fd5b348015604e57600080fd5b506055606b565b6040805160ff9092168252519081900360200190f35b60005460ff16905600a165627a7a72305820999359c62faede362eee89d721a94faf3d1bb1ac48b98b9772641a65e703b4e50029")
	state.SetCode(address, code)
	var definition = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getResult",
		"outputs": [
			{
				"name": "",
				"type": "uint8"
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
	}
]`
	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	// cplus, err := abi.Pack("plus", uint8(5), uint8(6))
	getResult, err := abi.Pack("getResult")
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Call(address, getResult, &Config{State: state})
	result1 := new(big.Int).SetBytes(ret)
	if result1.Cmp(big.NewInt(5)) != 0 {
		t.Error("Expected 5, got", result1)
	}
}*/

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