// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package kvm

import (
	"errors"
	"math/big"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
	"golang.org/x/crypto/sha3"
)

var (
	bigZero                  = new(big.Int)
	tt255                    = common.BigPow(2, 255)
	errWriteProtection       = errors.New("kvm: write protection")
	errReturnDataOutOfBounds = errors.New("kvm: return data out of bounds")
	errExecutionReverted     = errors.New("kvm: execution reverted")
	errMaxCodeSizeExceeded   = errors.New("kvm: max code size exceeded")
	errInvalidJump           = errors.New("kvm: invalid jump destination")
)

func opAdd(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	common.U256(y.Add(x, y))

	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opSub(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	common.U256(y.Sub(x, y))

	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opMul(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.pop()
	callContext.stack.push(common.U256(x.Mul(x, y)))

	kvm.interpreter.intPool.putOne(y)

	return nil, nil
}

func opDiv(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	if y.Sign() != 0 {
		common.U256(y.Div(x, y))
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opSdiv(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := common.S256(callContext.stack.pop()), common.S256(callContext.stack.pop())
	res := kvm.interpreter.intPool.getZero()

	if y.Sign() == 0 || x.Sign() == 0 {
		callContext.stack.push(res)
	} else {
		if x.Sign() != y.Sign() {
			res.Div(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Div(x.Abs(x), y.Abs(y))
		}
		callContext.stack.push(common.U256(res))
	}
	kvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opMod(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.pop()
	if y.Sign() == 0 {
		callContext.stack.push(x.SetUint64(0))
	} else {
		callContext.stack.push(common.U256(x.Mod(x, y)))
	}
	kvm.interpreter.intPool.putOne(y)
	return nil, nil
}

func opSmod(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := common.S256(callContext.stack.pop()), common.S256(callContext.stack.pop())
	res := kvm.interpreter.intPool.getZero()

	if y.Sign() == 0 {
		callContext.stack.push(res)
	} else {
		if x.Sign() < 0 {
			res.Mod(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Mod(x.Abs(x), y.Abs(y))
		}
		callContext.stack.push(common.U256(res))
	}
	kvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opExp(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	base, exponent := callContext.stack.pop(), callContext.stack.pop()
	// some shortcuts
	cmpToOne := exponent.Cmp(big1)
	if cmpToOne < 0 { // Exponent is zero
		// x ^ 0 == 1
		callContext.stack.push(base.SetUint64(1))
	} else if base.Sign() == 0 {
		// 0 ^ y, if y != 0, == 0
		callContext.stack.push(base.SetUint64(0))
	} else if cmpToOne == 0 { // Exponent is one
		// x ^ 1 == x
		callContext.stack.push(base)
	} else {
		callContext.stack.push(common.Exp(base, exponent))
		kvm.interpreter.intPool.put(base)
	}
	kvm.interpreter.intPool.putOne(exponent)
	return nil, nil
}

func opSignExtend(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	back := callContext.stack.pop()
	if back.Cmp(big.NewInt(31)) < 0 {
		bit := uint(back.Uint64()*8 + 7)
		num := callContext.stack.pop()
		mask := back.Lsh(common.Big1, bit)
		mask.Sub(mask, common.Big1)
		if num.Bit(int(bit)) > 0 {
			num.Or(num, mask.Not(mask))
		} else {
			num.And(num, mask)
		}

		callContext.stack.push(common.U256(num))
	}

	kvm.interpreter.intPool.putOne(back)
	return nil, nil
}

func opNot(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x := callContext.stack.peek()
	common.U256(x.Not(x))
	return nil, nil
}

func opLt(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	if x.Cmp(y) < 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opGt(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	if x.Cmp(y) > 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opSlt(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()

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
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opSgt(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()

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
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opEq(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	if x.Cmp(y) == 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opIszero(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x := callContext.stack.peek()
	if x.Sign() > 0 {
		x.SetUint64(0)
	} else {
		x.SetUint64(1)
	}
	return nil, nil
}

func opAnd(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.pop()
	callContext.stack.push(x.And(x, y))

	kvm.interpreter.intPool.putOne(y)
	return nil, nil
}

func opOr(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	y.Or(x, y)

	kvm.interpreter.intPool.putOne(x)
	return nil, nil
}

func opXor(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y := callContext.stack.pop(), callContext.stack.peek()
	y.Xor(x, y)

	kvm.interpreter.intPool.put(x)
	return nil, nil
}

func opByte(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	th, val := callContext.stack.pop(), callContext.stack.peek()
	if th.Cmp(common.Big32) < 0 {
		b := common.Byte(val, 32, int(th.Int64()))
		val.SetUint64(uint64(b))
	} else {
		val.SetUint64(0)
	}
	kvm.interpreter.intPool.put(th)
	return nil, nil
}

func opAddmod(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y, z := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	if z.Cmp(bigZero) > 0 {
		x.Add(x, y)
		x.Mod(x, z)
	 	callContext.stack.push(common.U256(x))
	} else {
	 	callContext.stack.push(x.SetUint64(0))
	}
	kvm.interpreter.intPool.put(y, z)
	return nil, nil
}

func opMulmod(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	x, y, z := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	if z.Cmp(bigZero) > 0 {
		x.Mul(x, y)
		x.Mod(x, z)
	 	callContext.stack.push(common.U256(x))
	} else {
	 	callContext.stack.push(x.SetUint64(0))
	}
	kvm.interpreter.intPool.put(y, z)
	return nil, nil
}

// opSHL implements Shift Left
// The SHL instruction (shift left) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the left by arg1 number of bits.
func opSHL(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := common.U256(callContext.stack.pop()), common.U256(callContext.stack.peek())
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
func opSHR(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := common.U256(callContext.stack.pop()), common.U256(callContext.stack.peek())
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
func opSAR(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Note, S256 returns (potentially) a new bigint, so we're popping, not peeking this one
	shift, value := common.U256(callContext.stack.pop()), common.S256(callContext.stack.pop())
	defer kvm.interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		if value.Sign() >= 0 {
			value.SetUint64(0)
		} else {
			value.SetInt64(-1)
		}
	 	callContext.stack.push(common.U256(value))
		return nil, nil
	}
	n := uint(shift.Uint64())
	value.Rsh(value, n)
 	callContext.stack.push(common.U256(value))

	return nil, nil
}

func opSha3(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	offset, size := callContext.stack.pop(), callContext.stack.pop()
	data := callContext.memory.GetPtr(offset.Int64(), size.Int64())

	if kvm.interpreter.hasher == nil {
		kvm.interpreter.hasher = sha3.NewLegacyKeccak256().(keccakState)
	} else {
		kvm.interpreter.hasher.Reset()
	}
	kvm.interpreter.hasher.Write(data)
	kvm.interpreter.hasher.Read(kvm.interpreter.hasherBuf[:])

	vm := kvm
	if vm.vmConfig.EnablePreimageRecording {
		vm.StateDB.AddPreimage(kvm.interpreter.hasherBuf, data)
	}
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(kvm.interpreter.hasherBuf[:]))

	kvm.interpreter.intPool.put(offset, size)
	return nil, nil
}

func opAddress(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(callContext.contract.Address().Bytes()))
	return nil, nil
}

func opBalance(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	slot := callContext.stack.peek()
	slot.Set(kvm.StateDB.GetBalance(common.BigToAddress(slot)))
	return nil, nil
}

func opOrigin(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(kvm.Origin.Bytes()))
	return nil, nil
}

func opCaller(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(callContext.contract.Caller().Bytes()))
	return nil, nil
}

func opCallValue(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().Set(callContext.contract.value))
	return nil, nil
}

func opCallDataLoad(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(getDataBig(callContext.contract.Input, callContext.stack.pop(), big32)))
	return nil, nil
}

func opCallDataSize(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetInt64(int64(len(callContext.contract.Input))))
	return nil, nil
}

func opCallDataCopy(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		memOffset  = callContext.stack.pop()
		dataOffset = callContext.stack.pop()
		length     = callContext.stack.pop()
	)
	callContext.memory.Set(memOffset.Uint64(), length.Uint64(), getDataBig(callContext.contract.Input, dataOffset, length))

	kvm.interpreter.intPool.put(memOffset, dataOffset, length)
	return nil, nil
}

func opReturnDataSize(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(uint64(len(kvm.interpreter.returnData))))
	return nil, nil
}

func opReturnDataCopy(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		memOffset  = callContext.stack.pop()
		dataOffset = callContext.stack.pop()
		length     = callContext.stack.pop()

		end = kvm.interpreter.intPool.get().Add(dataOffset, length)
	)
	defer kvm.interpreter.intPool.put(memOffset, dataOffset, length, end)

	if !end.IsUint64() || uint64(len(kvm.interpreter.returnData)) < end.Uint64() {
		return nil, errReturnDataOutOfBounds
	}
	callContext.memory.Set(memOffset.Uint64(), length.Uint64(), kvm.interpreter.returnData[dataOffset.Uint64():end.Uint64()])

	return nil, nil
}

func opExtCodeSize(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	slot := callContext.stack.peek()
	slot.SetUint64(uint64(kvm.StateDB.GetCodeSize(common.BigToAddress(slot))))

	return nil, nil
}

func opCodeSize(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	l := kvm.interpreter.intPool.get().SetInt64(int64(len(callContext.contract.Code)))
 	callContext.stack.push(l)

	return nil, nil
}

func opCodeCopy(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		memOffset  = callContext.stack.pop()
		codeOffset = callContext.stack.pop()
		length     = callContext.stack.pop()
	)
	codeCopy := getDataBig(callContext.contract.Code, codeOffset, length)
	callContext.memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	kvm.interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

func opExtCodeCopy(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		addr       = common.BigToAddress(callContext.stack.pop())
		memOffset  = callContext.stack.pop()
		codeOffset = callContext.stack.pop()
		length     = callContext.stack.pop()
	)
	codeCopy := getDataBig(kvm.StateDB.GetCode(addr), codeOffset, length)
	callContext.memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	kvm.interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

func opGasprice(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().Set(kvm.GasPrice))
	return nil, nil
}

func opBlockhash(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	num := callContext.stack.pop()

	n := kvm.interpreter.intPool.get().Sub(kvm.BlockHeight, common.Big257)
	if num.Cmp(n) > 0 && num.Cmp(kvm.BlockHeight) < 0 {
	 	callContext.stack.push(kvm.GetHash(num.Uint64()).Big())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	}
	kvm.interpreter.intPool.put(num, n)
	return nil, nil
}

func opCoinbase(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(kvm.Coinbase.Bytes()))
	return nil, nil
}

func opTimestamp(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(common.U256(kvm.interpreter.intPool.get().Set(kvm.Time)))
	return nil, nil
}

func opNumber(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(common.U256(kvm.interpreter.intPool.get().Set(kvm.BlockHeight)))
	return nil, nil
}

func opGasLimit(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(common.U256(kvm.interpreter.intPool.get().SetUint64(kvm.GasLimit)))
	return nil, nil
}

func opPop(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	kvm.interpreter.intPool.put(callContext.stack.pop())
	return nil, nil
}

func opMload(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	v := callContext.stack.peek()
	offset := v.Int64()
	v.SetBytes(callContext.memory.GetPtr(offset, 32))
	return nil, nil
}

func opMstore(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// pop value of the stack
	mStart, val := callContext.stack.pop(), callContext.stack.pop()
	 callContext.memory.Set32(mStart.Uint64(), val)

	kvm.interpreter.intPool.put(mStart, val)
	return nil, nil
}

func opMstore8(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	off, val := callContext.stack.pop().Int64(), callContext.stack.pop().Int64()
	 callContext.memory.store[off] = byte(val & 0xff)

	return nil, nil
}

func opSload(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	loc := callContext.stack.peek()
	val := kvm.StateDB.GetState(callContext.contract.Address(), common.BigToHash(loc))
	loc.SetBytes(val.Bytes())
	return nil, nil
}

func opSstore(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	loc := common.BigToHash(callContext.stack.pop())
	val := callContext.stack.pop()
	kvm.StateDB.SetState(callContext.contract.Address(), loc, common.BigToHash(val))

	kvm.interpreter.intPool.put(val)
	return nil, nil
}

func opJump(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	pos := callContext.stack.pop()
	if !callContext.contract.validJumpdest(pos) {
		return nil, errInvalidJump
	}
	*pc = pos.Uint64()

	kvm.interpreter.intPool.put(pos)
	return nil, nil
}

func opJumpi(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	pos, cond := callContext.stack.pop(), callContext.stack.pop()
	if cond.Sign() != 0 {
		if !callContext.contract.validJumpdest(pos) {
			return nil, errInvalidJump
		}
		*pc = pos.Uint64()
	} else {
		*pc++
	}

	kvm.interpreter.intPool.put(pos, cond)
	return nil, nil
}

func opJumpdest(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	return nil, nil
}

func opPc(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(*pc))
	return nil, nil
}

func opMsize(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetInt64(int64( callContext.memory.Len())))
	return nil, nil
}

func opGas(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(callContext.contract.Gas))
	return nil, nil
}

func opCreate(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		value        = callContext.stack.pop()
		offset, size = callContext.stack.pop(), callContext.stack.pop()
		input        =  callContext.memory.GetCopy(offset.Int64(), size.Int64())
		gas          = callContext.contract.Gas
	)

	callContext.contract.UseGas(gas)
	res, addr, returnGas, suberr := kvm.Create(callContext.contract, input, gas, value)
	// Push item on the stack based on the returned error. If the ruleset is
	// homestead we must check for CodeStoreOutOfGasError (homestead only
	// rule) and treat as an error, if the ruleset is frontier we must
	// ignore this error and pretend the operation was successful.
	if suberr != nil && suberr != ErrCodeStoreOutOfGas {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.get().SetBytes(addr.Bytes()))
	}

	callContext.contract.Gas += returnGas
	kvm.interpreter.intPool.put(value, offset, size)

	if suberr == errExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCall(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Pop gas. The actual gas in in kvm.callGasTemp.
	kvm.interpreter.intPool.put(callContext.stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	toAddr := common.BigToAddress(addr)
	value = common.U256(value)
	// Get the arguments from the memory.
	args := callContext.memory.GetPtr(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += configs.CallStipend
	}
	ret, returnGas, err := kvm.Call(callContext.contract, toAddr, args, gas, value)
	if err != nil {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		callContext.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opCallCode(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(callContext.stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	toAddr := common.BigToAddress(addr)
	value = common.U256(value)
	// Get arguments from the callContext.memory.
	args := callContext.memory.GetPtr(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += configs.CallStipend
	}
	ret, returnGas, err := kvm.CallCode(callContext.contract, toAddr, args, gas, value)
	if err != nil {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		callContext.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opDelegateCall(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(callContext.stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := callContext.memory.GetPtr(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := kvm.DelegateCall(callContext.contract, toAddr, args, gas)
	if err != nil {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		callContext.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opStaticCall(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	// Pop gas. The actual gas is in kvm.callGasTemp.
	kvm.interpreter.intPool.put(callContext.stack.pop())
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop(), callContext.stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := callContext.memory.GetPtr(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := kvm.StaticCall(callContext.contract, toAddr, args, gas)
	if err != nil {
	 	callContext.stack.push(kvm.interpreter.intPool.getZero())
	} else {
	 	callContext.stack.push(kvm.interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		callContext.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.contract.Gas += returnGas

	kvm.interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opReturn(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	offset, size := callContext.stack.pop(), callContext.stack.pop()
	ret := callContext.memory.GetPtr(offset.Int64(), size.Int64())

	kvm.interpreter.intPool.put(offset, size)
	return ret, nil
}

func opRevert(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	offset, size := callContext.stack.pop(), callContext.stack.pop()
	ret := callContext.memory.GetPtr(offset.Int64(), size.Int64())

	kvm.interpreter.intPool.put(offset, size)
	return ret, nil
}

func opStop(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	return nil, nil
}

func opSuicide(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	balance := kvm.StateDB.GetBalance(callContext.contract.Address())
	kvm.StateDB.AddBalance(common.BigToAddress(callContext.stack.pop()), balance)

	kvm.StateDB.Suicide(callContext.contract.Address())
	return nil, nil
}

// opPush1 is a specialized version of pushN
func opPush1(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
	var (
		codeLen = uint64(len(callContext.contract.Code))
		integer = kvm.interpreter.intPool.get()
	)
	*pc += 1
	if *pc < codeLen {
	 	callContext.stack.push(integer.SetUint64(uint64(callContext.contract.Code[*pc])))
	} else {
	 	callContext.stack.push(integer.SetUint64(0))
	}
	return nil, nil
}

// make push instruction function
func makePush(size uint64, pushByteSize int) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
		codeLen := len(callContext.contract.Code)

		startMin := codeLen
		if int(*pc+1) < startMin {
			startMin = int(*pc + 1)
		}

		endMin := codeLen
		if startMin+pushByteSize < endMin {
			endMin = startMin + pushByteSize
		}

		integer := kvm.interpreter.intPool.get()
	 	callContext.stack.push(integer.SetBytes(common.RightPadBytes(callContext.contract.Code[startMin:endMin], pushByteSize)))

		*pc += size
		return nil, nil
	}
}

// make dup instruction function
func makeDup(size int64) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
		callContext.stack.dup(kvm.interpreter.intPool, int(size))
		return nil, nil
	}
}

// make swap instruction function
func makeSwap(size int64) executionFunc {
	// switch n + 1 otherwise n would be swapped with n
	size++
	return func(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
		callContext.stack.swap(int(size))
		return nil, nil
	}
}

// make log instruction function
func makeLog(size int) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *callCtx) ([]byte, error) {
		topics := make([]common.Hash, size)
		mStart, mSize := callContext.stack.pop(), callContext.stack.pop()
		for i := 0; i < size; i++ {
			topics[i] = common.BigToHash(callContext.stack.pop())
		}

		d := callContext.memory.GetCopy(mStart.Int64(), mSize.Int64())
		kvm.StateDB.AddLog(&types.Log{
			Address: callContext.contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block height.
			BlockHeight: kvm.BlockHeight.Uint64(),
		})

		kvm.interpreter.intPool.put(mStart, mSize)
		return nil, nil
	}
}
