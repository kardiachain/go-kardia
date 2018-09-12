package ethsmc

import (
	"testing"
)

func TestEthSmcInputUnpack(t *testing.T) {
	smc := NewEthSmc()

	neoAddr := "AddZkjqPoPyhDhWoA8f9CXQeHQRDr8HbPo"

	inputBytes, err := smc.ethABI.Pack("deposit", neoAddr)
	if err != nil {
		t.Errorf("Cannot pack Eth method call deposit: %v", err)
	}

	method, err := smc.InputMethodName(inputBytes)
	if err != nil {
		t.Errorf("ABI fail to parse method name: %v ", err)
	}
	if method != "deposit" {
		t.Errorf("Parsed Method name mismatched: Expected 'deposit', see: %v", method)
	}

	param, err := smc.UnpackDepositInput(inputBytes)
	if err != nil {
		t.Errorf("Kardia API fail to unpack input: %v", err)
	}
	if param != neoAddr {
		t.Errorf("Unpacked param mismatched: Expected: %v, See: %v", neoAddr, param)
	}
}
