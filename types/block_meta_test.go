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
	"github.com/kardiachain/go-kardiamain/lib/merkle"
	krand "github.com/kardiachain/go-kardiamain/lib/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBlockMeta_ToProto(t *testing.T) {
	h := createHeaderRandom()
	bi := BlockID{Hash: h.Hash(), PartsHeader: PartSetHeader{Total: 123, Hash: krand.Hash(merkle.Size)}}

	bm := &BlockMeta{
		BlockID: bi,
		Header:  h,
	}

	tests := []struct {
		testName string
		bm       *BlockMeta
		expErr   bool
	}{
		{"success", bm, false},
		{"failure nil", nil, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb := tt.bm.ToProto()

			bm, err := BlockMetaFromProto(pb)

			if !tt.expErr {
				require.NoError(t, err, tt.testName)
				require.Equal(t, tt.bm, bm, tt.testName)
			} else {
				require.Error(t, err, tt.testName)
			}
		})
	}
}

func TestBlockMeta_ValidateBasic(t *testing.T) {
	h := createHeaderRandom()
	bi := BlockID{Hash: h.Hash(), PartsHeader: PartSetHeader{Total: 123, Hash: krand.Hash(merkle.Size)}}
	bi2 := BlockID{Hash: krand.Hash(merkle.Size), PartsHeader: PartSetHeader{Total: 123, Hash: krand.Hash(merkle.Size)}}
	bi3 := BlockID{Hash: krand.Hash(merkle.Size), PartsHeader: PartSetHeader{Total: 123, Hash: krand.Hash(100)}} // incorrect size

	bm := &BlockMeta{
		BlockID: bi,
		Header:  h,
	}

	bm2 := &BlockMeta{
		BlockID: bi2,
		Header:  h,
	}

	bm3 := &BlockMeta{
		BlockID: bi3,
		Header:  h,
	}

	tests := []struct {
		name    string
		bm      *BlockMeta
		wantErr bool
	}{
		{"success", bm, false},
		{"failure wrong blockID hash", bm2, true},
		{"failure wrong length blockID hash", bm3, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.bm.ValidateBasic(); (err != nil) != tt.wantErr {
				t.Errorf("BlockMeta.ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
