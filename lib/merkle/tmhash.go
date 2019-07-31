package merkle

import (
	"crypto/sha256"
)

const (
	TmHashSize = sha256.Size
)

// Sum returns the SHA256 of the bz.
func Sum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}
