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
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
)

var candidate_exchange_smc_code = common.Hex2Bytes("60806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630e40683614610051578063912991d314610146575b600080fd5b34801561005d57600080fd5b50610144600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610281565b005b34801561015257600080fd5b5061027f600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506103fc565b005b7f3ca643a7086eb63dfb0e5f2cec44808b9487badc9a643e9eaae2415149fb833c83838360405180806020018060200180602001848103845287818151815260200191508051906020019080838360005b838110156102ed5780820151818401526020810190506102d2565b50505050905090810190601f16801561031a5780820380516001836020036101000a031916815260200191505b50848103835286818151815260200191508051906020019080838360005b83811015610353578082015181840152602081019050610338565b50505050905090810190601f1680156103805780820380516001836020036101000a031916815260200191505b50848103825285818151815260200191508051906020019080838360005b838110156103b957808201518184015260208101905061039e565b50505050905090810190601f1680156103e65780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390a1505050565b7f90affc9ed2543eb1fb9de02387ab117d255429f9f5c25458d725cc772bc7221f848484846040518080602001806020018060200180602001858103855289818151815260200191508051906020019080838360005b8381101561046d578082015181840152602081019050610452565b50505050905090810190601f16801561049a5780820380516001836020036101000a031916815260200191505b50858103845288818151815260200191508051906020019080838360005b838110156104d35780820151818401526020810190506104b8565b50505050905090810190601f1680156105005780820380516001836020036101000a031916815260200191505b50858103835287818151815260200191508051906020019080838360005b8381101561053957808201518184015260208101905061051e565b50505050905090810190601f1680156105665780820380516001836020036101000a031916815260200191505b50858103825286818151815260200191508051906020019080838360005b8381101561059f578082015181840152602081019050610584565b50505050905090810190601f1680156105cc5780820380516001836020036101000a031916815260200191505b509850505050505050505060405180910390a1505050505600a165627a7a72305820aaf69639545c4279771ad00169e14a25b6d1d929396974fc4d1fa08cf2af26440029")
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
		"name": "forwardRequest",
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
				"name": "_response",
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
		"name": "forwardResponse",
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
		"name": "IncomingRequest",
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
				"name": "response",
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
		"name": "FulfilledRequest",
		"type": "event"
	}
]`

// TestForwardRequest tests if a tx sent to Kardia candidate info exchange contract to request info fires
// correct event and returns correct data (email, fromOrgId, toOrdId)
func TestForwardRequest(t *testing.T) {
	bc, txPool, err := SetupBlockchainForTesting()
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
	forwardRequestInput, err := abi.Pack("forwardRequest", "a@gmail.com", "org1", "org2")
	if err != nil {
		t.Fatal(err)
	}
	addrKeyBytes, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := tx_pool.GenerateSmcCall(addrKey, address, forwardRequestInput, txPool, false)
	// Apply tx and get returned logs from that tx
	logs, err := ApplyTransactionReturnLog(bc, statedb, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if there is event emitted from previous tx
	if len(logs) == 0 {
		t.Error("Expect length of log > 0, 0 is returned")
	}
	var incomingRequest struct {
		Email     string
		FromOrgID string
		ToOrgID   string
	}
	//input := common.FromHex("000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000b6140676d61696c2e636f6d0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003323130000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000033232300000000000000000000000000000000000000000000000000000000000")
	err = abi.UnpackIntoInterface(&incomingRequest, "IncomingRequest", logs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if incomingRequest.Email != "a@gmail.com" {
		t.Error("Expect requested email is a@gmail.com, got ", incomingRequest.Email)
	}
	if incomingRequest.FromOrgID != "org1" {
		t.Error("Expect from org1, got ", incomingRequest.FromOrgID)
	}
	if incomingRequest.ToOrgID != "org2" {
		t.Error("Expect to org2, got ", incomingRequest.ToOrgID)
	}
}

// TestForwardResponse tests if a tx sent to Kardia candidate info exchange contract to send candidate info fires
// correct event and returns correct data (email, fromOrgId, toOrdId)
func TestForwardResponse(t *testing.T) {
	bc, txPool, err := SetupBlockchainForTesting()
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
	forwardResponseInput, err := abi.Pack("forwardResponse", "external@gmail.com", "response1", "org2", "org1")
	if err != nil {
		t.Fatal(err)
	}
	addrKeyBytes, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := tx_pool.GenerateSmcCall(addrKey, address, forwardResponseInput, txPool, false)
	// Apply tx and get returned logs from that tx
	logs, err := ApplyTransactionReturnLog(bc, statedb, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if there is event emitted from previous tx
	if len(logs) == 0 {
		t.Error("Expect length of log > 0, 0 is returned")
	}
	var fulfilledRequest struct {
		Email     string
		Response  string
		FromOrgID string
		ToOrgID   string
	}
	err = abi.UnpackIntoInterface(&fulfilledRequest, "FulfilledRequest", logs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if fulfilledRequest.Email != "external@gmail.com" {
		t.Error("Expect requested email is external@gmail.com, got ", fulfilledRequest.Email)
	}
	if fulfilledRequest.Response != "response1" {
		t.Error("Expect name is external, got ", fulfilledRequest.Response)
	}
	if fulfilledRequest.FromOrgID != "org2" {
		t.Error("Expect from org2, got ", fulfilledRequest.FromOrgID)
	}
	if fulfilledRequest.ToOrgID != "org1" {
		t.Error("Expect to org1, got ", fulfilledRequest.ToOrgID)
	}
}
