/*
 *  Copyright 2018 KardiaChain
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
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm/sample_kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
)

// Runtime_bytecode for ./InterSmc.sol
var smc_a_code = common.Hex2Bytes("60806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166373d4a13a8114604d578063da358a3c146071575b600080fd5b348015605857600080fd5b50605f6088565b60408051918252519081900360200190f35b348015607c57600080fd5b506086600435608e565b005b60005481565b6000555600a165627a7a72305820408349f58cb50ba37a5c1f89b5c4dacc1077449c09ab590360ea2866dcbc0a460029")
var smc_a_definition = `[
    {
        "constant":true,
        "inputs":[],
        "name":"data",
        "outputs":[
            {
                "name":"",
                "type":"int256"
            }
        ],
        "payable":false,
        "stateMutability":"view",
        "type":"function"
    },
    {
        "constant":false,
        "inputs":[
            {
                "name":"_data",
                "type":"int256"
            }
        ],
        "name":"setData",
        "outputs":[],
        "payable":false,
        "stateMutability":"nonpayable",
        "type":"function"
    }
]`

var smc_b_code = common.Hex2Bytes("60806040526004361061004b5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416635adc75af8114610050578063d32fe93414610077575b600080fd5b34801561005c57600080fd5b506100656100aa565b60408051918252519081900360200190f35b34801561008357600080fd5b506100a873ffffffffffffffffffffffffffffffffffffffff600435166024356100b0565b005b60005481565b60008290508073ffffffffffffffffffffffffffffffffffffffff1663da358a3c836040518263ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180828152602001915050600060405180830381600087803b15801561012457600080fd5b505af1158015610138573d6000803e3d6000fd5b5050506000929092555050505600a165627a7a723058205824e91fcb7a1f7034282bc72a1641ff48abe2e8a99e0ef68c941da88fdc21a30029")
var smc_b_definition = `[
    {
        "constant":true,
        "inputs":[],
        "name":"datab",
        "outputs":[
            {
                "name":"",
                "type":"int256"
            }
        ],
        "payable":false,
        "stateMutability":"view",
        "type":"function"
    },
    {
        "constant":false,
        "inputs":[
            {
                "name":"aAddr",
                "type":"address"
            },
            {
                "name":"_data",
                "type":"int256"
            }
        ],
        "name":"testData",
        "outputs":[],
        "payable":false,
        "stateMutability":"nonpayable",
        "type":"function"
    }
]`

// Test call a contract from inside another contract
// Contract A is callee, B is caller
func TestExecuteInterSmc(t *testing.T) {
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(memorydb.New()))

	// Contract A
	addressA := common.HexToAddress("0x0a")
	state.SetCode(addressA, smc_a_code)
	abiA, errParseA := abi.JSON(strings.NewReader(smc_a_definition))
	if errParseA != nil {
		t.Fatal(errParseA)
	}
	// Contract B
	addressB := common.HexToAddress("0x0b")
	state.SetCode(addressB, smc_b_code)
	abiB, errParseB := abi.JSON(strings.NewReader(smc_b_definition))
	if errParseB != nil {
		t.Fatal(errParseB)
	}

	// Add default data to A to be 100
	setData, errPackSetData := abiA.Pack("setData", big.NewInt(100))
	if errPackSetData != nil {
		t.Fatal(errPackSetData)
	}
	_, _, errCallSetData := sample_kvm.Call(addressA, setData, &sample_kvm.Config{State: state})
	if errCallSetData != nil {
		t.Error(errCallSetData)
	}

	getData, errPackGetData := abiA.Pack("data")
	if errPackGetData != nil {
		t.Fatal(errPackGetData)
	}

	rgetData, _, errCallGetData := sample_kvm.Call(addressA, getData, &sample_kvm.Config{State: state})
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

	_, _, errCallTestData := sample_kvm.Call(addressB, testData, &sample_kvm.Config{State: state})

	if errCallTestData != nil {
		t.Fatal(errCallTestData)
	}
	// Now we call getData from A again, to check whether it's set to 10
	rgetData2, _, errCallGetData := sample_kvm.Call(addressA, getData, &sample_kvm.Config{State: state})

	getValue = new(big.Int).SetBytes(rgetData2)
	if errCallGetData != nil {
		t.Fatal(errCallGetData)
	}
	// Data should be 10 after be set from B
	if getValue.Cmp(big.NewInt(10)) != 0 {
		t.Error("Error get value, expected 100 got ", getValue)
	}
}
