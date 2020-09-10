/*
 *  Copyright 2019 KardiaChain
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

package merkle

import (
	"math/bits"
)

// SimpleHashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return leafHash(items[0])
	default:
		k := getSplitPoint(len(items))
		left := SimpleHashFromByteSlices(items[:k])
		right := SimpleHashFromByteSlices(items[k:])
		return innerHash(left, right)
	}
}

// SimpleHashFromMap computes a Merkle tree from sorted map.
// Like calling SimpleHashFromHashers with
// `item = []byte(Hash(key) | Hash(value))`,
// sorted by `item`.
func SimpleHashFromMap(m map[string][]byte) []byte {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}

// getSplitPoint returns the largest power of 2 less than length
func getSplitPoint(length int) int {
	if length < 1 {
		panic("Trying to split a tree with size < 1")
	}
	uLength := uint(length)
	bitlen := bits.Len(uLength)
	k := 1 << uint(bitlen-1)
	if k == length {
		k >>= 1
	}
	return k
}
