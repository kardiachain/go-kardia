package types

import (
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/merkle"
)

// ValidateHash returns an error if the hash is not empty, but its
// size != merkle.TmHashSize.
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != merkle.TmHashSize {
		return fmt.Errorf("Expected size to be %d bytes, got %d bytes",
			merkle.TmHashSize,
			len(h),
		)
	}
	return nil
}
