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

package account

import (
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

const (
	password          = "KardiaChain"
	expectedEncodedTx = "0xf86103018207d094c1fe56e3f58d3244f606306611a5d10c8333f1f60a8255441ca0428dbfc24e8c6ed2b458af901e03afb2aac83b0fd2b62670237061368bfee2f2a0731a84afbb6cefdff9416abc4a918f539662a5a6f0a1884e1dcb18798c3b314d"
)

func TestKeyStore(t *testing.T) {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	keystore := KeyStore{Path: dir}
	err := keystore.createKeyStore(password, "")

	if err != nil {
		t.Error(err)
	}

	address := keystore.Address

	// do login with password and new address
	keystore1 := KeyStore{Path: dir, Address: address}
	err = keystore1.GetKey(password)

	if err != nil {
		t.Error(err)
	}

	if keystore1.PrivateKey.D.Int64() != keystore.PrivateKey.D.Int64() {
		t.Error("private key does not match")
	}

}

func TestSignTx(t *testing.T) {
	keystore := KeyStore{Path: ""}
	_, err := keystore.NewKeyStoreJSON(password, "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	if err != nil {
		t.Error(err)
	}

	tx := types.NewTransaction(
		3,
		common.HexToAddress("c1fe56E3F58D3244F606306611a5d10c8333f1f6"),
		big.NewInt(10),
		2000,
		big.NewInt(1),
		common.FromHex("5544"),
	)
	tx, _ = types.SignTx(tx, &keystore.PrivateKey)
	b, _ := rlp.EncodeToBytes(tx)
	if common.Encode(b) != expectedEncodedTx {
		t.Error("Encoded Tx does not match with expected Tx")
	}
}
