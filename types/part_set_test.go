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
	added, err := partSet2.AddPart(&Part{Index: cmn.NewBigInt64(10000)})
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

func TestPartSetHeaderValidateBasic(t *testing.T) {
	testCases := []struct {
		testName              string
		malleatePartSetHeader func(*PartSetHeader)
		expectErr             bool
	}{
		{"Good PartSet", func(psHeader *PartSetHeader) {}, false},
		{"Invalid Hash", func(psHeader *PartSetHeader) { psHeader.Hash = make([]byte, 1) }, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			data := cmn.RandBytes(BlockPartSizeBytes * 100)
			ps := NewPartSetFromData(data, BlockPartSizeBytes)
			psHeader := ps.Header()
			tc.malleatePartSetHeader(&psHeader)
			assert.Equal(t, tc.expectErr, psHeader.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
