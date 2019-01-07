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

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/configs"
	g "github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/permissioned"
)

var Pubkey = "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860"
func GetBlockchain() (*blockchain.BlockChain, error) {
	// Start setting up blockchain
	var genesisAccounts = map[string]int64{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": int64(math.Pow10(15)),
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": int64(math.Pow10(15)),
	}
	kardiaPermissionSmcAddress := configs.GetContractAddressAt(permissioned.KardiaPermissionSmcIndex).String()
	privatechainCandidateSmcAddress := configs.GetContractAddressAt(permissioned.PrivateChainCandidateSmcIndex).String()
	var genesisContracts = map[string]string{
		kardiaPermissionSmcAddress: configs.GenesisContracts[kardiaPermissionSmcAddress],
		privatechainCandidateSmcAddress: configs.GenesisContracts[privatechainCandidateSmcAddress],
	}
	kaiDb := storage.NewMemStore()
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := g.SetupGenesisBlock(log.New(), kaiDb, genesis)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, false)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, err
	}
	return bc, nil
}

func GetSmcPermissionUtil() (*permissioned.PermissionSmcUtil, error) {
	bc, err := GetBlockchain()
	if err != nil {
		return nil, err
	}
	util, err := permissioned.NewSmcPermissionUtil(bc)
	if err != nil {
		return nil, err
	}
	return util, nil
}

func TestCheckNodeValid(t *testing.T) {
	util, err := GetSmcPermissionUtil()
	if err != nil {
		t.Fatal(err)
	}
	// Check valid node with correct type
	isValid, err := util.IsValidNode(Pubkey, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !isValid {
		t.Error("Expect valid node return true, got false")
	}
	// Check valid node with wrong type
	isValid, err = util.IsValidNode(Pubkey, 0)
	if err != nil {
		t.Fatal(err)
	}
	if isValid {
		t.Error("Expect valid node return false, got true")
	}
	// Check invalid node
	isValid, err = util.IsValidNode("anotherkey", 0)
	if err != nil {
		t.Fatal(err)
	}
	if isValid {
		t.Error("Expect valid node return false, got true")
	}
}

func TestCheckValidator(t *testing.T) {
	util, err := GetSmcPermissionUtil()
	if err != nil {
		t.Fatal(err)
	}
	// Check a valid validator
	isValidator, err := util.IsValidator(Pubkey)
	if !isValidator {
		t.Error("Check a valid validator, expect true, got false")
	}
	// Check an invalid validator
	isValidator, err = util.IsValidator("another")
	if isValidator {
		t.Error("Check an invalid validator, expect false, got true")
	}
}

func TestGetNodeInfo(t *testing.T) {
	util, err := GetSmcPermissionUtil()
	if err != nil {
		t.Fatal(err)
	}
	// Get info of a valid node
	address, nodeType, votingPower, listenAddress, err := util.GetNodeInfo(Pubkey)
	if err != nil {
		t.Fatal(err)
	}
	if address.String() != "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6" {
		t.Error("Expect address is 0xc1fe56E3F58D3244F606306611a5d10c8333f1f6, got ", address.String())
	}
	if nodeType.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expect node type is 1, got ", nodeType.String())
	}
	if votingPower.Cmp(big.NewInt(100)) != 0 {
		t.Error("Expect voting power is 100, got ", votingPower.String())
	}
	if listenAddress != "[::]:3000" {
		t.Error("Expect listen address [::]:3000, got ", listenAddress)
	}
	// Get info of an invalid node
	address, nodeType, votingPower, listenAddress, err = util.GetNodeInfo("abc")
	if err != nil {
		t.Fatal(err)
	}
	if address.String() != "0x0000000000000000000000000000000000000000" {
		t.Error("Expect address is 0x0000000000000000000000000000000000000000, got ", address.String())
	}
	if nodeType.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expect node type is 0, got ", nodeType.String())
	}
	if votingPower.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expect voting power is 0, got ", votingPower.String())
	}
	if listenAddress != "" {
		t.Error("Expect listen address is empty, got ", listenAddress)
	}
}

func TestGetInitialNodeByIndex(t *testing.T) {
	util, err := GetSmcPermissionUtil()
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	// Get info of a valid node
	key, address, listenAddress, votingPower, nodeType, err := util.GetAdminNodeByIndex(0)
	if err != nil {
		t.Fatal(err)
	}
	if key != Pubkey {
		t.Error("Expect key ", Pubkey, " got ", key)
	}
	if address.String() != "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6" {
		t.Error("Expect address is 0xc1fe56E3F58D3244F606306611a5d10c8333f1f6, got ", address.String())
	}
	if nodeType.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expect node type is 1, got ", nodeType.String())
	}
	if votingPower.Cmp(big.NewInt(100)) != 0 {
		t.Error("Expect voting power is 100, got ", votingPower.String())
	}
	if listenAddress != "[::]:3000" {
		t.Error("Expect listen address [::]:3000, got ", listenAddress)
	}
	// Get info of an invalid node
	key, address, listenAddress, votingPower, nodeType, err = util.GetAdminNodeByIndex(13)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Error("Expect empty key, got ", key)
	}
	if address.String() != "0x0000000000000000000000000000000000000000" {
		t.Error("Expect address is 0x0000000000000000000000000000000000000000, got ", address.String())
	}
	if nodeType.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expect node type is 0, got ", nodeType.String())
	}
	if votingPower.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expect voting power is 0, got ", votingPower.String())
	}
	if listenAddress != "" {
		t.Error("Expect listen address is empty, got ", listenAddress)
	}
}