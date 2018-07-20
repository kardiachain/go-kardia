package vm

import (
	"errors"
	"math/big"

	"github.com/kardiachain/go-kardia/params"
)

type (
	executionFunc       func(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error)
	gasFunc             func(params.GasTable, *KVM, *Contract, *Stack, *Memory, uint64) (uint64, error) // last parameter is the requested memory size as a uint64
	stackValidationFunc func(*Stack) error
	memorySizeFunc      func(*Stack) *big.Int
)

var errGasUintOverflow = errors.New("gas uint64 overflow")

type operation struct {
	// execute is the operation function
	execute executionFunc
	// gasCost is the gas function and returns the gas required for execution
	gasCost gasFunc
	// validateStack validates the stack (size) for the operation
	validateStack stackValidationFunc
	// memorySize returns the memory size required for the operation
	memorySize memorySizeFunc

	halts   bool // indicates whether the operation should halt further execution
	jumps   bool // indicates whether the program counter should not increment
	writes  bool // determines whether this a state modifying operation
	valid   bool // indication whether the retrieved operation is valid and known
	reverts bool // determines whether the operation reverts state (implicitly halts)
	returns bool // determines whether the operations sets the return data content
}

// NewInstructionSet returns the instructions that are supported by Kardia interpreter.
func newInstructionSet() [256]operation {
	return [256]operation{
		STOP: {
			execute:       opStop,
			gasCost:       constGasFunc(0),
			validateStack: makeStackFunc(0, 0),
			halts:         true,
			valid:         true,
		},
		ADD: {
			execute:       opAdd,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		MUL: {
			execute:       opMul,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SUB: {
			execute:       opSub,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		DIV: {
			execute:       opDiv,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SDIV: {
			execute:       opSdiv,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		MOD: {
			execute:       opMod,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SMOD: {
			execute:       opSmod,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		ADDMOD: {
			execute:       opAddmod,
			gasCost:       constGasFunc(GasMidStep),
			validateStack: makeStackFunc(3, 1),
			valid:         true,
		},
		MULMOD: {
			execute:       opMulmod,
			gasCost:       constGasFunc(GasMidStep),
			validateStack: makeStackFunc(3, 1),
			valid:         true,
		},
		EXP: {
			execute:       opExp,
			gasCost:       gasExp,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SIGNEXTEND: {
			execute:       opSignExtend,
			gasCost:       constGasFunc(GasFastStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		LT: {
			execute:       opLt,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		GT: {
			execute:       opGt,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SLT: {
			execute:       opSlt,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SGT: {
			execute:       opSgt,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		EQ: {
			execute:       opEq,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		ISZERO: {
			execute:       opIszero,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		AND: {
			execute:       opAnd,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		XOR: {
			execute:       opXor,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		OR: {
			execute:       opOr,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		NOT: {
			execute:       opNot,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		BYTE: {
			execute:       opByte,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SHA3: {
			execute:       opSha3,
			gasCost:       gasSha3,
			validateStack: makeStackFunc(2, 1),
			memorySize:    memorySha3,
			valid:         true,
		},
		ADDRESS: {
			execute:       opAddress,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		BALANCE: {
			execute:       opBalance,
			gasCost:       gasBalance,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		ORIGIN: {
			execute:       opOrigin,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLER: {
			execute:       opCaller,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLVALUE: {
			execute:       opCallValue,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLDATALOAD: {
			execute:       opCallDataLoad,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		CALLDATASIZE: {
			execute:       opCallDataSize,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLDATACOPY: {
			execute:       opCallDataCopy,
			gasCost:       gasCallDataCopy,
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryCallDataCopy,
			valid:         true,
		},
		CODESIZE: {
			execute:       opCodeSize,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CODECOPY: {
			execute:       opCodeCopy,
			gasCost:       gasCodeCopy,
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryCodeCopy,
			valid:         true,
		},
		GASPRICE: {
			execute:       opGasprice,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		EXTCODESIZE: {
			execute:       opExtCodeSize,
			gasCost:       gasExtCodeSize,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		EXTCODECOPY: {
			execute:       opExtCodeCopy,
			gasCost:       gasExtCodeCopy,
			validateStack: makeStackFunc(4, 0),
			memorySize:    memoryExtCodeCopy,
			valid:         true,
		},
		BLOCKHASH: {
			execute:       opBlockhash,
			gasCost:       constGasFunc(GasExtStep),
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		COINBASE: {
			execute:       opCoinbase,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		TIMESTAMP: {
			execute:       opTimestamp,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		NUMBER: {
			execute:       opNumber,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		GASLIMIT: {
			execute:       opGasLimit,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		/* TODO(huny@): Enable these as newer OpCodes
		DELEGATECALL: {
			execute:       opDelegateCall,
			gasCost:       gasDelegateCall,
			validateStack: makeStackFunc(6, 1),
			memorySize:    memoryDelegateCall,
			valid:         true,
			returns:       true,
		},
		STATICCALL: {
			execute:       opStaticCall,
			gasCost:       gasStaticCall,
			validateStack: makeStackFunc(6, 1),
			memorySize:    memoryStaticCall,
			valid:         true,
			returns:       true,
		},
		*/
		RETURNDATASIZE: {
			execute:       opReturnDataSize,
			gasCost:       constGasFunc(GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		RETURNDATACOPY: {
			execute:       opReturnDataCopy,
			gasCost:       gasReturnDataCopy,
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryReturnDataCopy,
			valid:         true,
		},
		/*
			REVERT: {
				execute:       opRevert,
				gasCost:       gasRevert,
				validateStack: makeStackFunc(2, 0),
				memorySize:    memoryRevert,
				valid:         true,
				reverts:       true,
				returns:       true,
			},
		*/
		SHL: {
			execute:       opSHL,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SHR: {
			execute:       opSHR,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SAR: {
			execute:       opSAR,
			gasCost:       constGasFunc(GasFastestStep),
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
	}
}
