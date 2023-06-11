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
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	kvm "github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	g "github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	stypes "github.com/kardiachain/go-kardia/mainchain/staking/types"
	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
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
	kaiDb := rawdb.NewStoreDB(blockDB)
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, kaiDb)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, genesisErr, nil
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig)
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
	util, err := staking.NewSmcStakingUtil()
	if err != nil {
		return nil, nil, err, nil
	}
	return bc, util, nil, stateDB
}

func setup() (*blockchain.BlockChain, *state.StateDB, *staking.StakingSmcUtil, *staking.ValidatorSmcUtil, *types.Block, error) {
	bc, util, err, stateDB := GetSmcStakingUtil()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	valUtil, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return nil, nil, nil, nil, nil, err
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
	block := types.NewBlock(head, nil, &types.Commit{}, nil, trie.NewStackTrie(nil))
	if err := util.SetRoot(stateDB, block.Header(), nil, kvm.Config{}); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return bc, stateDB, util, valUtil, block, nil
}

func getSmcValidatorUtil(valSmcAddr common.Address) (*staking.ValidatorSmcUtil, error) {
	util, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return nil, err
	}
	return util, nil
}

func finalizeTest(stateDB *state.StateDB, util *staking.StakingSmcUtil, block *types.Block) error {
	//test finalizeCommit finalize commit
	err := util.FinalizeCommit(stateDB, block.Header(), nil, kvm.Config{}, stypes.LastCommitInfo{})
	if err != nil {
		return err
	}

	//test double sign
	err = util.DoubleSign(stateDB, block.Header(), nil, kvm.Config{}, []stypes.Evidence{})
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
	_, stateDB, util, _, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", selfDelegate)
	if err != nil {
		t.Fatal(err)
	}

	address1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address1, "Val2", "10", "20", "1", selfDelegate)
	if err != nil {
		t.Fatal(err)
	}

	numberVals, _ := util.GetAllValsLength(stateDB, block.Header(), nil, kvm.Config{})
	assert.Equal(t, numberVals, big.NewInt(2))

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}

	valSmcAddr, err := util.GetValFromOwner(stateDB, block.Header(), nil, kvm.Config{}, common.HexToAddress("0x7cefc13b6e2aedeedfb7cb6c32457240746baee5"))
	addr, _ := util.GetValSmcAddr(stateDB, block.Header(), nil, kvm.Config{}, big.NewInt(0))

	assert.Equal(t, valSmcAddr, addr)
	valUtil, _ := staking.NewSmcValidatorUtil()

	delAmount, _ := new(big.Int).SetString(selfDelegate, 10)
	inforVal, err := valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Replace(string(inforVal.Name[:]), "\x00", "", -1)
	assert.Equal(t, name, "Val1")
	assert.Equal(t, inforVal.Tokens, delAmount)
	assert.Equal(t, inforVal.Status, uint8(0)) // status is unbond
	assert.Equal(t, inforVal.Jailed, false)

	err = valUtil.StartValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr, address)
	if err != nil {
		t.Fatal(err)
	}

	// check status validator after start
	inforVal, err = valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, inforVal.Status, uint8(0)) // status is bonded

	// check valset
	valSets, err := util.ApplyAndReturnValidatorSets(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(valSets))
}

func TestGetCommissionValidator(t *testing.T) {
	_, stateDB, util, _, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	valSmcAddr, err := util.GetValFromOwner(stateDB, block.Header(), nil, kvm.Config{}, common.HexToAddress("0x7cefc13b6e2aedeedfb7cb6c32457240746baee5"))
	valUtil, _ := staking.NewSmcValidatorUtil()

	// get infor commission of validator
	rate, maxRate, maxChangeRate, err := valUtil.GetCommissionValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, rate, big.NewInt(10))
	assert.Equal(t, maxRate, big.NewInt(20))
	assert.Equal(t, maxChangeRate, big.NewInt(1))
}

func TestGetValidatorsByDelegator(t *testing.T) {
	_, stateDB, util, valUtil, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	valSmcAddr, err := util.GetValSmcAddr(stateDB, block.Header(), nil, kvm.Config{}, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	delegatorAddr := common.HexToAddress("0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd")
	valsAddrs, err := util.GetValidatorsByDelegator(stateDB, block.Header(), nil, kvm.Config{}, delegatorAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, []common.Address{}, valsAddrs, "Validators list of this delegator must be empty")
	// delegate for this validator
	delAmount, _ := new(big.Int).SetString(selfDelegate, 10)
	err = valUtil.Delegate(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr, delegatorAddr, delAmount)
	if err != nil {
		t.Fatal(err)
	}
	valsAddrs, err = util.GetValidatorsByDelegator(stateDB, block.Header(), nil, kvm.Config{}, delegatorAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, []common.Address{valSmcAddr}, valsAddrs)
}

func TestDoubleSign(t *testing.T) {
	_, stateDB, util, valUtil, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", selfDelegate)
	if err != nil {
		t.Fatal(err)
	}
	valSmcAddr, err := util.GetValFromOwner(stateDB, block.Header(), nil, kvm.Config{}, common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5"))
	if err != nil {
		t.Fatal(err)
	}
	val, err := valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	assert.EqualValuesf(t, false, val.Jailed, "Created validator must not be jailed")

	if err = util.DoubleSign(stateDB, block.Header(), nil, kvm.Config{}, []stypes.Evidence{
		{
			Address:     address,
			VotingPower: big.NewInt(1),
			Height:      1,
		},
	}); err != nil {
		t.Fatal(err)
	}

	val, err = valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	assert.EqualValuesf(t, true, val.Jailed, "Double signed validator must be jailed")
}
