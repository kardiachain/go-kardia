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

/*
pragma solidity ^0.4.0;
contract SimpleCalc {

    function plus(uint8 x, uint8 y) public pure returns (uint8) {
        return x + y;
    }

    function dm() public pure returns (uint8) {
        return 10;
    }
}

Compiled version: 0.4.24+commit.e67f0147.Emscripten.clang
 */
// testing call a simple smart contract return sum of 2 parameters
func TestSimpleCalcContract2(t *testing.T) {
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
	var code = common.Hex2Bytes("6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680636f98b63c14604e578063916f402914607c575b600080fd5b348015605957600080fd5b50606060d0565b604051808260ff1660ff16815260200191505060405180910390f35b348015608757600080fd5b5060b4600480360381019080803560ff169060200190929190803560ff16906020019092919050505060d9565b604051808260ff1660ff16815260200191505060405180910390f35b6000600a905090565b60008183019050929150505600a165627a7a72305820128a36bc73a2b03d029970cd0b39a2ac5a0a992e2dadfc767c72ccc64128a1e00029")
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


	dm, err := abi.Pack("dm")
	if err != nil {
		t.Fatal(err)
	}
	ret1, _, err := Execute(code, dm, nil)

	if err != nil {
		t.Fatal(err)
	}
	//print(uint8(ret))cplus, err := abi.Pack("plus", uint8(5), uint8(6))

	num1 := new(big.Int).SetBytes(ret1)
	if num1.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
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

// simple smc to test
/*
pragma solidity ^0.4.0;
contract SimpleCalc {
    function dm() public pure returns(uint8) {
        return 10;
    }
}
 */
func TestCreateContractWithStorage(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0b")
	//following is code generated by the disasm tool
	/*state.SetCode(address, []byte{
		byte(vm.PUSH1), 0x80, byte(vm.PUSH1), 0x40, byte(vm.MSTORE), byte(vm.PUSH1), 0x4, byte(vm.CALLDATASIZE), byte(vm.LT), byte(vm.PUSH1), 0x3F,
		byte(vm.JUMPI), byte(vm.PUSH1), 0x0, byte(vm.CALLDATALOAD), byte(vm.PUSH29),
		common.Hex2Bytes("0x100000000000000000000000000000000000000000000000000000000")...,
		byte(vm.SWAP1), byte(vm.DIV), byte(vm.PUSH4), 0xFFFFFFFF, byte(vm.AND), byte(vm.DUP1),
		byte(vm.PUSH4), 0x6F98B63C, byte(vm.EQ), byte(vm.PUSH1), 0x44, byte(vm.JUMPI),
		byte(vm.JUMPDEST), byte(vm.PUSH1), 0, byte(vm.DUP1), byte(vm.REVERT), byte(vm.JUMPDEST), byte(vm.CALLVALUE),
		byte(vm.DUP1), byte(vm.ISZERO), byte(vm.PUSH1), 0x4F, byte(vm.JUMPI), byte(vm.PUSH1), 0x0, byte(vm.DUP1),
		byte(vm.REVERT), byte(vm.JUMPDEST),byte(vm.POP),
		byte(vm.PUSH1), 0x56, byte(vm.PUSH1), 0x72, byte(vm.JUMP), byte(vm.JUMPDEST), byte(vm.PUSH1), 0x40, byte(vm.MLOAD),
		byte(vm.DUP1), byte(vm.DUP3), byte(vm.PUSH1), 0xFF, byte(vm.AND), byte(vm.PUSH1), 0xFF, byte(vm.AND), byte(vm.DUP2),
		byte(vm.MSTORE), byte(vm.PUSH1), 0x20, byte(vm.ADD), byte(vm.SWAP2), byte(vm.POP), byte(vm.POP), byte(vm.PUSH1),
		0x40, byte(vm.MLOAD), byte(vm.DUP1), byte(vm.SWAP2), byte(vm.SUB), byte(vm.SWAP1), byte(vm.RETURN), byte(vm.JUMPDEST),
		byte(vm.PUSH1), 0x0, byte(vm.PUSH1), 0xA, byte(vm.SWAP1), byte(vm.POP), byte(vm.SWAP1), byte(vm.JUMP), byte(vm.STOP),
		byte(vm.LOG1), byte(vm.PUSH6), 0x627A7A723058, byte(vm.SHA3), byte(vm.SWAP9), byte(vm.RETURNDATACOPY), byte(vm.PUSH9),
		0xE805176957D333FA7D, byte(vm.PUSH21), 0x18D3CBA446E5F2A66F583C12824E98EA25B4190029})
*/
	var code = make([]byte, 0, 256)
	code = append(code,
		byte(vm.PUSH1), 0x80,byte(vm.PUSH1), 0x40,byte(vm.MSTORE),byte(vm.CALLVALUE),byte(vm.DUP1),byte(vm.ISZERO),byte(vm.PUSH2), 0x0010,byte(vm.JUMPI),byte(vm.PUSH1), 0x00,byte(vm.DUP1),byte(vm.REVERT),byte(vm.JUMPDEST),byte(vm.POP),byte(vm.PUSH1), 0xa7,byte(vm.DUP1),byte(vm.PUSH2), 0x001f,byte(vm.PUSH1), 0x00,byte(vm.CODECOPY),byte(vm.PUSH1), 0x00,byte(vm.RETURN),byte(vm.STOP),byte(vm.PUSH1), 0x80,byte(vm.PUSH1), 0x40,byte(vm.MSTORE),byte(vm.PUSH1), 0x04,byte(vm.CALLDATASIZE),byte(vm.LT),byte(vm.PUSH1), 0x3f,byte(vm.JUMPI),byte(vm.PUSH1), 0x00,byte(vm.CALLDATALOAD),byte(vm.PUSH29))
	code = append(code, common.Hex2Bytes("0x0100000000000000000000000000000000000000000000000000000000")...)
	code = append(code,byte(vm.SWAP1),
		byte(vm.DIV),byte(vm.PUSH4))
	code = append(code, common.Hex2Bytes("0xffffffff")...)
	code = append(code,byte(vm.AND),byte(vm.DUP1),byte(vm.PUSH4))
	code = append(code, common.Hex2Bytes("0x6f98b63c")...)
	code = append(code ,byte(vm.EQ),byte(vm.PUSH1), 0x44,
		byte(vm.JUMPI),byte(vm.JUMPDEST),byte(vm.PUSH1), 0x00,byte(vm.DUP1),byte(vm.REVERT),byte(vm.JUMPDEST),byte(vm.CALLVALUE),byte(vm.DUP1),
		byte(vm.ISZERO),byte(vm.PUSH1), 0x4f,byte(vm.JUMPI),byte(vm.PUSH1), 0x00,byte(vm.DUP1),byte(vm.REVERT),byte(vm.JUMPDEST),byte(vm.POP),
		byte(vm.PUSH1), 0x56,byte(vm.PUSH1), 0x72,byte(vm.JUMP),byte(vm.JUMPDEST),byte(vm.PUSH1), 0x40,byte(vm.MLOAD),byte(vm.DUP1),byte(vm.DUP3),
		byte(vm.PUSH1), 0xff,byte(vm.AND),byte(vm.PUSH1), 0xff,byte(vm.AND),byte(vm.DUP2),byte(vm.MSTORE),byte(vm.PUSH1), 0x20,byte(vm.ADD),
		byte(vm.SWAP2),byte(vm.POP),byte(vm.POP),byte(vm.PUSH1), 0x40,byte(vm.MLOAD),byte(vm.DUP1),byte(vm.SWAP2),byte(vm.SUB),byte(vm.SWAP1),
		byte(vm.RETURN),byte(vm.JUMPDEST),byte(vm.PUSH1), 0x00,byte(vm.PUSH1), 0x0a,byte(vm.SWAP1),byte(vm.POP),byte(vm.SWAP1),byte(vm.JUMP),
		byte(vm.STOP),byte(vm.LOG1),byte(vm.PUSH6))
	code = append(code, common.Hex2Bytes("0x627a7a723058")...)
	code = append(code,byte(vm.SHA3),byte(vm.SWAP9),byte(vm.RETURNDATACOPY),byte(vm.PUSH9))
	code = append(code, common.Hex2Bytes("0xe805176957d333fa7d")...)
	code = append(code, byte(vm.PUSH21))
	code = append(code, common.Hex2Bytes("0x18d3cba446e5f2a66f583c12824e98ea25b4190029")...)

	state.SetCode(address, code)
	//following is code generated by remix
	//state.SetCode(address, common.Hex2Bytes("608060405234801561001057600080fd5b5060a78061001f6000396000f300608060405260043610603f576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680636f98b63c146044575b600080fd5b348015604f57600080fd5b5060566072565b604051808260ff1660ff16815260200191505060405180910390f35b6000600a9050905600a165627a7a72305820983e68e805176957d333fa7d7418d3cba446e5f2a66f583c12824e98ea25b4190029"))
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
	getResult, err := abi.Pack("dm")
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Call(address, getResult, &Config{State: state})
	result1 := new(big.Int).SetBytes(ret)
	if result1.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", result1)
	}
}
/*

 */
func TestSimpleCounter(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0c")
	state.SetCode(address, common.Hex2Bytes("608060405260056000806101000a81548160ff021916908360ff16021790555034801561002b57600080fd5b5061011c8061003b6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631c6ec4ac14604e578063371303c0146095575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060a9565b604051808260ff1660ff16815260200191505060405180910390f35b34801560a057600080fd5b5060a760c1565b005b60008060009054906101000a900460ff169050919050565b60016000808282829054906101000a900460ff160192506101000a81548160ff021916908360ff1602179055505600a165627a7a723058201a15742f83edaa9025630fd001fdf13d1bdc6e0a6f615966b221c9cbc0d0b9e90029"))
	var def = `[
	{
		"constant": true,
		"inputs": [
			{
				"name": "c",
				"type": "uint8"
			}
		],
		"name": "display",
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
		"constant": false,
		"inputs": [],
		"name": "inc",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
	abi, err := abi.JSON(strings.NewReader(def))
	if err != nil {
		t.Fatal(err)
	}

	getResult, err := abi.Pack("display", uint8(10))
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Call(address, getResult, &Config{State: state})

	result1 := new(big.Int).SetBytes(ret)
	if result1.Cmp(big.NewInt(5)) != 0 {
		t.Error("Expected 5, got", result1)
	}
}

func TestReflect(t *testing.T) {
	state, _ := state.New(common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0d")
	state.SetCode(address, common.Hex2Bytes("608060405234801561001057600080fd5b5060a08061001f6000396000f300608060405260043610603e5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631c6ec4ac81146043575b600080fd5b348015604e57600080fd5b50605b60ff600435166071565b6040805160ff9092168252519081900360200190f35b905600a165627a7a72305820a58011575df315bc395a325e71075ecefa50558944dc093ab0d5d9bbf7b7bba10029"))
	var def = `[
	{
		"inputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "constructor"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "c",
				"type": "uint8"
			}
		],
		"name": "display",
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
	abi, err := abi.JSON(strings.NewReader(def))
	if err != nil {
		t.Fatal(err)
	}

	getResult, err := abi.Pack("display", uint8(10))
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := Call(address, getResult, &Config{State: state})
	println("Length :", len(ret))
	println(ret)
	result1 := new(big.Int).SetBytes(ret)
	if result1.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", result1)
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