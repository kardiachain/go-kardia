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

package utils

import (
	"github.com/kardiachain/go-kardia/kai/state"
	kaidb "github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"strings"
	"testing"

	"bytes"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/log"
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

// TestCreateForwardRequestTx checks if tx to request info contains correct input
func TestCreateForwardRequestTx(t *testing.T) {
	statedb, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	abi, err := abi.JSON(strings.NewReader(candidate_exchange_smc_definition))
	if err != nil {
		t.Fatal(err)
	}
	expectedInput, err := abi.Pack(configs.KardiaForwardRequestFunction, "a@gmail.com", "org1", "org2")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := CreateForwardRequestTx("a@gmail.com", "org1", "org2", state.ManageState(statedb))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(tx.Data(), expectedInput) != 0 {
		t.Error("Wrong input, expected ", string(expectedInput), " got ", string(tx.Data()))
	}
}

// TestCreateForwardResponseTx checks if tx to fulfill candidate info containa correct input
func TestCreateForwardResponseTx(t *testing.T) {
	statedb, _ := state.New(log.New(), common.Hash{}, state.NewDatabase(kaidb.NewMemStore()))
	abi, err := abi.JSON(strings.NewReader(candidate_exchange_smc_definition))
	if err != nil {
		t.Fatal(err)
	}
	expectedInput, err := abi.Pack(configs.KardiaForwardResponseFunction, "external@gmail.com", "response1", "org2", "org1")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := CreateForwardResponseTx("external@gmail.com", "response1",  "org2", "org1", state.ManageState(statedb))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(tx.Data(), expectedInput) != 0 {
		t.Error("Wrong input, expected ", common.Encode(expectedInput), " got ", common.Encode(tx.Data()))
	}
}
