/*
 *  Copyright 2020 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package types

import (
	"io/ioutil"
	"testing"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodePart(t *testing.T) {
	block := CreateNewBlock(1)
	partsSet := block.MakePartSet(BlockPartSizeBytes)

	partBytes, err := rlp.EncodeToBytes(partsSet.GetPart(0))

	if err != nil {
		t.Fatal(err)
	}

	decoded := &Part{}

	if err := rlp.DecodeBytes(partBytes, decoded); err != nil {
		t.Fatal(err)
	}

}
func TestBasicPartSet(t *testing.T) {
	// Construct random data of size partSize * 100
	const COUNT = 100
	data := cmn.RandBytes(BlockPartSizeBytes * COUNT)
	partSet := NewPartSetFromData(data, BlockPartSizeBytes)

	assert.NotEmpty(t, partSet.Hash())
	assert.EqualValues(t, COUNT, partSet.Total())
	assert.Equal(t, COUNT, partSet.BitArray().Size())
	assert.True(t, partSet.HashesTo(partSet.Hash()))
	assert.True(t, partSet.IsComplete())
	assert.EqualValues(t, COUNT, partSet.Count())

	// Test adding parts to a new partSet.
	partSet2 := NewPartSetFromHeader(partSet.Header())
	assert.True(t, partSet2.HasHeader(partSet.Header()))

	for i := 0; i < int(partSet.Total()); i++ {
		part := partSet.GetPart(i)
		added, err := partSet2.AddPart(part)
		if !added || err != nil {
			t.Errorf("failed to add part %v, error: %v", i, err)
		}
	}
	// adding part with invalid index
	added, err := partSet2.AddPart(&Part{Index: 10000})
	assert.False(t, added)
	assert.Error(t, err)

	// adding existing part
	added, err = partSet2.AddPart(partSet2.GetPart(0))
	assert.False(t, added)
	assert.Nil(t, err)

	assert.Equal(t, partSet.Hash(), partSet2.Hash())
	assert.EqualValues(t, COUNT, partSet2.Total())
	assert.True(t, partSet2.IsComplete())

	// Reconstruct data, assert that they are equal.
	data2Reader := partSet2.GetReader()
	data2, err := ioutil.ReadAll(data2Reader)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, data, data2)
}

func TestWrongProof(t *testing.T) {
	// Construct random data of size partSize * 100
	data := cmn.RandBytes(BlockPartSizeBytes * 100)
	partSet := NewPartSetFromData(data, BlockPartSizeBytes)

	// Test adding a part with wrong data.
	partSet2 := NewPartSetFromHeader(partSet.Header())

	// Test adding a part with wrong trail.
	part := partSet.GetPart(0)
	part.Proof.Aunts[0][0] += byte(0x01)
	added, err := partSet2.AddPart(part)
	if added || err == nil {
		t.Errorf("expected to fail adding a part with bad trail.")
	}

	// Test adding a part with wrong bytes.
	part = partSet.GetPart(1)
	part.Bytes[0] += byte(0x01)
	added, err = partSet2.AddPart(part)
	if added || err == nil {
		t.Errorf("expected to fail adding a part with bad bytes.")
	}
}

// func TestPartSetHeaderValidateBasic(t *testing.T) {
// 	testCases := []struct {
// 		testName              string
// 		malleatePartSetHeader func(*PartSetHeader)
// 		expectErr             bool
// 	}{
// 		{"Good PartSet", func(psHeader *PartSetHeader) {}, false},
// 		{"Invalid Hash", func(psHeader *PartSetHeader) { psHeader.Hash = make([]byte, 1) }, true},
// 	}
// 	for _, tc := range testCases {
// 		tc := tc
// 		t.Run(tc.testName, func(t *testing.T) {
// 			data := cmn.RandBytes(BlockPartSizeBytes * 100)
// 			ps := NewPartSetFromData(data, BlockPartSizeBytes)
// 			psHeader := ps.Header()
// 			tc.malleatePartSetHeader(&psHeader)
// 			assert.Equal(t, tc.expectErr, psHeader.ValidateBasic() != nil, "Validate Basic had an unexpected result")
// 		})
// 	}
// }

func TestPartValidateBasic(t *testing.T) {
	testCases := []struct {
		testName     string
		malleatePart func(*Part)
		expectErr    bool
	}{
		{"Good Part", func(pt *Part) {}, false},
		{"Too big part", func(pt *Part) { pt.Bytes = make([]byte, BlockPartSizeBytes+1) }, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			data := cmn.RandBytes(BlockPartSizeBytes * 100)
			ps := NewPartSetFromData(data, BlockPartSizeBytes)
			part := ps.GetPart(0)
			tc.malleatePart(part)
			assert.Equal(t, tc.expectErr, part.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
