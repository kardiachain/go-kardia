package ethsmc

import (
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/kardiachain/go-kardia/abi"
	"strings"
	"testing"
)

func TestEthSmcInputUnpack(t *testing.T) {
	// Eth contract, so use Eth ABI to pack instead of Kardia.
	ethAbi, err := ethabi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		t.Errorf("Geth ABI library fail to read abi def: %v", err)
	}

	neoAddr := "AddZkjqPoPyhDhWoA8f9CXQeHQRDr8HbPo"

	inputBytes, err := ethAbi.Pack("deposit", neoAddr)
	if err != nil {
		t.Errorf("Cannot pack Eth method call deposit: %v", err)
	}

	// Geth doesn't have func to unpack input, must use Kardia ABI library.
	kAbi, err := abi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		t.Errorf("Kardia ABI library fail to read abi def: %v", err)
	}

	var param string

	method, err := kAbi.MethodById(inputBytes[0:4])
	if err != nil {
		t.Errorf("ABI fail to parse method name: %v ", err)
	}
	if method.Name != "deposit" {
		t.Errorf("Parsed Method name mismatched: Expected 'deposit', see: %v", method.Name)
	}

	if err := kAbi.UnpackInput(&param, method.Name, inputBytes[4:]); err != nil {
		t.Errorf("ABI fail to unpack input: %v", err)
	}
	if param != neoAddr {
		t.Errorf("Unpacked param mismatched: Expected: %v, See: %v", neoAddr, param)
	}
}
