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

package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

func TestToBlock(t *testing.T) {
	b := types.NewBlock(&types.Header{
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: 0,
			},
		},
	}, nil, nil, nil, nil)
	mgoB := NewBlock(b).ToBlock()
	assert.Equal(t, mgoB.Hash(), b.Hash())

}
