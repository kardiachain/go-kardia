package account

import (
	"os"
	"path/filepath"
	"testing"
	"math/big"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

const (
	password = "KardiaChain"
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
