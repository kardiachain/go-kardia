package sha3

import "encoding/binary"

// xorInGeneric xors the bytes in buf into the state; it
// makes no non-portable assumptions about memory layout
// or alignment.
func xorInGeneric(d *state, buf []byte) {
	n := len(buf) / 8

	for i := 0; i < n; i++ {
		a := binary.LittleEndian.Uint64(buf)
		d.a[i] ^= a
		buf = buf[8:]
	}
}

// copyOutGeneric copies ulint64s to a byte buffer.
func copyOutGeneric(d *state, b []byte) {
	for i := 0; len(b) >= 8; i++ {
		binary.LittleEndian.PutUint64(b, d.a[i])
		b = b[8:]
	}
}
