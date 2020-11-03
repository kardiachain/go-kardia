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
	"testing"

	_ "github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/types"
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
	//
	//// Start setting up blockchain
	//blockDB := memorydb.New()
	//storeDB := kvstore.NewStoreDB(blockDB)
	//g := genesis.DefaultTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	//address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	//
	//chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), storeDB, g, nil)
	//if genesisErr != nil {
	//	t.Fatal(genesisErr)
	//}
	//
	//bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, false)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Create new contract message
	//msg := types.NewMessage(
	//	address,
	//	nil,
	//	3,
	//	big.NewInt(0),
	//	150000,
	//	big.NewInt(100),
	//	contractCode,
	//	true,
	//)
	//
	//// Create contract without fee
	//result, err := execute(bc, msg)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Get contractAddress from []byte
	//contractAddress := common.BytesToAddress(result)
	//
	//// Call set function
	//definition, err := abi.JSON(strings.NewReader(abiInterface))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Set 1 to counter
	//set, err := definition.Pack("set", uint8(1))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Create call set function message
	//msg = types.NewMessage(
	//	address,
	//	&contractAddress,
	//	3,
	//	big.NewInt(0),
	//	150000,
	//	big.NewInt(100),
	//	set,
	//	true,
	//)
	//
	//// Execute the message
	//if _, err := execute(bc, msg); err != nil {
	//	t.Fatal(err)
	//}
}

func TestStateTransition_TransitionDb_withFee(t *testing.T) {
	//// Start setting up blockchain
	//blockDB := memorydb.New()
	//storeDB := kvstore.NewStoreDB(blockDB)
	//g := genesis.DefaultTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	//address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	//
	//chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), storeDB, g, nil)
	//if genesisErr != nil {
	//	t.Fatal(genesisErr)
	//}
	//
	//bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, false)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Create new contract message
	//msg := types.NewMessage(
	//	address,
	//	nil,
	//	3,
	//	big.NewInt(0),
	//	150000,
	//	big.NewInt(100),
	//	contractCode,
	//	true,
	//)
	//
	//// Create contract with fee
	//result, err := executeWithFee(bc, msg)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Get contractAddress from []byte
	//contractAddress := common.BytesToAddress(result)
	//
	//// Call set function
	//definition, err := abi.JSON(strings.NewReader(abiInterface))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Set 1 to counter
	//set, err := definition.Pack("set", uint8(1))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Create call set function message
	//msg = types.NewMessage(
	//	address,
	//	&contractAddress,
	//	3,
	//	big.NewInt(0),
	//	150000,
	//	big.NewInt(100),
	//	set,
	//	true,
	//)
	//
	//// Execute the message
	//if _, err := executeWithFee(bc, msg); err != nil {
	//	t.Fatal(err)
	//}
}
