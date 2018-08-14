package account


import (
	"testing"
	"path/filepath"
	"os"
)


const (
	password = "KardiaChain"
)


func TestKeyStore(t *testing.T) {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	keystore := KeyStore{Path: dir}
	_ , err := keystore.createKeyStore(password)

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