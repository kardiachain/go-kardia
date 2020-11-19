// Package blockchain
package blockchain

import (
	"errors"
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)
