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
	"github.com/kardiachain/go-kardia/kai/state"
    kaidb "github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/kvm/sample_kvm"
	"strings"
	"testing"
)

// Runtime_bytecode for ./Ballot.sol
var ballot_smc_code = common.Hex2Bytes("608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029")
var ballot_smc_definition = `[
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

func TestBallotSmcExecuteVote(t *testing.T) {
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")
	state.SetCode(address, ballot_smc_code)

	abi, err := abi.JSON(strings.NewReader(ballot_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// get init winning proposal
	get, err := abi.Pack("winningProposal")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
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
	result, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}

	result, _, err = sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got", num)
	}
}

func TestBallotSmcExecuteVoteMultipleTime(t *testing.T) {
	state, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	address := common.HexToAddress("0x0a")
	state.SetCode(address, ballot_smc_code)

	abi, err := abi.JSON(strings.NewReader(ballot_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// get init winning proposal, should be 0
	get, err := abi.Pack("winningProposal")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0, got", num)
	}
	// create first vote for second candidate , should be successful
	vote, err := abi.Pack("vote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	var sender1 = common.HexToAddress("0x0b")
	state.CreateAccount(sender1)
	state.AddBalance(sender1, big.NewInt(500))
	result, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Origin: sender1})
	if err != nil {
		t.Fatal(err)
	}

	// now we get count of second candidate , should be 1
	getProposal, err := abi.Pack("getVote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getProposal, &sample_kvm.Config{State: state, Origin: sender1})

	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got", num)
	}
	// create duplicate vote for 2nd candidate , should be no error
	vote, err = abi.Pack("vote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Origin: sender1})
	if err != nil {
		t.Fatal(err)
	}

	// now we get vote count of candidate 2th, should be 1 because latter vote was invalid
	getProposal, err = abi.Pack("getVote", uint8(1))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getProposal, &sample_kvm.Config{State: state})

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
	result, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Origin: sender2})
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, vote, &sample_kvm.Config{State: state, Origin: sender3})
	if err != nil {
		t.Fatal(err)
	}
	// now we get the winning candidate, it shoud be 3rd candidate
	result, _, err = sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
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
	result, _, err = sample_kvm.Call(address, getProposal, &sample_kvm.Config{State: state})

	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(2)) != 0 {
		t.Error("Expected 2, got", num)
	}
}