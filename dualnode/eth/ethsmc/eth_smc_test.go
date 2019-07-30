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

package ethsmc

import (
	"math/big"
	"testing"
)

func TestEthSmcDepositUnpack(t *testing.T) {
	smc := NewEthSmc()

	neoAddr := "AddZkjqPoPyhDhWoA8f9CXQeHQRDr8HbPo"
	destination := "ETH-NEO"
	inputBytes, err := smc.ethABI.Pack("deposit", neoAddr, destination)
	if err != nil {
		t.Fatalf("Cannot pack Eth method call deposit: %v", err)
	}

	method, err := smc.InputMethodName(inputBytes)
	if err != nil {
		t.Fatalf("ABI fail to parse method name: %v ", err)
	}
	if method != "deposit" {
		t.Fatalf("Parsed Method name mismatched: Expected 'deposit', see: %v", method)
	}

	receiver, unpackedDestination, err := smc.UnpackDepositInput(inputBytes)
	if err != nil {
		t.Fatalf("ETH ABI fail to unpack input: %v", err)
	}
	if unpackedDestination != destination {
		t.Fatalf("Unpacked param mismatched: Expected: %v, See: %v", destination, unpackedDestination)
	}
	if receiver != neoAddr {
		t.Fatalf("Unpacked param mismatched: Expected: %v, See: %v", neoAddr, receiver)
	}
}

func TestEthSmc_packReleaseInput(t *testing.T) {
	smc := NewEthSmc()
	inputBytes := smc.packReleaseInput("ethreceiver", big.NewInt(100000000000000000))
	method, err := smc.InputMethodName(inputBytes)
	if err != nil {
		t.Fatalf("ABI fail to parse method name: %v ", err)
	}
	if method != "release" {
		t.Fatalf("Expected Method name 'release', see: %v", method)
	}
}

func TestEthSmc_CreateEthReleaseTx(t *testing.T) {
	smc := NewEthSmc()
	tx := smc.CreateEthReleaseTx(EthContractAddress, big.NewInt(100000000000000000), "eth receiver", 233)
	t.Logf("Created tx: %v", tx)
}
