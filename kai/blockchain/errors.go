// Package blockchain
package blockchain

import (
	"errors"
)

var (
	ErrNoGenesis = errors.New("genesis not found in chain")
)
