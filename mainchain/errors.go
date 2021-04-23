// Package kai
package kai

import (
	"errors"
)

// API Err
var (
	ErrHeaderNotFound    = errors.New("header for hash not found")
	ErrInvalidArguments  = errors.New("invalid arguments; neither block nor hash specified")
	ErrHashNotCanonical  = errors.New("hash is not currently canonical")
	ErrMissingBlockBody  = errors.New("block body is missing")
	ErrBlockInfoNotFound = errors.New("block info is missing")
	ErrExceedGasLimit    = errors.New("gas limit exceeds gas cap")
	ErrNotEnoughGasPrice = errors.New("not enough gas price")
	ErrNilGasPrice       = errors.New("nil gas price")
	ErrBlockNotFound     = errors.New("block not found")
)
