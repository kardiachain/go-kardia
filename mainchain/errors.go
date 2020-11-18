// Package kai
package kai

import (
	"errors"
)

// API Err
var (
	ErrHeaderNotFound   = errors.New("header for hash not found")
	ErrInvalidArguments = errors.New("invalid arguments; neither block nor hash specified")
	ErrHashNotCanonical = errors.New("hash is not currently canonical")
	ErrMissingBlockBody = errors.New("block body is missing")
)
