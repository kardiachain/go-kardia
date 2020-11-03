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

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/account"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/types"
)

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*configs.ChainConfig, common.Hash, error) {
	return genesis.SetupGenesisBlock(log.New(), db, g, nil)
}

func TestGenesis_AllocFromData(t *testing.T) {
	var data = make(map[string]*big.Int, len(privKeys))
	// todo: CreateKeyStoreJSON take too long (~1s/private key)
	for _, pk := range privKeys {
		keystore := account.KeyStore{Path: ""}
		keystoreJson, err := keystore.NewKeyStoreJSON(keystorePassword, pk)
		assert.Nil(t, err, "Cannot create key store", err)
		assert.NotNil(t, keystoreJson, "KeyStore should not nil")
		data[keystoreJson.Address] = configs.InitValue
	}

	ga, err := genesis.AllocFromData(data)
	assert.Nil(t, err, "Genesis alloc from data must success", err)

	for _, el := range addresses {
		_, ok := ga[common.HexToAddress(el)]
		assert.Equal(t, true, ok, "Genesis alloc failed", common.HexToAddress(el))
	}
}

func TestGenesis_AllocFromContractData(t *testing.T) {
	//for id, c := range genesisContracts {
	//	ga, err := genesis.AllocFromContractData(c)
	//	assert.Nil(t, err, "alloc failed", err)
	//	assert.NotNil(t, ga, "genesis alloc nil")
	//	for addr, data := range genesisContracts[id] {
	//		gc, ok := ga[common.HexToAddress(addr)]
	//		assert.True(t, ok, "cannot get gc")
	//		assert.Equal(t, common.Hex2Bytes(data), gc.Code, "Code in [[bytes should be equal")
	//		assert.Equal(t, genesis.ToCell(100), gc.Balance, "balance should be 100")
	//	}
	//}
}

func TestGenesis_ToCell(t *testing.T) {
	cell := genesis.ToCell(int64(math.Pow(10, 6)))
	assert.Equal(t, 25, len(cell.String()))
}

func TestGenesisAllocFromAccountAndContract(t *testing.T) {
	blockDB := memorydb.New()
	db := kvstore.NewStoreDB(blockDB)
	// Create genesis block with state_processor_test.genesisAccounts
	g := genesis.DefaultTestnetFullGenesisBlock(configs.GenesisAccounts, genesisContracts)
	chainConfig, hash, err := setupGenesis(g, db)
	assert.Nil(t, err, "cannot setup genesis")
	bc, err := blockchain.NewBlockChain(log.New(), db, chainConfig, false)
	assert.Nil(t, err, "cannot create block chain", err)

	headBlockHash := db.ReadHeadBlockHash()
	canonicalHash := db.ReadCanonicalHash(0)

	assert.Condition(t, func() bool {
		return hash.Equal(headBlockHash) && hash.Equal(canonicalHash)
	})

	// Init new State with current BlockHash
	s, err := bc.State()
	assert.Nil(t, err, "cannot get state", err)
	if err != nil {
		t.Error(err)
	}

	for address, code := range genesisContracts {
		smcCode := common.Encode(s.GetCode(common.HexToAddress(address)))
		assert.Equal(t, "0x"+code, smcCode, "scmCode should equal %s. Got: %s", "0x"+code, smcCode)
	}
	// Get balance from addresses
	for addr := range configs.GenesisAccounts {
		b := s.GetBalance(common.HexToAddress(addr))
		assert.Equal(t, initBalance, b, "balance should equal %v. Got: %v", initBalance, b)
	}
}

//
//func TestGenesisAllocFromAccountAndContract(t *testing.T) {
//	blockDB := memorydb.New()
//	db := kvstore.NewStoreDB(blockDB)
//	// Create genesis block with state_processor_test.genesisAccounts
//	g := genesis.DefaultTestnetFullGenesisBlock(defaultGenesisAccounts, genesisContracts)
//	chainConfig, hash, err := setupGenesis(g, db)
//	if err != nil {
//		t.Error(err)
//	}
//	bc, err := blockchain.NewBlockChain(log.New(), db, chainConfig, false)
//	if err != nil {
//		t.Fatal(err)
//	}
//	headBlockHash := db.ReadHeadBlockHash()
//	canonicalHash := db.ReadCanonicalHash(0)
//
//	if !hash.Equal(headBlockHash) || !hash.Equal(canonicalHash) {
//		t.Error("Current BlockHash does not match")
//	}
//
//	// Init new State with current BlockHash
//	s, err := bc.State()
//	if err != nil {
//		t.Error(err)
//	} else {
//		// Get code from addresses
//		for address, code := range genesisContracts {
//			smc_code := common.Encode(s.GetCode(common.HexToAddress(address)))
//
//			if smc_code != "0x"+code {
//				t.Errorf("Code does not match, expected %v \n got %v", smc_code, code)
//			}
//		}
//		// Get balance from addresses
//		for addr := range configs.GenesisAccounts {
//			b := s.GetBalance(common.HexToAddress(addr))
//			if b.Cmp(initBalance) != 0 {
//				t.Error("Balance does not match", "state balance", b, "init balance", initBalance)
//			}
//		}
//	}
//}
