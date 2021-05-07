// Copyright 2015 The go-ethereum Authors
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

package rpc

import (
	"encoding/json"
	"testing"

	"github.com/kardiachain/go-kardia/lib/common"
)

func TestBlockHeightJSONUnmarshal(t *testing.T) {
	tests := []struct {
		input    string
		mustFail bool
		expected BlockHeight
	}{
		0:  {`"0x"`, true, BlockHeight(0)},
		1:  {`"0x0"`, true, BlockHeight(0)},
		2:  {`"0X1"`, true, BlockHeight(0)},
		3:  {`"0x01"`, true, BlockHeight(0)},
		4:  {`"0x12"`, true, BlockHeight(0)},
		5:  {`"ff"`, true, BlockHeight(0)},
		6:  {`0`, false, BlockHeight(0)},
		7:  {`"pending"`, false, PendingBlockHeight},
		8:  {`"latest"`, false, LatestBlockHeight},
		9:  {`"earliest"`, false, EarliestBlockHeight},
		10: {`someString`, true, BlockHeight(0)},
		11: {`""`, true, BlockHeight(0)},
		12: {``, true, BlockHeight(0)},
	}

	for i, test := range tests {
		var num BlockHeight
		err := json.Unmarshal([]byte(test.input), &num)
		if test.mustFail && err == nil {
			t.Errorf("Test %d should fail", i)
			continue
		}
		if !test.mustFail && err != nil {
			t.Errorf("Test %d should pass but got err: %v", i, err)
			continue
		}
		if num != test.expected {
			t.Errorf("Test %d got unexpected value, want %d, got %d", i, test.expected, num)
		}
	}
}

func TestBlockHeightOrHash_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input    string
		mustFail bool
		expected BlockHeightOrHash
	}{
		0:  {`"0x"`, true, BlockHeightOrHash{}},
		1:  {`"0x0"`, true, BlockHeightOrHash{}},
		2:  {`"0X1"`, true, BlockHeightOrHash{}},
		3:  {`"0x01"`, true, BlockHeightOrHash{}},
		4:  {`"0x12"`, true, BlockHeightOrHash{}},
		5:  {`0`, false, BlockHeightOrHashWithNumber(0)},
		6:  {`"ff"`, true, BlockHeightOrHash{}},
		7:  {`"pending"`, false, BlockHeightOrHashWithNumber(PendingBlockHeight)},
		8:  {`"latest"`, false, BlockHeightOrHashWithNumber(LatestBlockHeight)},
		9:  {`"earliest"`, false, BlockHeightOrHashWithNumber(EarliestBlockHeight)},
		10: {`someString`, true, BlockHeightOrHash{}},
		11: {`""`, true, BlockHeightOrHash{}},
		12: {``, true, BlockHeightOrHash{}},
		13: {`"0x0000000000000000000000000000000000000000000000000000000000000000"`, false, BlockHeightOrHashWithHash(common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"), false)},
		14: {`{"blockHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`, false, BlockHeightOrHashWithHash(common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"), false)},
		15: {`{"blockHash":"0x0000000000000000000000000000000000000000000000000000000000000000","requireCanonical":false}`, false, BlockHeightOrHashWithHash(common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"), false)},
		16: {`{"blockHash":"0x0000000000000000000000000000000000000000000000000000000000000000","requireCanonical":true}`, false, BlockHeightOrHashWithHash(common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"), true)},
		17: {`{"BlockHeight":1}`, false, BlockHeightOrHashWithNumber(1)},
		18: {`{"BlockHeight":"pending"}`, false, BlockHeightOrHashWithNumber(PendingBlockHeight)},
		19: {`{"BlockHeight":"latest"}`, false, BlockHeightOrHashWithNumber(LatestBlockHeight)},
		20: {`{"BlockHeight":"earliest"}`, false, BlockHeightOrHashWithNumber(EarliestBlockHeight)},
		21: {`{"BlockHeight":"0x1", "blockHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`, true, BlockHeightOrHash{}},
	}

	for i, test := range tests {
		var bnh BlockHeightOrHash
		err := json.Unmarshal([]byte(test.input), &bnh)
		if test.mustFail && err == nil {
			t.Errorf("Test %d should fail", i)
			continue
		}
		if !test.mustFail && err != nil {
			t.Errorf("Test %d should pass but got err: %v", i, err)
			continue
		}
		hash, hashOk := bnh.Hash()
		expectedHash, expectedHashOk := test.expected.Hash()
		num, numOk := bnh.Height()
		expectedNum, expectedNumOk := test.expected.Height()
		if bnh.RequireCanonical != test.expected.RequireCanonical ||
			hash != expectedHash || hashOk != expectedHashOk ||
			num != expectedNum || numOk != expectedNumOk {
			t.Errorf("Test %d got unexpected value, want %v, got %v", i, test.expected, bnh)
		}
	}
}
