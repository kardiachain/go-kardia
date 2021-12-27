package kvm

import (
	"github.com/holiman/uint256"
	"github.com/kardiachain/go-kardia/configs"
)

// enable1344 applies EIP-1344 (ChainID Opcode)
// - Adds an opcode that returns the current chain’s EIP-155 unique identifier
func enable1344(jt *JumpTable) {
	// New opcode
	jt[CHAINID] = &operation{
		execute:     opChainID,
		constantGas: configs.GasQuickStep,
		minStack:    minStack(0, 1),
		maxStack:    maxStack(0, 1),
	}
}

// opChainID implements CHAINID opcode
func opChainID(pc *uint64, interpreter *KVM, scope *ScopeContext) ([]byte, error) {
	chainId, _ := uint256.FromBig(interpreter.chainConfig.ChainID)
	scope.Stack.push(chainId)
	return nil, nil
}
