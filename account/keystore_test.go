package account

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	password = "KardiaChain"
)

func TestKeyStore(t *testing.T) {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	keystore := KeyStore{Path: dir}
	err := keystore.createKeyStore(password, nil)

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
