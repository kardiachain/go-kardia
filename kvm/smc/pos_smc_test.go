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
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/mainchain/staking"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/kvm/sample_kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/types"
)

// Smart contract content is at ./PoS.sol
var pos_smc_code = common.Hex2Bytes("608060405260043610610072576000357c01000000000000000000000000000000000000000000000000000000009004806320c94d981461007757806326476204146100e757806330a563471461012b578063b048e05614610156578063b13c744b146101d1578063e35c0f7d1461024c575b600080fd5b34801561008357600080fd5b506100c66004803603602081101561009a57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061029f565b60405180838152602001821515151581526020019250505060405180910390f35b610129600480360360208110156100fd57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506102d0565b005b34801561013757600080fd5b506101406103d8565b6040518082815260200191505060405180910390f35b34801561016257600080fd5b5061018f6004803603602081101561017957600080fd5b81019080803590602001909291905050506103e5565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3480156101dd57600080fd5b5061020a600480360360208110156101f457600080fd5b810190808035906020019092919050505061041a565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561025857600080fd5b50610261610458565b6040518082600a60200280838360005b8381101561028c578082015181840152602081019050610271565b5050505090500191505060405180910390f35b60006020528060005260406000206000915090508060000154908060010160009054906101000a900460ff16905082565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000209050600015158160010160009054906101000a900460ff16151514156103b45760018160010160006101000a81548160ff02191690831515021790555060018290806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550505b3481600001600082825401925050819055506103d48282600001546104e5565b5050565b6000600180549050905090565b600281600a811015156103f457fe5b016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60018181548110151561042957fe5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b610460610676565b610468610676565b6002600a806020026040519081016040528092919082600a80156104d7576020028201915b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001906001019080831161048d575b505050505090508091505090565b60008090505b600a81101561057f5781600080600284600a8110151561050757fe5b0160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000015410156105725761057f565b80806001019150506104eb565b60006001600a0390505b8181111561062157600260018203600a811015156105a357fe5b0160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600282600a811015156105d457fe5b0160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550808060019003915050610589565b5082600282600a8110151561063257fe5b0160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550505050565b61014060405190810160405280600a9060208202803883398082019150509050509056fea165627a7a72305820d8a92847efd177d4e504067b229d8b19dfbdb2a60a9463e8d154ce0e5501074c0029")
var pos_smc_definition = `[
	{
		"constant": true,
		"inputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"name": "candidateMap",
		"outputs": [
			{
				"name": "totalStake",
				"type": "uint256"
			},
			{
				"name": "existed",
				"type": "bool"
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
				"name": "candidate",
				"type": "address"
			}
		],
		"name": "stake",
		"outputs": [],
		"payable": true,
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getCandidateCount",
		"outputs": [
			{
				"name": "candidateCount",
				"type": "uint256"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "",
				"type": "uint256"
			}
		],
		"name": "validatorList",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "",
				"type": "uint256"
			}
		],
		"name": "candidateList",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getValidatorList",
		"outputs": [
			{
				"name": "",
				"type": "address[10]"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*configs.ChainConfig, common.Hash, error) {
	stakingUtil, _ := staking.NewSmcStakingUtil()
	return genesis.SetupGenesisBlock(log.New(), db, g, stakingUtil)
}

func SetupBlockchainForTesting() (*blockchain.BlockChain, error) {
	initValue := genesis.ToCell(int64(math.Pow10(6)))

	var genesisAccounts = map[string]*big.Int{
		"0x1234": initValue,
		"0x5678": initValue,
		"0xabcd": initValue,
	}
	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	chainConfig, _, genesisErr := setupGenesis(g, kaiDb)
	if genesisErr != nil {
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, nil, nil)
	return bc, err
}

// E2e PoS testing
func TestPoS(t *testing.T) {
	bc, err := SetupBlockchainForTesting()
	if err != nil {
		t.Fatal(err)
	}
	state, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}

	// Setup contract code into genesis state
	address := common.HexToAddress("0x0a")
	state.SetCode(address, pos_smc_code)
	abi, err := abi.JSON(strings.NewReader(pos_smc_definition))
	if err != nil {
		t.Fatal(err)
	}

	// Gets candidate count and verifies it is 0
	get, err := abi.Pack("getCandidateCount")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0 candidate, got", num)
	}

	// Stake for one candidate
	stake, err := abi.Pack("stake", common.HexToAddress("0x1111"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, stake, &sample_kvm.Config{State: state, Value: big.NewInt(1000), Origin: common.HexToAddress("0x1234")})
	if err != nil {
		t.Fatal(err)
	}

	// Stake for second candidate
	stake, err = abi.Pack("stake", common.HexToAddress("0x2222"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, stake, &sample_kvm.Config{State: state, Value: big.NewInt(2000), Origin: common.HexToAddress("0x5678")})
	if err != nil {
		t.Fatal(err)
	}

	// Stake for third candidate
	stake, err = abi.Pack("stake", common.HexToAddress("0x3333"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, stake, &sample_kvm.Config{State: state, Value: big.NewInt(3000), Origin: common.HexToAddress("0xabcd")})
	if err != nil {
		t.Fatal(err)
	}

	// Gets candidate count again and verifies it is 3 now
	result, _, err = sample_kvm.Call(address, get, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(3)) != 0 {
		t.Error("Expected 3 candidate, got", num)
	}

	// Get first candidate to verify
	getCandidate, err := abi.Pack("candidateMap", common.HexToAddress("0x1111"))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getCandidate, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result[:32])
	if num.Cmp(big.NewInt(1000)) != 0 {
		t.Error("Expected 1000 stake, got", num)
	}

	// Get second candidate to verify
	getCandidate, err = abi.Pack("candidateMap", common.HexToAddress("0x2222"))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getCandidate, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result[:32])
	if num.Cmp(big.NewInt(2000)) != 0 {
		t.Error("Expected 2000 stake, got", num)
	}

	// Get first validator
	getVal, err := abi.Pack("validatorList", big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getVal, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	addr := common.BytesToAddress(result)
	if !addr.Equal(common.HexToAddress("0x3333")) {
		t.Error("Expected 0x3333, got ", addr.String())
	}

	// Check current validator list
	getValList, err := abi.Pack("getValidatorList")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getValList, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	// Solidity memory layout is with 32-byte boundary so although address is 20-byte, it will return 32-byte.
	addr = common.BytesToAddress(result[12:32])
	if !addr.Equal(common.HexToAddress("0x3333")) {
		t.Error("Expected 0x3333, got ", addr.String())
	}

	// Now, stake more for first candidate, so he will become the highest validator
	stake, err = abi.Pack("stake", common.HexToAddress("0x1111"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, stake, &sample_kvm.Config{State: state, Value: big.NewInt(3000), Origin: common.HexToAddress("0x5678")})
	if err != nil {
		t.Fatal(err)
	}

	// Now check for the validator list again and verify first candidate is at the top
	getValList, err = abi.Pack("getValidatorList")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(address, getValList, &sample_kvm.Config{State: state})
	if err != nil {
		t.Fatal(err)
	}
	// Solidity memory layout is with 32-byte boundary so although address is 20-byte, it will return 32-byte.
	addr = common.BytesToAddress(result[12:32])
	if !addr.Equal(common.HexToAddress("0x1111")) {
		t.Error("Expected 0x1111, got ", addr.String())
	}
}
