package evidence

import (
	"fmt"

	"github.com/kardiachain/go-kardiamain/types"
)

const (
	baseKeyPending = byte(0x01)
)

func keySuffix(evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("%s/%X", bE(evidence.Height().Int64()), evidence.Hash()))
}

// big endian padded hex
func bE(h int64) string {
	return fmt.Sprintf("%0.16X", h)
}

func keyPending(evidence types.Evidence) []byte {
	return append([]byte{baseKeyPending}, keySuffix(evidence)...)
}
