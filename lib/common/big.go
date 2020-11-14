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
	"fmt"
	"math/big"
)

var (
	tt255   = BigPow(2, 255)
	tt256   = BigPow(2, 256)
	tt256m1 = new(big.Int).Sub(tt256, big.NewInt(1))
	tt63    = BigPow(2, 63)
)

const (
	// number of bits in a big.Word
	wordBits = 32 << (uint64(^big.Word(0)) >> 63)
	// number of bytes in a big.Word
	wordBytes = wordBits / 8
)

// HexOrDecimal256 marshals big.Int as hex or decimal.
type HexOrDecimal256 big.Int

// UnmarshalText implements encoding.TextUnmarshaler.
func (i *HexOrDecimal256) UnmarshalText(input []byte) error {
	bigint, ok := ParseBig256(string(input))
	if !ok {
		return fmt.Errorf("invalid hex or decimal integer %q", input)
	}
	*i = HexOrDecimal256(*bigint)
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (i *HexOrDecimal256) MarshalText() ([]byte, error) {
	if i == nil {
		return []byte("0x0"), nil
	}
	return []byte(fmt.Sprintf("%#x", (*big.Int)(i))), nil
}

// ParseBig256 parses s as a 256 bit integer in decimal or hexadecimal syntax.
// Leading zeros are accepted. The empty string parses as zero.
func ParseBig256(s string) (*big.Int, bool) {
	if s == "" {
		return new(big.Int), true
	}
	var bigint *big.Int
	var ok bool
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		bigint, ok = new(big.Int).SetString(s[2:], 16)
	} else {
		bigint, ok = new(big.Int).SetString(s, 10)
	}
	if ok && bigint.BitLen() > 256 {
		bigint, ok = nil, false
	}
	return bigint, ok
}

// MustParseBig256 parses s as a 256 bit big integer and panics if the string is invalid.
func MustParseBig256(s string) *big.Int {
	v, ok := ParseBig256(s)
	if !ok {
		panic("invalid 256 bit integer: " + s)
	}
	return v
}

// BigPow returns a ** b as a big integer.
func BigPow(a, b int64) *big.Int {
	r := big.NewInt(a)
	return r.Exp(r, big.NewInt(b), nil)
}

// BigMax returns the larger of x or y.
func BigMax(x, y *big.Int) *big.Int {
	if x.Cmp(y) < 0 {
		return y
	}
	return x
}

// BigMin returns the smaller of x or y.
func BigMin(x, y *big.Int) *big.Int {
	if x.Cmp(y) > 0 {
		return y
	}
	return x
}

// FirstBitSet returns the index of the first 1 bit in v, counting from LSB.
func FirstBitSet(v *big.Int) int {
	for i := 0; i < v.BitLen(); i++ {
		if v.Bit(i) > 0 {
			return i
		}
	}
	return v.BitLen()
}

// PaddedBigBytes encodes a big integer as a big-endian byte slice. The length
// of the slice is at least n bytes.
func PaddedBigBytes(bigint *big.Int, n int) []byte {
	if bigint.BitLen()/8 >= n {
		return bigint.Bytes()
	}
	ret := make([]byte, n)
	ReadBits(bigint, ret)
	return ret
}

// bigEndianByteAt returns the byte at position n,
// in Big-Endian encoding
// So n==0 returns the least significant byte
func bigEndianByteAt(bigint *big.Int, n int) byte {
	words := bigint.Bits()
	// Check word-bucket the byte will reside in
	i := n / wordBytes
	if i >= len(words) {
		return byte(0)
	}
	word := words[i]
	// Offset of the byte
	shift := 8 * uint(n%wordBytes)

	return byte(word >> shift)
}

// Byte returns the byte at position n,
// with the supplied padlength in Little-Endian encoding.
// n==0 returns the MSB
// Example: bigint '5', padlength 32, n=31 => 5
func Byte(bigint *big.Int, padlength, n int) byte {
	if n >= padlength {
		return byte(0)
	}
	return bigEndianByteAt(bigint, padlength-1-n)
}

// ReadBits encodes the absolute value of bigint as big-endian bytes. Callers must ensure
// that buf has enough space. If buf is too short the result will be incomplete.
func ReadBits(bigint *big.Int, buf []byte) {
	i := len(buf)
	for _, d := range bigint.Bits() {
		for j := 0; j < wordBytes && i > 0; j++ {
			i--
			buf[i] = byte(d)
			d >>= 8
		}
	}
}

// U256 encodes as a 256 bit two's complement number. This operation is destructive.
func U256(x *big.Int) *big.Int {
	return x.And(x, tt256m1)
}

// U256Bytes converts a big Int into a 256bit EVM number.
// This operation is destructive.
func U256Bytes(n *big.Int) []byte {
	return PaddedBigBytes(U256(n), 32)
}

// S256 interprets x as a two's complement number.
// x must not exceed 256 bits (the result is undefined if it does) and is not modified.
//
//   S256(0)        = 0
//   S256(1)        = 1
//   S256(2**255)   = -2**255
//   S256(2**256-1) = -1
func S256(x *big.Int) *big.Int {
	if x.Cmp(tt255) < 0 {
		return x
	}
	return new(big.Int).Sub(x, tt256)
}

// Exp implements exponentiation by squaring.
// Exp returns a newly-allocated big integer and does not change
// base or exponent. The result is truncated to 256 bits.
//
// Courtesy @karalabe and @chfast
func Exp(base, exponent *big.Int) *big.Int {
	result := big.NewInt(1)

	for _, word := range exponent.Bits() {
		for i := 0; i < wordBits; i++ {
			if word&1 == 1 {
				U256(result.Mul(result, base))
			}
			U256(base.Mul(base, base))
			word >>= 1
		}
	}
	return result
}

// BigInt struct
type BigInt struct {
	bigint *big.Int
}

// NewBigInt allocates and returns a new BigInt set to x.
func NewBigInt(x int64) *BigInt {
	return &BigInt{big.NewInt(x)}
}

// SetInt64 sets the big int to x.
func (x *BigInt) SetInt64(i int64) {
	x.bigint.SetInt64(i)
}

// GetInt64 returns the int64 representation of x. If x cannot be represented in
// an int64, the result is undefined.
func (x *BigInt) GetInt64() int64 {
	return x.bigint.Int64()
}

// SetUint64 sets the big uint to x.
func (x *BigInt) SetUint64(i uint64) {
	x.bigint.SetUint64(i)
}

// GetUint64 returns the uint64 representation of x. If x cannot be represented in
// an uint64, the result is undefined.
func (x *BigInt) GetUint64() uint64 {
	return x.bigint.Uint64()
}

// IsGreaterThan returns true if x is greater than y
func (x *BigInt) IsGreaterThan(y *BigInt) bool {
	return x.GetInt64() > y.GetInt64()
}

// IsGreaterThanOrEqual returns true if x is greater than or equals y
func (x *BigInt) IsGreaterThanOrEqual(y *BigInt) bool {
	return x.GetInt64() >= y.GetInt64()
}

// IsGreaterThanInt returns true if x is greater than y
func (x *BigInt) IsGreaterThanInt(y int64) bool {
	return x.GetInt64() > y
}

// IsGreaterThanUint returns true if x is greater than y
func (x *BigInt) IsGreaterThanUint(y uint64) bool {
	return x.GetUint64() > y
}

// IsGreaterThanOrEqualToInt returns true if x is greater than or equals to y
func (x *BigInt) IsGreaterThanOrEqualToInt(y int64) bool {
	return x.GetInt64() >= y
}

// IsGreaterThanOrEqualToUint returns true if x is greater than or equals to y
func (x *BigInt) IsGreaterThanOrEqualToUint(y uint64) bool {
	return x.GetUint64() >= y
}

// IsLessThan returns true if x is less than y
func (x *BigInt) IsLessThan(y *BigInt) bool {
	return x.GetInt64() < y.GetInt64()
}

// IsLessThanOrEquals returns true if x is less than or equals y
func (x *BigInt) IsLessThanOrEquals(y *BigInt) bool {
	return x.GetInt64() <= y.GetInt64()
}

// IsLessThanInt returns true if x is less than y
func (x *BigInt) IsLessThanInt(y int64) bool {
	return x.GetInt64() < y
}

// IsLessThanUint returns true if x is less than y
func (x *BigInt) IsLessThanUint(y uint64) bool {
	return x.GetUint64() < y
}

// IsLessThanOrEqualsInt returns true if x is less than y
func (x *BigInt) IsLessThanOrEqualsInt(y int64) bool {
	return x.GetInt64() <= y
}

// IsLessThanOrEqualsUint returns true if x is less than y
func (x *BigInt) IsLessThanOrEqualsUint(y uint64) bool {
	return x.GetUint64() <= y
}

// Equals returns true if x equals to y
func (x *BigInt) Equals(y *BigInt) bool {
	return x.GetInt64() == y.GetInt64()
}

// EqualsInt returns true if x equals to y
func (x *BigInt) EqualsInt(y int64) bool {
	return x.GetInt64() == y
}

// EqualsUint returns true if x equals to y
func (x *BigInt) EqualsUint(y uint64) bool {
	return x.GetUint64() == y
}

// Add x + y
func (x *BigInt) Add(y *BigInt) *BigInt {
	return NewBigInt(x.GetInt64() + y.GetInt64())
}

// AddInt x + y
func (x *BigInt) AddInt(y int64) *BigInt {
	return NewBigInt(x.GetInt64() + y)
}

// AddUint x + y
func (x *BigInt) AddUint(y uint64) *BigInt {
	return x.AddInt(int64(y))
}

// Sub x - y
func (x *BigInt) Sub(y *BigInt) *BigInt {
	return NewBigInt(x.GetInt64() - y.GetInt64())
}

// SubInt x - y
func (x *BigInt) SubInt(y int64) *BigInt {
	return NewBigInt(x.GetInt64() - y)
}

// SubUint x - y
func (x *BigInt) SubUint(y uint64) *BigInt {
	return x.SubInt(int64(y))
}

// Mul x * y
func (x *BigInt) Mul(y *BigInt) *BigInt {
	return NewBigInt(x.GetInt64() * y.GetInt64())
}

// Div x / y
func (x *BigInt) Div(y *BigInt) *BigInt {
	return NewBigInt(x.GetInt64() / y.GetInt64())
}

// ValidInt64 validate BigInt not overflow Int64
func (x *BigInt) ValidInt64() bool {
	return x.bigint.IsInt64()
}

// ValidUint64 validate BigInt not overflow Uint64
func (x *BigInt) ValidUint64() bool {
	return x.bigint.IsUint64()
}

// Copy returns copy of x
func (x *BigInt) Copy() *BigInt {
	cpy := *x
	return &cpy
}

// String returns x as string
func (x *BigInt) String() string {
	return fmt.Sprintf("%v", x.GetInt64())
}
