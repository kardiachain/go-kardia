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
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	kvm "github.com/kardiachain/go-kardiamain/kvm"
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

func getSmcValidatorUtil(valSmcAddr common.Address) (*staking.ValidatorSmcUtil, error) {
	util, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return nil, err
	}
	return util, nil
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
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", "10")
	if err != nil {
		t.Fatal(err)
	}

	address1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address1, "Val2", "10", "20", "1", "10")
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
	err = valUtil.Delegate(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr, address, delAmount)
	if err != nil {
		fmt.Println("errr", err)
		t.Fatal(err)
	}

	name, tokens, status, jailed, err := valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}
	name = strings.Replace(name, "\x00", "", -1)
	assert.Equal(t, name, "Val1")
	assert.Equal(t, tokens, delAmount)
	assert.Equal(t, status, uint8(1)) // status is unbond
	assert.Equal(t, jailed, false)

	err = valUtil.StartValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr, address)
	if err != nil {
		t.Fatal(err)
	}

	// check status validator after start
	_, _, status, _, err = valUtil.GetInforValidator(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, status, uint8(2)) // status is bonded

	// check valset
	valSets, err := util.ApplyAndReturnValidatorSets(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(valSets))
}
