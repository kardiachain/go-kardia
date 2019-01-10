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
	"encoding/hex"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/tool"
	"strings"
	"testing"
)

var candidate_exchange_smc_code = common.Hex2Bytes("60806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631a83260c146100515780635e89f51b14610146575b600080fd5b34801561005d57600080fd5b50610144600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506102f4565b005b34801561015257600080fd5b506102f2600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803560ff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050919291929050505061046f565b005b7fe2df4c83eef1ab3c88f888395f79598f4f2b295eb4701a77c0278a2027392b3f83838360405180806020018060200180602001848103845287818151815260200191508051906020019080838360005b83811015610360578082015181840152602081019050610345565b50505050905090810190601f16801561038d5780820380516001836020036101000a031916815260200191505b50848103835286818151815260200191508051906020019080838360005b838110156103c65780820151818401526020810190506103ab565b50505050905090810190601f1680156103f35780820380516001836020036101000a031916815260200191505b50848103825285818151815260200191508051906020019080838360005b8381101561042c578082015181840152602081019050610411565b50505050905090810190601f1680156104595780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390a1505050565b7f35b41175b5371971b4b197e996dbb4a277264e61dbbd0757972f549544bfe3ce878787878787876040518080602001806020018860ff1660ff1681526020018773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200180602001806020018060200186810386528d818151815260200191508051906020019080838360005b8381101561052557808201518184015260208101905061050a565b50505050905090810190601f1680156105525780820380516001836020036101000a031916815260200191505b5086810385528c818151815260200191508051906020019080838360005b8381101561058b578082015181840152602081019050610570565b50505050905090810190601f1680156105b85780820380516001836020036101000a031916815260200191505b50868103845289818151815260200191508051906020019080838360005b838110156105f15780820151818401526020810190506105d6565b50505050905090810190601f16801561061e5780820380516001836020036101000a031916815260200191505b50868103835288818151815260200191508051906020019080838360005b8381101561065757808201518184015260208101905061063c565b50505050905090810190601f1680156106845780820380516001836020036101000a031916815260200191505b50868103825287818151815260200191508051906020019080838360005b838110156106bd5780820151818401526020810190506106a2565b50505050905090810190601f1680156106ea5780820380516001836020036101000a031916815260200191505b509c5050505050505050505050505060405180910390a1505050505050505600a165627a7a72305820e2f09b64ab74198849d89572d7cccab1a49327414e18705712450ef1486565f30029")
var candidate_exchange_smc_definition = `[
	{
		"constant": false,
		"inputs": [
			{
				"name": "_email",
				"type": "string"
			},
			{
				"name": "_fromOrgID",
				"type": "string"
			},
			{
				"name": "_toOrgID",
				"type": "string"
			}
		],
		"name": "requestCandidateInfo",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "_email",
				"type": "string"
			},
			{
				"name": "_name",
				"type": "string"
			},
			{
				"name": "_age",
				"type": "uint8"
			},
			{
				"name": "_addr",
				"type": "address"
			},
			{
				"name": "_source",
				"type": "string"
			},
			{
				"name": "_fromOrgID",
				"type": "string"
			},
			{
				"name": "_toOrgID",
				"type": "string"
			}
		],
		"name": "fulfillCandidateInfo",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "email",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "fromOrgID",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "toOrgID",
				"type": "string"
			}
		],
		"name": "ExternalCandidateInfoRequested",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": false,
				"name": "email",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "name",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "age",
				"type": "uint8"
			},
			{
				"indexed": false,
				"name": "addr",
				"type": "address"
			},
			{
				"indexed": false,
				"name": "source",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "fromOrgID",
				"type": "string"
			},
			{
				"indexed": false,
				"name": "toOrgID",
				"type": "string"
			}
		],
		"name": "ExternalCandidateInfoFulfilled",
		"type": "event"
	}
]`

// TestRequestExternalCandidateInfo tests if a tx sent to Kardia candidate info exchange contract to request info fires
// correct event and returns correct data (email, fromOrgId, toOrdId)
func TestRequestExternalCandidateInfo(t *testing.T) {
	bc, err := SetupBlockchainForTesting()
	if err != nil {
		t.Fatal(err)
	}
	statedb, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}
	// Setup contract code into newly generated state
	address := common.HexToAddress("0x0a")
	statedb.SetCode(address, candidate_exchange_smc_code)
	abi, err := abi.JSON(strings.NewReader(candidate_exchange_smc_definition))
	if err != nil {
		t.Fatal(err)
	}
	// Create tx to request candidate info from external chain
	requestCandidateInfoInput, err := abi.Pack("requestCandidateInfo", "a@gmail.com", "org1", "org2")
	if err != nil {
		t.Fatal(err)
	}
	addrKeyBytes, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := tool.GenerateSmcCall(addrKey, address, requestCandidateInfoInput,
		state.ManageState(statedb))
	// Apply tx and get returned logs from that tx
	logs, err := ApplyTransactionReturnLog(bc, statedb, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if there is event emitted from previous tx
	if len(logs) == 0 {
		t.Error("Expect length of log > 0, 0 is returned")
	}
	var externalCandidateInfoRequested struct {
		Email     string
		FromOrgID string
		ToOrgID   string
	}
	err = abi.Unpack(&externalCandidateInfoRequested, "ExternalCandidateInfoRequested", logs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if externalCandidateInfoRequested.Email != "a@gmail.com" {
		t.Error("Expect requested email is a@gmail.com, got ", externalCandidateInfoRequested.Email)
	}
	if externalCandidateInfoRequested.FromOrgID != "org1" {
		t.Error("Expect from org1, got ", externalCandidateInfoRequested.FromOrgID)
	}
	if externalCandidateInfoRequested.ToOrgID != "org2" {
		t.Error("Expect to org2, got ", externalCandidateInfoRequested.ToOrgID)
	}
}

// TestFulFillExternalCandidateIno tests if a tx sent to Kardia candidate info exchange contract to fulfill info fires
// correct event and returns correct data (email, fromOrgId, toOrdId)
func TestFulfillExternalCandidateInfo(t *testing.T) {
	bc, err := SetupBlockchainForTesting()
	if err != nil {
		t.Fatal(err)
	}
	statedb, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}
	// Setup contract code into newly generated state
	address := common.HexToAddress("0x0a")
	statedb.SetCode(address, candidate_exchange_smc_code)
	abi, err := abi.JSON(strings.NewReader(candidate_exchange_smc_definition))
	if err != nil {
		t.Fatal(err)
	}
	// Create tx to request candidate info from external chain
	fulfillCandidateInfoInput, err := abi.Pack("fulfillCandidateInfo", "external@gmail.com", "external", uint8(20),
		common.HexToAddress("0x123"), "PV2", "org2", "org1")
	if err != nil {
		t.Fatal(err)
	}
	addrKeyBytes, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := tool.GenerateSmcCall(addrKey, address, fulfillCandidateInfoInput, state.ManageState(statedb))
	// Apply tx and get returned logs from that tx
	logs, err := ApplyTransactionReturnLog(bc, statedb, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if there is event emitted from previous tx
	if len(logs) == 0 {
		t.Error("Expect length of log > 0, 0 is returned")
	}
	var externalCandidateInfoFulfilled struct {
		Email     string
		Name      string
		Age       uint8
		Addr      common.Address
		Source    string
		FromOrgID string
		ToOrgID   string
	}
	err = abi.Unpack(&externalCandidateInfoFulfilled, "ExternalCandidateInfoFulfilled", logs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if externalCandidateInfoFulfilled.Email != "external@gmail.com" {
		t.Error("Expect requested email is external@gmail.com, got ", externalCandidateInfoFulfilled.Email)
	}
	if externalCandidateInfoFulfilled.Name != "external" {
		t.Error("Expect name is external, got ", externalCandidateInfoFulfilled.Name)
	}
	if externalCandidateInfoFulfilled.Age != uint8(20) {
		t.Error("Expect age is 20, got ", externalCandidateInfoFulfilled.Age)
	}
	if externalCandidateInfoFulfilled.Addr != common.HexToAddress("0x123") {
		t.Error("Exxect address is 0x123, got ", externalCandidateInfoFulfilled.Addr.String())
	}
	if externalCandidateInfoFulfilled.Source != "PV2" {
		t.Error("Expect source is PV2, got ", externalCandidateInfoFulfilled.Source)
	}
	if externalCandidateInfoFulfilled.FromOrgID != "org2" {
		t.Error("Expect from org2, got ", externalCandidateInfoFulfilled.FromOrgID)
	}
	if externalCandidateInfoFulfilled.ToOrgID != "org1" {
		t.Error("Expect to org1, got ", externalCandidateInfoFulfilled.ToOrgID)
	}
}
