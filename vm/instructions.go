package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/params"
	"github.com/kardiachain/go-kardia/types"
)

var (
	bigZero                  = new(big.Int)
	tt255                    = common.BigPow(2, 255)
	errWriteProtection       = errors.New("kvm: write protection")
	errReturnDataOutOfBounds = errors.New("kvm: return data out of bounds")
	errExecutionReverted     = errors.New("kvm: execution reverted")
	errMaxCodeSizeExceeded   = errors.New("kvm: max code size exceeded")
)

func opAdd(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	common.U256(y.Add(x, y))

	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opSub(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	common.U256(y.Sub(x, y))

	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opMul(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(common.U256(x.Mul(x, y)))

	kvm.interpreter.intPool.put(y)

	return nil, nil
}

func opDiv(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if y.Sign() != 0 {
		common.U256(y.Div(x, y))
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opSdiv(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := common.S256(stack.pop()), common.S256(stack.pop())
	res := kvm.interpreter.intPool.getZero()

	if y.Sign() == 0 || x.Sign() == 0 {
		stack.push(res)
	} else {
		if x.Sign() != y.Sign() {
			res.Div(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Div(x.Abs(x), y.Abs(y))
		}
		stack.push(common.U256(res))
	}
	kvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opMod(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if y.Sign() == 0 {
		stack.push(x.SetUint64(0))
	} else {
		stack.push(common.U256(x.Mod(x, y)))
	}
	kvm.interpreter.intPool.put(y)
	return nil, nil
}

func opSmod(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := common.S256(stack.pop()), common.S256(stack.pop())
	res := kvm.interpreter.intPool.getZero()

	if y.Sign() == 0 {
		stack.push(res)
	} else {
		if x.Sign() < 0 {
			res.Mod(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Mod(x.Abs(x), y.Abs(y))
		}
		stack.push(common.U256(res))
	}
	kvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opExp(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	base, exponent := stack.pop(), stack.pop()
	stack.push(common.Exp(base, exponent))

	kvm.interpreter.intPool.put(base, exponent)

	return nil, nil
}

func opSignExtend(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	back := stack.pop()
	if back.Cmp(big.NewInt(31)) < 0 {
		bit := uint(back.Uint64()*8 + 7)
		num := stack.pop()
		mask := back.Lsh(common.Big1, bit)
		mask.Sub(mask, common.Big1)
		if num.Bit(int(bit)) > 0 {
			num.Or(num, mask.Not(mask))
		} else {
			num.And(num, mask)
		}

		stack.push(common.U256(num))
	}

	kvm.interpreter.intPool.put(back)
	return nil, nil
}

func opNot(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.peek()
	common.U256(x.Not(x))
	return nil, nil
}

func opLt(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) < 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opGt(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) > 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opSlt(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()

	xSign := x.Cmp(tt255)
	ySign := y.Cmp(tt255)

	switch {
	case xSign >= 0 && ySign < 0:
		y.SetUint64(1)

	case xSign < 0 && ySign >= 0:
		y.SetUint64(0)

	default:
		if x.Cmp(y) < 0 {
			y.SetUint64(1)
		} else {
			y.SetUint64(0)
		}
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opSgt(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()

	xSign := x.Cmp(tt255)
	ySign := y.Cmp(tt255)

	switch {
	case xSign >= 0 && ySign < 0:
		y.SetUint64(0)

	case xSign < 0 && ySign >= 0:
		y.SetUint64(1)

	default:
		if x.Cmp(y) > 0 {
			y.SetUint64(1)
		} else {
			y.SetUint64(0)
		}
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opEq(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) == 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opIszero(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.peek()
	if x.Sign() > 0 {
		x.SetUint64(0)
	} else {
		x.SetUint64(1)
	}
	return nil, nil
}

func opAnd(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(x.And(x, y))

	kvm.interpreter.intPool.put(y)
	return nil, nil
}

func opOr(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	y.Or(x, y)

	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opXor(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	y.Xor(x, y)

	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opByte(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	th, val := stack.pop(), stack.peek()
	if th.Cmp(common.Big32) < 0 {
		b := common.Byte(val, 32, int(th.Int64()))
		val.SetUint64(uint64(b))
	} else {
		val.SetUint64(0)
	}
	kvm.interpreter.intPool.put(th)
	return nil, nil
}

func opAddmod(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(bigZero) > 0 {
		x.Add(x, y)
		x.Mod(x, z)
		stack.push(common.U256(x))
	} else {
		stack.push(x.SetUint64(0))
	}
	kvm.interpreter.intPool.put(y, z)
	return nil, nil
}

func opMulmod(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(bigZero) > 0 {
		x.Mul(x, y)
		x.Mod(x, z)
		stack.push(common.U256(x))
	} else {
		stack.push(x.SetUint64(0))
	}
	kvm.interpreter.intPool.put(y, z)
	return nil, nil
}

// opSHL implements Shift Left
// The SHL instruction (shift left) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the left by arg1 number of bits.
func opSHL(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := common.U256(stack.pop()), common.U256(stack.peek())
	defer kvm.interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		value.SetUint64(0)
		return nil, nil
	}
	n := uint(shift.Uint64())
	common.U256(value.Lsh(value, n))

	return nil, nil
}

// opSHR implements Logical Shift Right
// The SHR instruction (logical shift right) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the right by arg1 number of bits with zero fill.
func opSHR(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := common.U256(stack.pop()), common.U256(stack.peek())
	defer kvm.interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		value.SetUint64(0)
		return nil, nil
	}
	n := uint(shift.Uint64())
	common.U256(value.Rsh(value, n))

	return nil, nil
}

// opSAR implements Arithmetic Shift Right
// The SAR instruction (arithmetic shift right) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the right by arg1 number of bits with sign extension.
func opSAR(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, S256 returns (potentially) a new bigint, so we're popping, not peeking this one
	shift, value := common.U256(stack.pop()), common.S256(stack.pop())
	defer kvm.interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		if value.Sign() > 0 {
			value.SetUint64(0)
		} else {
			value.SetInt64(-1)
		}
		stack.push(common.U256(value))
		return nil, nil
	}
	n := uint(shift.Uint64())
	value.Rsh(value, n)
	stack.push(common.U256(value))

	return nil, nil
}

func opSha3(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	data := memory.Get(offset.Int64(), size.Int64())
	hash := crypto.Keccak256(data)

	stack.push(kvm.interpreter.intPool.get().SetBytes(hash))

	kvm.interpreter.intPool.put(offset, size)
	return nil, nil
}

func opStop(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opAddress(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Address().Big())
	return nil, nil
}

func opBalance(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	slot := stack.peek()
	slot.Set(kvm.StateDB.GetBalance(common.BigToAddress(slot)))
	return nil, nil
}

func opOrigin(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.Origin.Big())
	return nil, nil
}

func opCaller(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Caller().Big())
	return nil, nil
}

func opCallValue(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().Set(contract.value))
	return nil, nil
}

func opCallDataLoad(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetBytes(getDataBig(contract.Input, stack.pop(), big32)))
	return nil, nil
}

func opCallDataSize(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetInt64(int64(len(contract.Input))))
	return nil, nil
}

func opCallDataCopy(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		dataOffset = stack.pop()
		length     = stack.pop()
	)
	memory.Set(memOffset.Uint64(), length.Uint64(), getDataBig(contract.Input, dataOffset, length))

	kvm.interpreter.intPool.put(memOffset, dataOffset, length)
	return nil, nil
}

func opReturnDataSize(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetUint64(uint64(len(kvm.interpreter.returnData))))
	return nil, nil
}

func opReturnDataCopy(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		dataOffset = stack.pop()
		length     = stack.pop()

		end = kvm.interpreter.intPool.get().Add(dataOffset, length)
	)
	defer kvm.interpreter.intPool.put(memOffset, dataOffset, length, end)

	if end.BitLen() > 64 || uint64(len(kvm.interpreter.returnData)) < end.Uint64() {
		return nil, errReturnDataOutOfBounds
	}
	memory.Set(memOffset.Uint64(), length.Uint64(), kvm.interpreter.returnData[dataOffset.Uint64():end.Uint64()])

	return nil, nil
}

func opExtCodeSize(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	slot := stack.peek()
	slot.SetUint64(uint64(kvm.StateDB.GetCodeSize(common.BigToAddress(slot))))

	return nil, nil
}

func opCodeSize(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	l := kvm.interpreter.intPool.get().SetInt64(int64(len(contract.Code)))
	stack.push(l)

	return nil, nil
}

func opCodeCopy(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	codeCopy := getDataBig(contract.Code, codeOffset, length)
	memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	kvm.interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

func opExtCodeCopy(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		addr       = common.BigToAddress(stack.pop())
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	codeCopy := getDataBig(kvm.StateDB.GetCode(addr), codeOffset, length)
	memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	kvm.interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

func opGasprice(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().Set(kvm.GasPrice))
	return nil, nil
}

func opBlockhash(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	num := stack.pop()

	n := kvm.interpreter.intPool.get().Sub(new(big.Int).SetUint64(kvm.BlockHeight), common.Big257)
	if num.Cmp(n) > 0 && num.Cmp(new(big.Int).SetUint64(kvm.BlockHeight)) < 0 {
		stack.push(kvm.GetHash(num.Uint64()).Big())
	} else {
		stack.push(kvm.interpreter.intPool.getZero())
	}
	kvm.interpreter.intPool.put(num, n)
	return nil, nil
}

func opCoinbase(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.Coinbase.Big())
	return nil, nil
}

func opTimestamp(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(common.U256(kvm.interpreter.intPool.get().Set(kvm.Time)))
	return nil, nil
}

func opNumber(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(common.U256(kvm.interpreter.intPool.get().Set(new(big.Int).SetUint64(kvm.BlockHeight))))
	return nil, nil
}

func opGasLimit(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(common.U256(kvm.interpreter.intPool.get().SetUint64(kvm.GasLimit)))
	return nil, nil
}

func opPop(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	kvm.interpreter.intPool.put(stack.pop())
	return nil, nil
}

func opMload(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset := stack.pop()
	val := kvm.interpreter.intPool.get().SetBytes(memory.Get(offset.Int64(), 32))
	stack.push(val)

	kvm.interpreter.intPool.put(offset)
	return nil, nil
}

func opMstore(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// pop value of the stack
	mStart, val := stack.pop(), stack.pop()
	memory.Set32(mStart.Uint64(), val)

	kvm.interpreter.intPool.put(mStart, val)
	return nil, nil
}

func opMstore8(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	off, val := stack.pop().Int64(), stack.pop().Int64()
	memory.store[off] = byte(val & 0xff)

	return nil, nil
}

func opSload(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := stack.peek()
	val := kvm.StateDB.GetState(contract.Address(), common.BigToHash(loc))
	loc.SetBytes(val.Bytes())
	return nil, nil
}

func opSstore(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := common.BigToHash(stack.pop())
	val := stack.pop()
	kvm.StateDB.SetState(contract.Address(), loc, common.BigToHash(val))

	kvm.interpreter.intPool.put(val)
	return nil, nil
}

func opJump(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos := stack.pop()
	if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
		nop := contract.GetOp(pos.Uint64())
		return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
	}
	*pc = pos.Uint64()

	kvm.interpreter.intPool.put(pos)
	return nil, nil
}

func opJumpi(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos, cond := stack.pop(), stack.pop()
	if cond.Sign() != 0 {
		if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
			nop := contract.GetOp(pos.Uint64())
			return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
		}
		*pc = pos.Uint64()
	} else {
		*pc++
	}

	kvm.interpreter.intPool.put(pos, cond)
	return nil, nil
}

func opJumpdest(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opPc(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetUint64(*pc))
	return nil, nil
}

func opMsize(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetInt64(int64(memory.Len())))
	return nil, nil
}

func opGas(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(kvm.interpreter.intPool.get().SetUint64(contract.Gas))
	return nil, nil
}

func opCreate(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		value        = stack.pop()
		offset, size = stack.pop(), stack.pop()
		input        = memory.Get(offset.Int64(), size.Int64())
		gas          = contract.Gas
	)

	contract.UseGas(gas)
	res, addr, returnGas, suberr := kvm.Create(contract, input, gas, value)
	if suberr != nil {
		stack.push(kvm.interpreter.intPool.getZero())
	} else {
		stack.push(addr.Big())
	}
	contract.Gas += returnGas
	kvm.interpreter.intPool.put(value, offset, size)

	if suberr == errExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCall(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas in in kvm.callGasTemp.
	kvm.interpreter.intPool.put(stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	value = common.U256(value)
	// Get the arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}
	ret, returnGas, err := kvm.Call(contract, toAddr, args, gas, value)
	if err != nil {
		stack.push(kvm.interpreter.intPool.getZero())
	} else {
		stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opCallCode(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	value = common.U256(value)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}
	ret, returnGas, err := kvm.CallCode(contract, toAddr, args, gas, value)
	if err != nil {
		stack.push(kvm.interpreter.intPool.getZero())
	} else {
		stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opDelegateCall(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := kvm.DelegateCall(contract, toAddr, args, gas)
	if err != nil {
		stack.push(kvm.interpreter.intPool.getZero())
	} else {
		stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opStaticCall(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := kvm.StaticCall(contract, toAddr, args, gas)
	if err != nil {
		stack.push(kvm.interpreter.intPool.getZero())
	} else {
		stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opReturn(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	ret := memory.GetPtr(offset.Int64(), size.Int64())

	kvm.interpreter.intPool.put(offset, size)
	return ret, nil
}

func opRevert(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	ret := memory.GetPtr(offset.Int64(), size.Int64())

	kvm.interpreter.intPool.put(offset, size)
	return ret, nil
}

func opSuicide(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	balance := kvm.StateDB.GetBalance(contract.Address())
	kvm.StateDB.AddBalance(common.BigToAddress(stack.pop()), balance)

	kvm.StateDB.Suicide(contract.Address())
	return nil, nil
}

// make push instruction function
func makePush(size uint64, pushByteSize int) executionFunc {
	return func(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		codeLen := len(contract.Code)

		startMin := codeLen
		if int(*pc+1) < startMin {
			startMin = int(*pc + 1)
		}

		endMin := codeLen
		if startMin+pushByteSize < endMin {
			endMin = startMin + pushByteSize
		}

		integer := kvm.interpreter.intPool.get()
		stack.push(integer.SetBytes(common.RightPadBytes(contract.Code[startMin:endMin], pushByteSize)))

		*pc += size
		return nil, nil
	}
}

// make dup instruction function
func makeDup(size int64) executionFunc {
	return func(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.dup(kvm.interpreter.intPool, int(size))
		return nil, nil
	}
}

// make swap instruction function
func makeSwap(size int64) executionFunc {
	// switch n + 1 otherwise n would be swapped with n
	size++
	return func(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.swap(int(size))
		return nil, nil
	}
}

// make log instruction function
func makeLog(size int) executionFunc {
	return func(pc *uint64, kvm *KVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		topics := make([]common.Hash, size)
		mStart, mSize := stack.pop(), stack.pop()
		for i := 0; i < size; i++ {
			topics[i] = common.BigToHash(stack.pop())
		}

		d := memory.Get(mStart.Int64(), mSize.Int64())
		kvm.StateDB.AddLog(&types.Log{
			Address: contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block height.
			BlockHeight: kvm.BlockHeight,
		})

		kvm.interpreter.intPool.put(mStart, mSize)
		return nil, nil
	}
}
