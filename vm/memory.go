package vm

import (
	"math/big"
)

// Memory implements a simple memory model for the ethereum virtual machine.
type Memory struct {
	store       []byte
	lastGasCost uint64
}

// NewMemory returns a new memory memory model.
func NewMemory() *Memory {
	return &Memory{}
}

// Get returns offset + size as a new slice
func (m *Memory) Get(offset, size int64) (cpy []byte) {
	if size == 0 {
		return nil
	}

	if len(m.store) > int(offset) {
		cpy = make([]byte, size)
		copy(cpy, m.store[offset:offset+size])

		return
	}

	return
}

// Len returns the length of the backing slice
func (m *Memory) Len() int {
	return len(m.store)
}

//==============================================================================
// Memory table
//==============================================================================
func memorySha3(stack *Stack) *big.Int {
	return calcMemSize(stack.Back(0), stack.Back(1))
}
