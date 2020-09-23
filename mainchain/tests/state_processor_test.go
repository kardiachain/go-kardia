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

package tests

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/types"
)

// GenesisAccounts are used to initialized accounts in genesis block
var initValue = genesis.ToCell(int64(math.Pow10(6)))
var genesisAccounts = map[string]*big.Int{
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
}

// The following abiInterface and contractCode are generated from 'Counter' smartcontract:
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
var (
	abiInterface = `[
  {
    "constant": false,
    "inputs": [
      {
        "name": "x",
        "type": "uint8"
      }
    ],
    "name": "set",
    "outputs": [],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "get",
    "outputs": [
      {
        "name": "",
        "type": "uint8"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  }
]`
	contractCode = common.Hex2Bytes("608060405234801561001057600080fd5b5060da8061001f6000396000f30060806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166324b8ba5f8114604d5780636d4ce63c146067575b600080fd5b348015605857600080fd5b50606560ff60043516608f565b005b348015607257600080fd5b50607960a5565b6040805160ff9092168252519081900360200190f35b6000805460ff191660ff92909216919091179055565b60005460ff16905600a165627a7a723058206cc1a54f543612d04d3f16b0bbb49e9ded9ccf6d47f7789fe3577260346ed44d0029")
	address      = common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
)

func execute(bc *blockchain.BlockChain, msg types.Message) ([]byte, error) {

	// Get stateDb
	stateDb, err := bc.State()
	if err != nil {
		return nil, err
	}

	// Get balance of address
	originBalance := stateDb.GetBalance(address)
	gasPool := new(types.GasPool).AddGas(bc.CurrentBlock().Header().GasLimit)

	// Create a new context to be used in the KVM environment
	context := vm.NewKVMContext(msg, bc.CurrentBlock().Header(), bc)
	vmenv := kvm.NewKVM(context, stateDb, kvm.Config{
		IsZeroFee: true,
	})

	ret, usedGas, failed, err := blockchain.NewStateTransition(vmenv, msg, gasPool).TransitionDb()
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	if failed {
		return nil, errors.New("transaction failed")
	}
	if usedGas != 0 {
		return nil, errors.New("usedGas must be zero")
	}

	balance := stateDb.GetBalance(address)
	if originBalance.Cmp(balance) != 0 {
		return nil, errors.New("originBalance should equal to balance")
	}

	return ret, nil
}

func executeWithFee(bc *blockchain.BlockChain, msg types.Message) ([]byte, error) {

	// Get stateDb
	stateDb, err := bc.State()
	if err != nil {
		return nil, err
	}

	// Get balance of address
	originBalance := stateDb.GetBalance(address)
	gasPool := new(types.GasPool).AddGas(bc.CurrentBlock().Header().GasLimit)

	// Create a new context to be used in the KVM environment
	context := vm.NewKVMContext(msg, bc.CurrentBlock().Header(), bc)
	vmenv := kvm.NewKVM(context, stateDb, kvm.Config{
		IsZeroFee: false,
	})

	ret, usedGas, failed, err := blockchain.NewStateTransition(vmenv, msg, gasPool).TransitionDb()
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	if failed {
		return nil, errors.New("transaction failed")
	}
	if usedGas == 0 {
		return nil, errors.New("usedGas must not be zero")
	}

	balance := stateDb.GetBalance(address)
	if originBalance.Cmp(balance) == 0 {
		return nil, errors.New("originBalance should not equal to balance")
	}

	return ret, nil
}

func TestStateTransition_TransitionDb_noFee(t *testing.T) {

	// Start setting up blockchain
	blockDB := memorydb.New()
	storeDB := kvstore.NewStoreDB(blockDB)
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), storeDB, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
	if genesisErr != nil {
		t.Fatal(genesisErr)
	}

	bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create new contract message
	msg := types.NewMessage(
		address,
		nil,
		3,
		big.NewInt(0),
		150000,
		big.NewInt(100),
		contractCode,
		true,
	)

	// Create contract without fee
	result, err := execute(bc, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Get contractAddress from []byte
	contractAddress := common.BytesToAddress(result)

	// Call set function
	definition, err := abi.JSON(strings.NewReader(abiInterface))
	if err != nil {
		t.Fatal(err)
	}

	// Set 1 to counter
	set, err := definition.Pack("set", uint8(1))
	if err != nil {
		t.Fatal(err)
	}

	// Create call set function message
	msg = types.NewMessage(
		address,
		&contractAddress,
		3,
		big.NewInt(0),
		150000,
		big.NewInt(100),
		set,
		true,
	)

	// Execute the message
	if _, err := execute(bc, msg); err != nil {
		t.Fatal(err)
	}
}

func TestStateTransition_TransitionDb_withFee(t *testing.T) {
	// Start setting up blockchain
	blockDB := memorydb.New()
	storeDB := kvstore.NewStoreDB(blockDB)
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), storeDB, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
	if genesisErr != nil {
		t.Fatal(genesisErr)
	}

	bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create new contract message
	msg := types.NewMessage(
		address,
		nil,
		3,
		big.NewInt(0),
		150000,
		big.NewInt(100),
		contractCode,
		true,
	)

	// Create contract with fee
	result, err := executeWithFee(bc, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Get contractAddress from []byte
	contractAddress := common.BytesToAddress(result)

	// Call set function
	definition, err := abi.JSON(strings.NewReader(abiInterface))
	if err != nil {
		t.Fatal(err)
	}

	// Set 1 to counter
	set, err := definition.Pack("set", uint8(1))
	if err != nil {
		t.Fatal(err)
	}

	// Create call set function message
	msg = types.NewMessage(
		address,
		&contractAddress,
		3,
		big.NewInt(0),
		150000,
		big.NewInt(100),
		set,
		true,
	)

	// Execute the message
	if _, err := executeWithFee(bc, msg); err != nil {
		t.Fatal(err)
	}
}
