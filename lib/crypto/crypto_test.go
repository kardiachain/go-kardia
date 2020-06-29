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

package crypto

import (
	"bytes"
	"encoding/hex"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"testing"
)

func TestKeccak256Hash(t *testing.T) {
	msg := []byte("abc")
	exp, _ := hex.DecodeString("4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45")
	verifyHash(t, "Sha3-256-array", func(in []byte) []byte { h := Keccak256Hash(in); return h[:] }, msg, exp)
}

func TestNewContractAddress(t *testing.T) {
	testAddrHex := "970e8128ab834e8eac17ab8e3812f010678cf791"
	testPrivHex := "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"

	key, _ := HexToECDSA(testPrivHex)
	addr := common.HexToAddress(testAddrHex)
	genAddr := PubkeyToAddress(key.PublicKey)
	verifyAddr(t, genAddr, addr)

	addr0 := CreateAddress(addr, 0)
	addr1 := CreateAddress(addr, 1)

	verifyAddr(t, common.HexToAddress("333c3310824b7c685133f2bedb2ca4b8b4df633d"), addr0)
	verifyAddr(t, common.HexToAddress("8bda78331c916a08481428e4b07c96d3e916d165"), addr1)

}

func verifyHash(t *testing.T, name string, f func([]byte) []byte, msg, exp []byte) {
	sum := f(msg)
	if !bytes.Equal(exp, sum) {
		t.Fatalf("hash %s mismatch: want: %x have: %x", name, exp, sum)
	}
}

func verifyAddr(t *testing.T, addr0, addr1 common.Address) {
	if addr0 != addr1 {
		t.Fatalf("address mismatch: want: %x have: %x", addr0, addr1)
	}
}

func TestStringToPublicKey(t *testing.T) {
	pubString := "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860"
	expectedAddress := "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"

	pub, err := StringToPublicKey(pubString)
	if err != nil {
		t.Fatal(err)
	}

	if PubkeyToAddress(*pub).Hex() != expectedAddress {
		t.Fatal("Address does not match")
	}
}

func TestStringToPrivateKey(t *testing.T) {
	privateString := "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06"
	expectedAddress := "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"

	priv, err := StringToPrivateKey(privateString)
	if err != nil {
		t.Fatal(err)
	}

	if PubkeyToAddress(priv.PublicKey).Hex() != expectedAddress {
		t.Fatal("Address does not match")
	}
}
