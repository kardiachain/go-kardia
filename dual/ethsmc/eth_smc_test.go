package ethsmc

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"testing"
)

func TestEthSmcDepositUnpack(t *testing.T) {
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

func TestEthSmc_packReleaseInput(t *testing.T) {
	smc := NewEthSmc()
	inputBytes := smc.packReleaseInput(big.NewInt(100000000000000000))
	inputHex := hexutil.Encode(inputBytes)

	expectedInput := "0x0357371d000000000000000000000000ff6781f2cc6f9b6b4a68a0afc3aae89133bbb236000000000000000000000000000000000000000000000000016345785d8a0000"

	if inputHex != expectedInput {
		t.Errorf("Unexpected packed release input: %v", inputHex)
	}
}

func TestEthSmc_CreateEthReleaseTx(t *testing.T) {
	smc := NewEthSmc()
	tx := smc.CreateEthReleaseTx(big.NewInt(100000000000000000), 233)

	t.Logf("Created tx: %v", tx)
}
