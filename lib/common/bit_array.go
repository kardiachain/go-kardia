/*
 *  Copyright 2018 KardiaChain
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

package common

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	kprotobits "github.com/kardiachain/go-kardiamain/proto/kardiachain/libs/bits"
)

type BitArray struct {
	mtx   sync.Mutex
	Bits  uint     `json:"bits"`  // NOTE: persisted via reflect, must be exported
	Elems []uint64 `json:"elems"` // NOTE: persisted via reflect, must be exported
}

// There is no BitArray whose Size is 0.  Use nil instead.
func NewBitArray(bits int) *BitArray {
	if bits <= 0 {
		return nil
	}
	return &BitArray{
		Bits:  uint(bits),
		Elems: make([]uint64, (bits+63)/64),
	}
}

func (bA *BitArray) Size() int {
	if bA == nil {
		return 0
	}
	return int(bA.Bits)
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA *BitArray) GetIndex(i int) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.getIndex(i)
}

func (bA *BitArray) getIndex(i int) bool {
	if i >= int(bA.Bits) {
		return false
	}
	return bA.Elems[i/64]&(uint64(1)<<uint(i%64)) > 0
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA *BitArray) SetIndex(i int, v bool) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.setIndex(i, v)
}

func (bA *BitArray) setIndex(i int, v bool) bool {
	if i >= int(bA.Bits) {
		return false
	}
	if v {
		bA.Elems[i/64] |= uint64(1) << uint(i%64)
	} else {
		bA.Elems[i/64] &= ^(uint64(1) << uint(i%64))
	}
	return true
}

func (bA *BitArray) Copy() *BitArray {
	if bA == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.copy()
}

func (bA *BitArray) copy() *BitArray {
	c := make([]uint64, len(bA.Elems))
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bA.Bits,
		Elems: c,
	}
}

func (bA *BitArray) copyBits(bits int) *BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  uint(bits),
		Elems: c,
	}
}

// Returns a BitArray of larger bits size.
func (bA *BitArray) Or(o *BitArray) *BitArray {
	if bA == nil && o == nil {
		return nil
	}
	if bA == nil && o != nil {
		return o.Copy()
	}
	if o == nil {
		return bA.Copy()
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	c := bA.copyBits(MaxInt(int(bA.Bits), int(o.Bits)))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] |= o.Elems[i]
	}
	return c
}

// Returns a BitArray of smaller bit size.
func (bA *BitArray) And(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.and(o)
}

func (bA *BitArray) and(o *BitArray) *BitArray {
	c := bA.copyBits(MinInt(int(bA.Bits), int(o.Bits)))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] &= o.Elems[i]
	}
	return c
}

func (bA *BitArray) Not() *BitArray {
	if bA == nil {
		return nil // Degenerate
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	c := bA.copy()
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] = ^c.Elems[i]
	}
	return c
}

func (bA *BitArray) Sub(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		// TODO: Decide if we should do 1's complement here?
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	if bA.Bits > o.Bits {
		c := bA.copy()
		for i := 0; i < len(o.Elems)-1; i++ {
			c.Elems[i] &= ^c.Elems[i]
		}
		i := len(o.Elems) - 1
		if i >= 0 {
			for idx := i * 64; idx < int(o.Bits); idx++ {
				// NOTE: each individual GetIndex() call to o is safe.
				c.setIndex(idx, c.getIndex(idx) && !o.GetIndex(idx))
			}
		}
		return c
	}
	return bA.and(o.Not()) // Note degenerate case where o == nil
}

func (bA *BitArray) IsEmpty() bool {
	if bA == nil {
		return true // should this be opposite?
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	for _, e := range bA.Elems {
		if e > 0 {
			return false
		}
	}
	return true
}

func (bA *BitArray) IsFull() bool {
	if bA == nil {
		return true
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	// Check all elements except the last
	for _, elem := range bA.Elems[:len(bA.Elems)-1] {
		if (^elem) != 0 {
			return false
		}
	}

	// Check that the last element has (lastElemBits) 1's
	lastElemBits := (bA.Bits+63)%64 + 1
	lastElem := bA.Elems[len(bA.Elems)-1]
	return (lastElem+1)&((uint64(1)<<uint(lastElemBits))-1) == 0
}

// PickRandom returns a random index in the bit array, and its value.
// It uses the global randomness in `random.go` to get this index.
func (bA *BitArray) PickRandom() (int, bool) {
	if bA == nil {
		return 0, false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	length := len(bA.Elems)
	if length == 0 {
		return 0, false
	}
	randElemStart := RandIntn(length)
	for i := 0; i < length; i++ {
		elemIdx := (i + randElemStart) % length
		if elemIdx < length-1 {
			if bA.Elems[elemIdx] > 0 {
				randBitStart := RandIntn(64)
				for j := 0; j < 64; j++ {
					bitIdx := (j + randBitStart) % 64
					if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
						return 64*elemIdx + bitIdx, true
					}
				}
				PanicSanity("should not happen")
			}
		} else {
			// Special case for last elem, to ignore straggler bits
			elemBits := bA.Bits % 64
			if elemBits == 0 {
				elemBits = 64
			}
			randBitStart := RandIntn(int(elemBits))
			for j := 0; j < int(elemBits); j++ {
				bitIdx := (j + randBitStart) % int(elemBits)
				if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
					return 64*elemIdx + bitIdx, true
				}
			}
		}
	}
	return 0, false
}

// String returns a string representation of BitArray: BA{<bit-string>},
// where <bit-string> is a sequence of 'x' (1) and '_' (0).
// The <bit-string> includes spaces and newlines to help people.
// Example: "BA{_x_}" or "nil-BitArray" for nil.
func (bA *BitArray) String() string {
	if bA == nil {
		return "nil-BitArray"
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	lines := []string{}
	bits := ""
	for i := 0; i < int(bA.Bits); i++ {
		if bA.getIndex(i) {
			bits += "x"
		} else {
			bits += "_"
		}
		if i%100 == 99 {
			lines = append(lines, bits)
			bits = ""
		}
		if i%10 == 9 {
			bits += "  "
		}
		if i%50 == 49 {
			bits += "  "
		}
	}
	if len(bits) > 0 {
		lines = append(lines, bits)
	}
	return fmt.Sprintf("BA{%v:%v}", bA.Bits, strings.Join(lines, " "))

}

func (bA *BitArray) Bytes() []byte {
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	numBytes := (bA.Bits + 7) / 8
	bytes := make([]byte, numBytes)
	for i := 0; i < len(bA.Elems); i++ {
		elemBytes := [8]byte{}
		binary.LittleEndian.PutUint64(elemBytes[:], bA.Elems[i])
		copy(bytes[i*8:], elemBytes[:])
	}
	return bytes
}

// NOTE: other bitarray o is not locked when reading,
// so if necessary, caller must copy or lock o prior to calling Update.
// If bA is nil, does nothing.
func (bA *BitArray) Update(o *BitArray) {
	if bA == nil || o == nil {
		return
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	copy(bA.Elems, o.Elems)
}

// ToProto converts BitArray to protobuf
func (bA *BitArray) ToProto() *kprotobits.BitArray {
	if bA == nil || len(bA.Elems) == 0 {
		return nil
	}

	return &kprotobits.BitArray{
		Bits:  int64(bA.Bits),
		Elems: bA.Elems,
	}
}

// FromProto sets a protobuf BitArray to the given pointer.
func (bA *BitArray) FromProto(protoBitArray *kprotobits.BitArray) {
	if protoBitArray == nil {
		bA = nil
		return
	}

	bA.Bits = uint(protoBitArray.Bits)
	if len(protoBitArray.Elems) > 0 {
		bA.Elems = protoBitArray.Elems
	}
}
