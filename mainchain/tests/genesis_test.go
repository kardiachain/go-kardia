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
	"math"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/types"
)

const (
	password = "KardiaChain"
)

var (
	addresses = []string{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5",
		"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd",
		"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28",
		"0x94FD535AAB6C01302147Be7819D07817647f7B63",
		"0xa8073C95521a6Db54f4b5ca31a04773B093e9274",
		"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547",
		"0xBA30505351c17F4c818d94a990eDeD95e166474b",
		"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0",
		"0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
	}
	privKeys = []string{
		"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
		"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
		"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
		"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
		"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
		"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
		"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
		"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
		"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
		"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d",
	}
	InitValue        = big.NewInt(int64(math.Pow10(10))) // Update Genesis Account Values
	initBalance      = InitValue.Mul(InitValue, big.NewInt(int64(math.Pow10(18))))
	genesisContracts = map[string]string{
		// Simple voting contract bytecode in genesis block, source code in kvm/smc/Ballot.sol
		"0x00000000000000000000000000000000736D6332": "608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029",
		// Counter contract bytecode in genesis block, source code in kvm/smc/SimpleCounter.sol
		"0x00000000000000000000000000000000736D6331": "6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e5780636d4ce63c14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060a9565b005b348015608657600080fd5b50608d60c6565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008060009054906101000a900460ff169050905600a165627a7a7230582083f88bef40b78ed8ab5f620a7a1fb7953640a541335c5c352ff0877be0ecd0c60029",
	}
)

func TestGenesisAllocFromData(t *testing.T) {

	var data = make(map[string]*big.Int, len(privKeys))
	for i := range addresses {
		data[addresses[i]] = initBalance
	}

	ga, err := genesis.GenesisAllocFromData(data)
	if err != nil {
		t.Error(err)
	}

	for _, el := range addresses {
		if _, ok := ga[common.HexToAddress(el)]; ok == false {
			t.Error("address ", el, " is not valid")
		}
	}
}

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*configs.ChainConfig, common.Hash, error) {
	stakingUtil, _ := staking.NewSmcStakingUtil()
	return genesis.SetupGenesisBlock(log.New(), db, g, stakingUtil)
}

func TestCreateGenesisBlock(t *testing.T) {
	// Test generate genesis block
	// allocData is get from genesisAccounts in default_node_config
	InitValue = big.NewInt(int64(math.Pow10(10))) // Update Genesis Account Values
	initBalance = InitValue.Mul(InitValue, big.NewInt(int64(math.Pow10(18))))
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}

	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
	// Init kai database
	blockDB := memorydb.New()
	db := kvstore.NewStoreDB(blockDB)
	// Create genesis block with state_processor_test.genesisAccounts
	g := genesis.DefaultTestnetGenesisBlock(genesisAccounts)
	chainConfig, hash, err := setupGenesis(g, db)
	if err != nil {
		t.Error(err)
	}
	bc, err := blockchain.NewBlockChain(log.New(), db, chainConfig)
	if err != nil {
		t.Fatal(err)
	}

	// There are 2 ways of getting current blockHash
	// ReadHeadBlockHash or ReadCanonicalHash
	headBlockHash := db.ReadHeadBlockHash()
	canonicalHash := db.ReadCanonicalHash(0)

	if !hash.Equal(headBlockHash) || !hash.Equal(canonicalHash) {
		t.Error("Current BlockHash does not match")
	}

	// Init new State with current BlockHash
	s, err := bc.State()
	if err != nil {
		t.Error(err)
	} else {
		// Get balance from addresses
		for addr := range genesisAccounts {
			b := s.GetBalance(common.HexToAddress(addr))
			if b.Cmp(initBalance) != 0 {
				t.Error("Balance does not match", "state balance", b, "balance", initBalance)
			}
		}

	}
}

func TestGenesisAllocFromAccountAndContract(t *testing.T) {
	blockDB := memorydb.New()
	db := kvstore.NewStoreDB(blockDB)
	// Create genesis block with state_processor_test.genesisAccounts
	InitValue = big.NewInt(int64(math.Pow10(10))) // Update Genesis Account Values
	initBalance = InitValue.Mul(InitValue, big.NewInt(int64(math.Pow10(18))))
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, hash, err := setupGenesis(g, db)
	if err != nil {
		t.Error(err)
	}
	bc, err := blockchain.NewBlockChain(log.New(), db, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	headBlockHash := db.ReadHeadBlockHash()
	canonicalHash := db.ReadCanonicalHash(0)

	if !hash.Equal(headBlockHash) || !hash.Equal(canonicalHash) {
		t.Error("Current BlockHash does not match")
	}

	// Init new State with current BlockHash
	s, err := bc.State()
	if err != nil {
		t.Error(err)
	} else {
		// Get code from addresses
		for address, code := range genesisContracts {
			smc_code := common.Encode(s.GetCode(common.HexToAddress(address)))

			if smc_code != "0x"+code {
				t.Errorf("Code does not match, expected %v \n got %v", smc_code, code)
			}
		}
		// Get balance from addresses
		for addr := range genesisAccounts {
			b := s.GetBalance(common.HexToAddress(addr))
			if b.Cmp(initBalance) != 0 {
				t.Error("Balance does not match", "state balance", b, "init balance", initBalance)
			}
		}
	}
}
