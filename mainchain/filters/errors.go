package filters

import (
	"errors"
)

// API Err
var (
	ErrHeaderNotFound    = errors.New("header for hash not found")
	ErrBlockInfoNotFound = errors.New("block info is missing")
)
