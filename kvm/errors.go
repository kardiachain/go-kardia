// Package kvm
package kvm

import (
	"errors"
)

// Contract
// List execution errors
var (
	ErrOutOfGas                 = errors.New("out of gas")
	ErrCodeStoreOutOfGas        = errors.New("contract creation code storage out of gas")
	ErrDepth                    = errors.New("max call depth exceeded")
	ErrInsufficientBalance      = errors.New("insufficient balance for transfer")
	ErrContractAddressCollision = errors.New("contract address collision")
)

// Instructions
var (
	ErrWriteProtection       = errors.New("kvm: write protection")
	ErrReturnDataOutOfBounds = errors.New("kvm: return data out of bounds")
	ErrExecutionReverted     = errors.New("kvm: execution reverted")
	ErrMaxCodeSizeExceeded   = errors.New("kvm: max code size exceeded")
	ErrInvalidJump           = errors.New("kvm: invalid jump destination")
)

// Logger
var errTraceLimitReached = errors.New("the number of logs reached the specified limit")
