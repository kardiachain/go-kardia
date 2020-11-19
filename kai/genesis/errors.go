// Package genesis
package genesis

import (
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/common"
)

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type ErrMismatch struct {
	Stored, New common.Hash
}

func (e *ErrMismatch) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

var errGenesisNoConfig = errors.New("genesis has no chain configuration")
