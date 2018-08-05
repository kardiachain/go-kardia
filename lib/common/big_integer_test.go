package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/kardiachain/go-kardia/lib/rlp"
)

var encodingTests = []struct {
	input    int64
	expected string
}{
	{10, "C20A01"},
	{123, "C27B01"},
	{-10, "C20A80"},
}

func TestRlpEncodeDecode(t *testing.T) {
	for i, test := range encodingTests {
		x := NewBigInt(test.input)
		encoded, err := rlp.EncodeToBytes(x)
		if err != nil {
			t.Errorf("test %d: encoding %v failed", i, test.input)
		} else if !bytes.Equal(encoded, unhex(test.expected)) {
			t.Errorf("test %d: encoding %X doesn't match %#v", i, encoded, test.expected)
		} else {
			t.Logf("test %d: encoding %v of %T type as %X matches %#v", i, x, x, encoded, test.expected)
		}

	}
}

func unhex(str string) []byte {
	b, err := hex.DecodeString(str)
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %q", str))
	}
	return b
}
