package ethsmc

import (
	"math/big"
	"testing"
)

func TestEthSmcDepositUnpack(t *testing.T) {
	smc := NewEthSmc()

	neoAddr := "AddZkjqPoPyhDhWoA8f9CXQeHQRDr8HbPo"

	inputBytes, err := smc.ethABI.Pack("deposit", neoAddr)
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

	param, err := smc.UnpackDepositInput(inputBytes)
	if err != nil {
		t.Fatalf("Kardia API fail to unpack input: %v", err)
	}
	if param != neoAddr {
		t.Fatalf("Unpacked param mismatched: Expected: %v, See: %v", neoAddr, param)
	}
}

func TestEthSmc_packReleaseInput(t *testing.T) {
	smc := NewEthSmc()
	inputBytes := smc.packReleaseInput(big.NewInt(100000000000000000))

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
	tx := smc.CreateEthReleaseTx(big.NewInt(100000000000000000), 233)

	t.Logf("Created tx: %v", tx)
}
