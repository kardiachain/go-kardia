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
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	kvm "github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/kvm/sample_kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"

	"github.com/kardiachain/go-kardiamain/mainchain/staking"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	votingPower     int64 = 15000000000000000
	commissionRate        = "100000000000000000"
	maxRate               = "250000000000000000"
	maxChangeRate         = "50000000000000000"
	minSelfDelegate       = "10000000000000000000000000"
	selfDelegate          = "15000000000000000000000000"
)

func GetBlockchainStaking() (*blockchain.BlockChain, error, *state.StateDB) {
	logger := log.New()
	logger.AddTag("test state")
	// Start setting up blockchain
	initValue, ok := big.NewInt(0).SetString("1000000000000000000000000000", 10)
	if !ok {
		log.Error("error initialize value")
		return nil, errors.New("error initialize value"), nil
	}
	var genesisAccounts = map[string]*big.Int{
		"0x1234": initValue,
		"0x5678": initValue,
		"0xabcd": initValue,
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
		"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": initValue,
	}

	configs.AddDefaultContract()

	for address := range genesisAccounts {
		genesisAccounts[address] = initValue
	}

	genesisContracts := make(map[string]string)
	for key, contract := range configs.GetContracts() {
		configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
		if key != configs.StakingContractKey {
			genesisContracts[contract.Address] = contract.ByteCode
		}
	}

	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, kaiDb)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, genesisErr, nil
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, false)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, err, nil
	}
	stateDB, err := bc.State()
	if err != nil {
		return nil, err, nil
	}
	return bc, nil, stateDB
}

func GetSmcStakingUtil() (*blockchain.BlockChain, *staking.StakingSmcUtil, error, *state.StateDB) {
	bc, err, stateDB := GetBlockchainStaking()
	if err != nil {
		return nil, nil, err, nil
	}
	util, err := staking.NewSmcStakingnUtil()
	if err != nil {
		return nil, nil, err, nil
	}
	return bc, util, nil, stateDB
}

func setup() (*blockchain.BlockChain, *state.StateDB, *staking.StakingSmcUtil, *types.Block, error) {
	bc, util, err, stateDB := GetSmcStakingUtil()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	head := &types.Header{
		Height:   0,
		GasLimit: uint64(100000000000),
		AppHash:  common.Hash{},
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: uint32(0),
			},
		},
	}
	block := types.NewBlock(head, nil, &types.Commit{}, nil)
	return bc, stateDB, util, block, nil
}

func finalizeTest(stateDB *state.StateDB, util *staking.StakingSmcUtil, block *types.Block) error {
	//test finalizeCommit finalize commit
	err := util.FinalizeCommit(stateDB, block.Header(), nil, kvm.Config{}, staking.LastCommitInfo{})
	if err != nil {
		return err
	}

	//test double sign
	err = util.DoubleSign(stateDB, block.Header(), nil, kvm.Config{}, []staking.Evidence{})
	if err != nil {
		return err
	}

	//test set address root
	err = util.SetRoot(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		return err
	}

	return nil
}

func TestCreateValidator(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "10", "20", "1", "10", "11")
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ApplyAndReturnValidatorSets(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidators(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	newValidator := types.NewValidator(address, votingPower)

	validators, err := util.GetValidators(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, newValidator, validators[0], "Validators fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidator(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	expectedCommissionRate, ok1 := new(big.Int).SetString(commissionRate, 10)
	expectedMaxRate, ok2 := new(big.Int).SetString(maxRate, 10)
	expectedMaxChangeRate, ok3 := new(big.Int).SetString(maxChangeRate, 10)
	expectedSelfDelegate, ok4 := new(big.Int).SetString(selfDelegate, 10)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		t.Fatal("Error while parsing genesis validator params")
	}
	newValidator := &types.Validator{
		Address:        address,
		VotingPower:    votingPower,
		StakedAmount:   expectedSelfDelegate,
		CommissionRate: expectedCommissionRate,
		MaxRate:        expectedMaxRate,
		MaxChangeRate:  expectedMaxChangeRate,
	}

	validator, err := util.GetValidator(stateDB, block.Header(), nil, kvm.Config{}, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, newValidator, validator, "Validator fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidatorPower(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	validatorPower, err := util.GetValidatorPower(stateDB, block.Header(), nil, kvm.Config{}, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, votingPower, validatorPower, "Validator power fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidatorCommission(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		valAddr = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	validatorCommission, err := util.GetValidatorCommission(stateDB, block.Header(), nil, kvm.Config{}, valAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNilf(t, validatorCommission, "Validator commission must not be nil")
	assert.IsTypef(t, big.NewInt(0), validatorCommission, "Validator power fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDelegationsByValidator(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	delegations, err := util.GetDelegationsByValidator(stateDB, block.Header(), nil, kvm.Config{}, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNilf(t, delegations, "A validator must have at least 1 delegation")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDelegationRewards(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	delegationRewards, err := util.GetDelegationRewards(stateDB, block.Header(), nil, kvm.Config{}, address, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNilf(t, delegationRewards, "Delegator's reward must not be nil")
	assert.IsTypef(t, big.NewInt(0), delegationRewards, "Delegator's reward fetched from staking SMC must be a *big.Int")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDelegatorStake(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		address = common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	stake, err := util.GetDelegatorStake(stateDB, block.Header(), nil, kvm.Config{}, address, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNilf(t, stake, "Delegator's stake must not be nil")
	assert.IsTypef(t, big.NewInt(0), stake, "Delegator's stake fetched from staking SMC must be a *big.Int")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetInflation(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	abi := util.Abi

	// Successfully
	setInflation, err := abi.Pack("setInflation", big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ConstructAndApplySmcCallMsg(stateDB, block.Header(), nil, kvm.Config{}, setInflation)
	if err != nil {
		t.Fatal(err)
	}

	// Successfully
	getInflation, err := abi.Pack("getInflation")
	if err != nil {
		t.Fatal(err)
	}
	inflation, err := util.ConstructAndApplySmcCallMsg(stateDB, block.Header(), nil, kvm.Config{}, getInflation)
	if err != nil {
		t.Fatal(err)
	}

	num := new(big.Int).SetBytes(inflation)
	if num.Cmp(big.NewInt(100)) != 0 {
		t.Error("Expected 100, got ", num)
	}
}

func TestSetTotalSupply(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	abi := util.Abi

	// Successfully
	setTotalSupply, err := abi.Pack("setTotalSupply", big.NewInt(200000))
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ConstructAndApplySmcCallMsg(stateDB, block.Header(), nil, kvm.Config{}, setTotalSupply)
	if err != nil {
		t.Fatal(err)
	}

	getTotalSupply, err := abi.Pack("getTotalSupply")
	if err != nil {
		t.Fatal(err)
	}
	result, err := util.ConstructAndApplySmcCallMsg(stateDB, block.Header(), nil, kvm.Config{}, getTotalSupply)
	if err != nil {
		t.Fatal(err)
	}

	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(200000)) != 0 {
		t.Error("Expected 200000, got ", num)
	}

}
func TestCreateValidator2(t *testing.T) {
	bc, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	owner := common.HexToAddress("0x1234")
	abi := util.Abi

	baseProposerReward := big.NewInt(1)
	bonusProposerReward := big.NewInt(1)
	slashFractionDowntime := big.NewInt(1)
	slashFractionDoubleSign := big.NewInt(2)
	unBondingTime := big.NewInt(1)
	signedBlockWindow := big.NewInt(2)
	minSignedBlockPerWindow := big.NewInt(1)
	// set params
	setParams, err := abi.Pack("setParams", big.NewInt(100), big.NewInt(600), baseProposerReward, bonusProposerReward, slashFractionDowntime, slashFractionDoubleSign, unBondingTime, signedBlockWindow, minSignedBlockPerWindow)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, setParams, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	var (
		validator1 = common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}

	//check get delegation
	getDelegation, err := abi.Pack("getDelegation", validator1, validator1)
	if err != nil {
		t.Fatal(err)
	}
	msg := types.NewMessage(
		owner,
		&util.ContractAddress,
		0,
		big.NewInt(0),
		3000000,
		big.NewInt(0),
		getDelegation,
		false,
	)
	res, err := staking.Apply(log.New(), bc, stateDB, block.Header(), kvm.Config{}, msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(res)
	if num.Cmp(big.NewInt(1000000000000000000)) != 0 {
		t.Error("Expected delegation 1000000000000000000, got #", num)
	}

	//check get validator
	getValidator, err := abi.Pack("getValidator", validator1)
	if err != nil {
		t.Fatal(err)
	}
	msg = types.NewMessage(
		owner,
		&util.ContractAddress,
		0,
		big.NewInt(0),
		3000000,
		big.NewInt(0),
		getValidator,
		false,
	)
	res, err = staking.Apply(log.New(), bc, stateDB, block.Header(), kvm.Config{}, msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal(err)
	}

	//check bond
	num = new(big.Int).SetBytes(res[:32])
	expectedDelegation, ok := new(big.Int).SetString("15000000000000000000000000", 10)
	if !ok {
		t.Fatal("Cannot parse string to big.Int")
	}
	if num.Cmp(expectedDelegation) != 0 {
		t.Error("Expected delegation 15000000000000000000000000, got #", num)
	}
	//check delegation shares
	num = new(big.Int).SetBytes(res[32:64])
	if num.Cmp(big.NewInt(1000000000000000000)) != 0 {
		t.Error("Expected delegation 1000000000000000000, got #", num)
	}

	//check get validators
	validators, err := util.GetValidators(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	//check get address of validator from getValidators
	if !validators[0].Address.Equal(validator1) {
		t.Error("Error for address")
	}
	//check voting power from getValidators
	if num.Cmp(big.NewInt(1000000000000000000)) != 0 {
		t.Error("Expected delegation 1000000000000000000, got #", num)
	}

	// check get getAllDelegatorStake
	getAllDelegatorStake, err := abi.Pack("getAllDelegatorStake", validator1)
	if err != nil {
		t.Fatal(err)
	}
	msg = types.NewMessage(
		owner,
		&util.ContractAddress,
		0,
		big.NewInt(0),
		3000000,
		big.NewInt(0),
		getAllDelegatorStake,
		false,
	)
	res, err = staking.Apply(log.New(), bc, stateDB, block.Header(), kvm.Config{}, msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(res)
	expectedDelegation, ok = new(big.Int).SetString("15000000000000000000000000", 10)
	if !ok {
		t.Fatal("Cannot parse string to big.Int")
	}
	if num.Cmp(expectedDelegation) != 0 {
		t.Error("Expected delegation 15000000000000000000000000, got #", num)
	}

}
func TestDelegate(t *testing.T) {
	bc, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	owner := common.HexToAddress("0x1234")
	// Setup contract code into genesis state
	abi := util.Abi
	baseProposerReward := big.NewInt(1)
	bonusProposerReward := big.NewInt(1)
	slashFractionDowntime := big.NewInt(1)
	slashFractionDoubleSign := big.NewInt(2)
	unBondingTime := big.NewInt(1)
	signedBlockWindow := big.NewInt(2)
	minSignedBlockPerWindow := big.NewInt(1)
	// set params
	setParams, err := abi.Pack("setParams", big.NewInt(100), big.NewInt(600), baseProposerReward, bonusProposerReward, slashFractionDowntime, slashFractionDoubleSign, unBondingTime, signedBlockWindow, minSignedBlockPerWindow)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, setParams, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	//create validator
	var (
		validator1 = common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	)
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, validator1, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	//check delegate
	bond1 := big.NewInt(20000)
	// bond2 := big.NewInt(20000)
	account1 := common.HexToAddress("0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd")
	// account2 := common.HexToAddress("0x5678")
	delegate, err := abi.Pack("delegate", validator1)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(util.ContractAddress, delegate, &sample_kvm.Config{State: stateDB, Value: bond1, Origin: account1})
	if err != nil {
		t.Fatal(err)
	}

	//check get delegation
	getValidatorsByDelegator, err := abi.Pack("getValidatorsByDelegator", account1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := sample_kvm.Call(util.ContractAddress, getValidatorsByDelegator, &sample_kvm.Config{State: stateDB, Origin: account1})
	if err != nil {
		t.Fatal(err)
	}
	validatorAddress := string(hex.EncodeToString(result[76:96]))
	if strings.TrimRight(validatorAddress, "\n") != "c1fe56e3f58d3244f606306611a5d10c8333f1f6" {
		t.Error("Error for address")
	}

	//check get delegation
	getDelegation, err := abi.Pack("getDelegation", validator1, account1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(util.ContractAddress, getDelegation, &sample_kvm.Config{State: stateDB, Origin: account1})
	if err != nil {
		t.Fatal(err)
	}
	num := new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected delegation 0, got #", num)
	}

	//check get delegation by validator
	getDelegationsByValidator, err := abi.Pack("getDelegationsByValidator", validator1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(util.ContractAddress, getDelegationsByValidator, &sample_kvm.Config{State: stateDB, Origin: validator1})
	if err != nil {
		t.Fatal(err)
	}

	delAddress := string(hex.EncodeToString(result[140:160]))
	if strings.TrimRight(delAddress, "\n") != "ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd" {
		t.Error("Error for address")
	}

	getAllDelegatorStake, err := abi.Pack("getAllDelegatorStake", account1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(util.ContractAddress, getAllDelegatorStake, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// check undelegate
	amountUndel := big.NewInt(500)
	undelegate, err := abi.Pack("undelegate", validator1, amountUndel)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(util.ContractAddress, undelegate, &sample_kvm.Config{State: stateDB, Origin: account1})
	if err != nil {
		t.Fatal(err)
	}

	getAllDelegatorStake, err = abi.Pack("getAllDelegatorStake", account1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(util.ContractAddress, getAllDelegatorStake, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result)
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected delegation 0, got #", num)
	}

	//check get getAllDelegatorStake
	getUBDEntries, err := abi.Pack("getUBDEntries", validator1, account1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = sample_kvm.Call(util.ContractAddress, getUBDEntries, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}
	num = new(big.Int).SetBytes(result[96:128])
	if num.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected delegation 0, got #", num)
	}

	//check apply and return validator sets
	valsSet, err := util.ApplyAndReturnValidatorSets(stateDB, block.Header(), bc, kvm.Config{})
	if !valsSet[0].Address.Equal(validator1) {
		t.Error("Error for address")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoubleSign(t *testing.T) {
	_, stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	var (
		validator1 = common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
		owner      = common.HexToAddress("0x1234")
		abi        = util.Abi
	)

	baseProposerReward := big.NewInt(1)
	bonusProposerReward := big.NewInt(1)
	slashFractionDowntime := big.NewInt(1)
	slashFractionDoubleSign := big.NewInt(2)
	unBondingTime := big.NewInt(1)
	signedBlockWindow := big.NewInt(2)
	minSignedBlockPerWindow := big.NewInt(1)
	// set params
	setParams, err := abi.Pack("setParams", big.NewInt(100), big.NewInt(600), baseProposerReward, bonusProposerReward, slashFractionDowntime, slashFractionDoubleSign, unBondingTime, signedBlockWindow, minSignedBlockPerWindow)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, setParams, &sample_kvm.Config{State: stateDB, Origin: owner})
	if err != nil {
		t.Fatal(err)
	}

	// create validator
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, validator1, commissionRate, maxRate, maxChangeRate, minSelfDelegate, selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	//check get delegation
	doubleSign, err := abi.Pack("doubleSign", validator1, big.NewInt(votingPower), big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = sample_kvm.Call(address, doubleSign, &sample_kvm.Config{State: stateDB, Origin: validator1})
	if err != nil {
		t.Fatal(err)
	}
}
