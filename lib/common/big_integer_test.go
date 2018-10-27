// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

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
		x := NewBigInt64(test.input)
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
