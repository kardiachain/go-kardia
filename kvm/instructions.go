/*
 *  Copyright 2020 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package kvm

import (
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

func opAdd(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Add(&x, y)
	return nil, nil
}

func opSub(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Sub(&x, y)
	return nil, nil
}

func opMul(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Mul(&x, y)
	return nil, nil
}

func opDiv(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Div(&x, y)
	return nil, nil
}

func opSdiv(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.SDiv(&x, y)
	return nil, nil
}

func opMod(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Mod(&x, y)
	return nil, nil
}

func opSmod(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.SMod(&x, y)
	return nil, nil
}

func opExp(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	base, exponent := callContext.Stack.pop(), callContext.Stack.peek()
	exponent.Exp(&base, exponent)
	return nil, nil
}

func opSignExtend(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	back, num := callContext.Stack.pop(), callContext.Stack.peek()
	num.ExtendSign(num, &back)
	return nil, nil
}

func opNot(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x := callContext.Stack.peek()
	x.Not(x)
	return nil, nil
}

func opLt(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	if x.Lt(y) {
		y.SetOne()
	} else {
		y.Clear()
	}
	return nil, nil
}

func opGt(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	if x.Gt(y) {
		y.SetOne()
	} else {
		y.Clear()
	}
	return nil, nil
}

func opSlt(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	if x.Slt(y) {
		y.SetOne()
	} else {
		y.Clear()
	}
	return nil, nil
}

func opSgt(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	if x.Sgt(y) {
		y.SetOne()
	} else {
		y.Clear()
	}
	return nil, nil
}

func opEq(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	if x.Eq(y) {
		y.SetOne()
	} else {
		y.Clear()
	}
	return nil, nil
}

func opIszero(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x := callContext.Stack.peek()
	if x.IsZero() {
		x.SetOne()
	} else {
		x.Clear()
	}
	return nil, nil
}

func opAnd(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.And(&x, y)
	return nil, nil
}

func opOr(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Or(&x, y)
	return nil, nil
}

func opXor(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y := callContext.Stack.pop(), callContext.Stack.peek()
	y.Xor(&x, y)
	return nil, nil
}

func opByte(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	th, val := callContext.Stack.pop(), callContext.Stack.peek()
	val.Byte(&th)
	return nil, nil
}

func opAddmod(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y, z := callContext.Stack.pop(), callContext.Stack.pop(), callContext.Stack.peek()
	if z.IsZero() {
		z.Clear()
	} else {
		z.AddMod(&x, &y, z)
	}
	return nil, nil
}

func opMulmod(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x, y, z := callContext.Stack.pop(), callContext.Stack.pop(), callContext.Stack.peek()
	z.MulMod(&x, &y, z)
	return nil, nil
}

// opSHL implements Shift Left
// The SHL instruction (shift left) pops 2 values from the Stack, first arg1 and then arg2,
// and pushes on the Stack arg2 shifted to the left by arg1 number of bits.
func opSHL(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	// Note, second operand is left in the Stack; accumulate result into it, and no need to push it afterwards
	shift, value := callContext.Stack.pop(), callContext.Stack.peek()
	if shift.LtUint64(256) {
		value.Lsh(value, uint(shift.Uint64()))
	} else {
		value.Clear()
	}
	return nil, nil
}

// opSHR implements Logical Shift Right
// The SHR instruction (logical shift right) pops 2 values from the Stack, first arg1 and then arg2,
// and pushes on the Stack arg2 shifted to the right by arg1 number of bits with zero fill.
func opSHR(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	// Note, second operand is left in the Stack; accumulate result into it, and no need to push it afterwards
	shift, value := callContext.Stack.pop(), callContext.Stack.peek()
	if shift.LtUint64(256) {
		value.Rsh(value, uint(shift.Uint64()))
	} else {
		value.Clear()
	}
	return nil, nil
}

// opSAR implements Arithmetic Shift Right
// The SAR instruction (arithmetic shift right) pops 2 values from the Stack, first arg1 and then arg2,
// and pushes on the Stack arg2 shifted to the right by arg1 number of bits with sign extension.
func opSAR(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	shift, value := callContext.Stack.pop(), callContext.Stack.peek()
	if shift.GtUint64(256) {
		if value.Sign() >= 0 {
			value.Clear()
		} else {
			// Max negative shift: all bits set
			value.SetAllOne()
		}
		return nil, nil
	}
	n := uint(shift.Uint64())
	value.SRsh(value, n)
	return nil, nil
}

func opSha3(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	offset, size := callContext.Stack.pop(), callContext.Stack.peek()
	data := callContext.Memory.GetPtr(int64(offset.Uint64()), int64(size.Uint64()))

	if kvm.interpreter.hasher == nil {
		kvm.interpreter.hasher = sha3.NewLegacyKeccak256().(keccakState)
	} else {
		kvm.interpreter.hasher.Reset()
	}
	kvm.interpreter.hasher.Write(data)
	kvm.interpreter.hasher.Read(kvm.interpreter.hasherBuf[:])

	if kvm.vmConfig.EnablePreimageRecording {
		kvm.StateDB.AddPreimage(kvm.interpreter.hasherBuf, data)
	}
	size.SetBytes(kvm.interpreter.hasherBuf[:])
	return nil, nil
}

func opAddress(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetBytes(callContext.Contract.Address().Bytes()))
	return nil, nil
}

func opBalance(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	slot := callContext.Stack.peek()
	address := common.Address(slot.Bytes20())
	slot.SetFromBig(kvm.StateDB.GetBalance(address))
	return nil, nil
}

func opOrigin(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetBytes(kvm.Origin.Bytes()))
	return nil, nil
}

func opCaller(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetBytes(callContext.Contract.Caller().Bytes()))
	return nil, nil
}

func opCallValue(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	v, _ := uint256.FromBig(callContext.Contract.value)
	callContext.Stack.push(v)
	return nil, nil
}

func opCallDataLoad(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	x := callContext.Stack.peek()
	if offset, overflow := x.Uint64WithOverflow(); !overflow {
		data := getData(callContext.Contract.Input, offset, 32)
		x.SetBytes(data)
	} else {
		x.Clear()
	}
	return nil, nil
}

func opCallDataSize(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(uint64(len(callContext.Contract.Input))))
	return nil, nil
}

func opCallDataCopy(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		memOffset  = callContext.Stack.pop()
		dataOffset = callContext.Stack.pop()
		length     = callContext.Stack.pop()
	)
	dataOffset64, overflow := dataOffset.Uint64WithOverflow()
	if overflow {
		dataOffset64 = 0xffffffffffffffff
	}
	// These values are checked for overflow during gas cost calculation
	memOffset64 := memOffset.Uint64()
	length64 := length.Uint64()
	callContext.Memory.Set(memOffset64, length64, getData(callContext.Contract.Input, dataOffset64, length64))

	return nil, nil
}

func opReturnDataSize(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(uint64(len(kvm.interpreter.returnData))))
	return nil, nil
}

func opReturnDataCopy(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		memOffset  = callContext.Stack.pop()
		dataOffset = callContext.Stack.pop()
		length     = callContext.Stack.pop()
	)

	offset64, overflow := dataOffset.Uint64WithOverflow()
	if overflow {
		return nil, ErrReturnDataOutOfBounds
	}
	// we can reuse dataOffset now (aliasing it for clarity)
	var end = dataOffset
	end.Add(&dataOffset, &length)
	end64, overflow := end.Uint64WithOverflow()
	if overflow || uint64(len(kvm.interpreter.returnData)) < end64 {
		return nil, ErrReturnDataOutOfBounds
	}
	callContext.Memory.Set(memOffset.Uint64(), length.Uint64(), kvm.interpreter.returnData[offset64:end64])
	return nil, nil
}

func opExtCodeSize(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	slot := callContext.Stack.peek()
	slot.SetUint64(uint64(kvm.StateDB.GetCodeSize(common.Address(slot.Bytes20()))))
	return nil, nil
}

func opCodeSize(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	l := new(uint256.Int)
	l.SetUint64(uint64(len(callContext.Contract.Code)))
	callContext.Stack.push(l)
	return nil, nil
}

func opCodeCopy(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		memOffset  = callContext.Stack.pop()
		codeOffset = callContext.Stack.pop()
		length     = callContext.Stack.pop()
	)
	uint64CodeOffset, overflow := codeOffset.Uint64WithOverflow()
	if overflow {
		uint64CodeOffset = 0xffffffffffffffff
	}
	codeCopy := getData(callContext.Contract.Code, uint64CodeOffset, length.Uint64())
	callContext.Memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)
	return nil, nil
}

func opExtCodeCopy(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		stack      = callContext.Stack
		a          = stack.pop()
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	uint64CodeOffset, overflow := codeOffset.Uint64WithOverflow()
	if overflow {
		uint64CodeOffset = 0xffffffffffffffff
	}
	addr := common.Address(a.Bytes20())
	codeCopy := getData(kvm.StateDB.GetCode(addr), uint64CodeOffset, length.Uint64())
	callContext.Memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)
	return nil, nil
}

// opExtCodeHash returns the code hash of a specified account.
// There are several cases when the function is called, while we can relay everything
// to `state.GetCodeHash` function to ensure the correctness.
//   (1) Caller tries to get the code hash of a normal Contract account, state
// should return the relative code hash and set it as the result.
//
//   (2) Caller tries to get the code hash of a non-existent account, state should
// return common.Hash{} and zero will be set as the result.
//
//   (3) Caller tries to get the code hash for an account without Contract code,
// state should return emptyCodeHash(0xc5d246...) as the result.
//
//   (4) Caller tries to get the code hash of a precompiled account, the result
// should be zero or emptyCodeHash.
//
// It is worth noting that in order to avoid unnecessary create and clean,
// all precompile accounts on mainnet have been transferred 1 wei, so the return
// here should be emptyCodeHash.
// If the precompile account is not transferred any amount on a private or
// customized chain, the return value will be zero.
//
//   (5) Caller tries to get the code hash for an account which is marked as suicided
// in the current transaction, the code hash of this account should be returned.
//
//   (6) Caller tries to get the code hash for an account which is marked as deleted,
// this account should be regarded as a non-existent account and zero should be returned.
func opExtCodeHash(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	slot := callContext.Stack.peek()
	address := common.Address(slot.Bytes20())
	if kvm.StateDB.Empty(address) {
		slot.Clear()
	} else {
		slot.SetBytes(kvm.StateDB.GetCodeHash(address).Bytes())
	}
	return nil, nil
}

func opGasprice(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	v, _ := uint256.FromBig(kvm.GasPrice)
	callContext.Stack.push(v)
	return nil, nil
}

func opBlockhash(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	num := callContext.Stack.peek()
	num64, overflow := num.Uint64WithOverflow()
	if overflow {
		num.Clear()
		return nil, nil
	}
	var upper, lower uint64
	upper = kvm.BlockHeight.Uint64()
	if upper < 257 {
		lower = 0
	} else {
		lower = upper - 256
	}
	if num64 >= lower && num64 < upper {
		num.SetBytes(kvm.GetHash(num64).Bytes())
	} else {
		num.Clear()
	}
	return nil, nil
}

func opCoinbase(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetBytes(kvm.Coinbase.Bytes()))
	return nil, nil
}

func opTimestamp(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	v, _ := uint256.FromBig(kvm.Time)
	callContext.Stack.push(v)
	return nil, nil
}

func opNumber(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	v, _ := uint256.FromBig(kvm.BlockHeight)
	callContext.Stack.push(v)
	return nil, nil
}

func opGasLimit(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(kvm.GasLimit))
	return nil, nil
}

func opPop(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.pop()
	return nil, nil
}

func opMload(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	v := callContext.Stack.peek()
	offset := int64(v.Uint64())
	v.SetBytes(callContext.Memory.GetPtr(offset, 32))
	return nil, nil
}

func opMstore(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	// pop value of the Stack
	mStart, val := callContext.Stack.pop(), callContext.Stack.pop()
	callContext.Memory.Set32(mStart.Uint64(), &val)
	return nil, nil
}

func opMstore8(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	off, val := callContext.Stack.pop(), callContext.Stack.pop()
	callContext.Memory.store[off.Uint64()] = byte(val.Uint64())
	return nil, nil
}

func opSload(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	loc := callContext.Stack.peek()
	hash := common.Hash(loc.Bytes32())
	val := kvm.StateDB.GetState(callContext.Contract.Address(), hash)
	loc.SetBytes(val.Bytes())
	return nil, nil
}

func opSstore(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	loc := callContext.Stack.pop()
	val := callContext.Stack.pop()
	kvm.StateDB.SetState(callContext.Contract.Address(),
		common.Hash(loc.Bytes32()), common.Hash(val.Bytes32()))
	return nil, nil
}

func opJump(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	pos := callContext.Stack.pop()
	if !callContext.Contract.validJumpdest(&pos) {
		return nil, ErrInvalidJump
	}
	*pc = pos.Uint64()
	return nil, nil
}

func opJumpi(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	pos, cond := callContext.Stack.pop(), callContext.Stack.pop()
	if !cond.IsZero() {
		if !callContext.Contract.validJumpdest(&pos) {
			return nil, ErrInvalidJump
		}
		*pc = pos.Uint64()
	} else {
		*pc++
	}

	return nil, nil
}

func opJumpdest(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	return nil, nil
}

func opBeginSub(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	return nil, ErrInvalidSubroutineEntry
}

func opJumpSub(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	if len(callContext.Rstack.data) >= 1023 {
		return nil, ErrReturnStackExceeded
	}
	pos := callContext.Stack.pop()
	if !pos.IsUint64() {
		return nil, ErrInvalidJump
	}
	posU64 := pos.Uint64()
	if !callContext.Contract.validJumpSubdest(posU64) {
		return nil, ErrInvalidJump
	}
	callContext.Rstack.push(uint32(*pc))
	*pc = posU64 + 1
	return nil, nil
}

func opReturnSub(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	if len(callContext.Rstack.data) == 0 {
		return nil, ErrInvalidRetsub
	}
	// Other than the check that the return Stack is not empty, there is no
	// need to validate the pc from 'returns', since we only ever push valid
	//values onto it via jumpsub.
	*pc = uint64(callContext.Rstack.pop()) + 1
	return nil, nil
}

func opPc(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(*pc))
	return nil, nil
}

func opMsize(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(uint64(callContext.Memory.Len())))
	return nil, nil
}

func opGas(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	callContext.Stack.push(new(uint256.Int).SetUint64(callContext.Contract.Gas))
	return nil, nil
}

func opCreate(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		value        = callContext.Stack.pop()
		offset, size = callContext.Stack.pop(), callContext.Stack.pop()
		input        = callContext.Memory.GetCopy(int64(offset.Uint64()), int64(size.Uint64()))
		gas          = callContext.Contract.Gas
	)
	// TODO: potentially use "all but one 64th" gas rule here
	gas -= gas / 64

	// reuse size int for stackvalue
	stackvalue := size

	callContext.Contract.UseGas(gas)
	//TODO: use uint256.Int instead of converting with toBig()
	var bigVal = big0
	if !value.IsZero() {
		bigVal = value.ToBig()
	}

	res, addr, returnGas, suberr := kvm.Create(callContext.Contract, input, gas, bigVal)
	// All returned errors including CodeStoreOutOfGasError are treated as error.
	// KVM run similar to EVM from Homestead ruleset.
	if suberr == ErrCodeStoreOutOfGas {
		stackvalue.Clear()
	} else if suberr != nil && suberr != ErrCodeStoreOutOfGas {
		stackvalue.Clear()
	} else {
		stackvalue.SetBytes(addr.Bytes())
	}

	callContext.Stack.push(&stackvalue)
	callContext.Contract.Gas += returnGas

	if suberr == ErrExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCreate2(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		endowment    = callContext.Stack.pop()
		offset, size = callContext.Stack.pop(), callContext.Stack.pop()
		salt         = callContext.Stack.pop()
		input        = callContext.Memory.GetCopy(int64(offset.Uint64()), int64(size.Uint64()))
		gas          = callContext.Contract.Gas
	)

	// TODO: potentially use  "all but one 64th" gas rule here
	// gas -= gas / 64
	callContext.Contract.UseGas(gas)
	// reuse size int for stackvalue
	stackvalue := size
	//TODO: use uint256.Int instead of converting with toBig()
	bigEndowment := big0
	if !endowment.IsZero() {
		bigEndowment = endowment.ToBig()
	}
	res, addr, returnGas, suberr := kvm.Create2(callContext.Contract, input, gas,
		bigEndowment, &salt)
	// Push item on the Stack based on the returned error.
	if suberr != nil {
		stackvalue.Clear()
	} else {
		stackvalue.SetBytes(addr.Bytes())
	}
	callContext.Stack.push(&stackvalue)
	callContext.Contract.Gas += returnGas

	if suberr == ErrExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCall(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	stack := callContext.Stack
	// Pop gas. The actual gas in interpreter.evm.callGasTemp.
	// We can use this as a temporary value
	temp := stack.pop()
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.Address(addr.Bytes20())
	// Get the arguments from the Memory.
	args := callContext.Memory.GetPtr(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	var bigVal = big0
	//TODO: use uint256.Int instead of converting with toBig()
	// By using big0 here, we save an alloc for the most common case (non-ether-transferring Contract calls),
	// but it would make more sense to extend the usage of uint256.Int
	if !value.IsZero() {
		gas += configs.CallStipend
		bigVal = value.ToBig()
	}

	ret, returnGas, err := kvm.Call(callContext.Contract, toAddr, args, gas, bigVal)

	if err != nil {
		temp.Clear()
	} else {
		temp.SetOne()
	}
	stack.push(&temp)
	if err == nil || err == ErrExecutionReverted {
		callContext.Memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.Contract.Gas += returnGas

	return ret, nil
}

func opCallCode(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	// Pop gas. The actual gas is in interpreter.evm.callGasTemp.
	stack := callContext.Stack
	// We use it as a temporary value
	temp := stack.pop()
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.Address(addr.Bytes20())
	// Get arguments from the Memory.
	args := callContext.Memory.GetPtr(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	//TODO: use uint256.Int instead of converting with toBig()
	var bigVal = big0
	if !value.IsZero() {
		gas += configs.CallStipend
		bigVal = value.ToBig()
	}

	ret, returnGas, err := kvm.CallCode(callContext.Contract, toAddr, args, gas, bigVal)
	if err != nil {
		temp.Clear()
	} else {
		temp.SetOne()
	}
	stack.push(&temp)
	if err == nil || err == ErrExecutionReverted {
		callContext.Memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.Contract.Gas += returnGas

	return ret, nil
}

func opDelegateCall(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	stack := callContext.Stack
	// Pop gas. The actual gas is in interpreter.evm.callGasTemp.
	// We use it as a temporary value
	temp := stack.pop()
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.Address(addr.Bytes20())
	// Get arguments from the Memory.
	args := callContext.Memory.GetPtr(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	ret, returnGas, err := kvm.DelegateCall(callContext.Contract, toAddr, args, gas)
	if err != nil {
		temp.Clear()
	} else {
		temp.SetOne()
	}
	stack.push(&temp)
	if err == nil || err == ErrExecutionReverted {
		callContext.Memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.Contract.Gas += returnGas

	return ret, nil
}

func opStaticCall(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	// Pop gas. The actual gas is in interpreter.evm.callGasTemp.
	stack := callContext.Stack
	// We use it as a temporary value
	temp := stack.pop()
	gas := kvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.Address(addr.Bytes20())
	// Get arguments from the Memory.
	args := callContext.Memory.GetPtr(int64(inOffset.Uint64()), int64(inSize.Uint64()))

	ret, returnGas, err := kvm.StaticCall(callContext.Contract, toAddr, args, gas)
	if err != nil {
		temp.Clear()
	} else {
		temp.SetOne()
	}
	stack.push(&temp)
	if err == nil || err == ErrExecutionReverted {
		callContext.Memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	callContext.Contract.Gas += returnGas

	return ret, nil
}

func opReturn(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	offset, size := callContext.Stack.pop(), callContext.Stack.pop()
	ret := callContext.Memory.GetPtr(int64(offset.Uint64()), int64(size.Uint64()))

	return ret, nil
}

func opRevert(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	offset, size := callContext.Stack.pop(), callContext.Stack.pop()
	ret := callContext.Memory.GetPtr(int64(offset.Uint64()), int64(size.Uint64()))

	return ret, nil
}

func opStop(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	return nil, nil
}

func opSuicide(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	beneficiary := callContext.Stack.pop()
	balance := kvm.StateDB.GetBalance(callContext.Contract.Address())
	kvm.StateDB.AddBalance(common.Address(beneficiary.Bytes20()), balance)
	kvm.StateDB.Suicide(callContext.Contract.Address())
	return nil, nil
}

// NOT SUPPPORT ChainID yet
// opChainID implements CHAINID opcode
// func opChainID(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
// 	chainId, _ := uint256.FromBig(kvm.vmConfig.ChainID)
// 	callContext.Stack.push(chainId)
// 	return nil, nil
// }

func opSelfBalance(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	balance, _ := uint256.FromBig(kvm.StateDB.GetBalance(callContext.Contract.Address()))
	callContext.Stack.push(balance)
	return nil, nil
}

// make log instruction function
func makeLog(size int) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
		topics := make([]common.Hash, size)
		stack := callContext.Stack
		mStart, mSize := stack.pop(), stack.pop()
		for i := 0; i < size; i++ {
			addr := stack.pop()
			topics[i] = common.Hash(addr.Bytes32())
		}

		d := callContext.Memory.GetCopy(int64(mStart.Uint64()), int64(mSize.Uint64()))
		kvm.StateDB.AddLog(&types.Log{
			Address: callContext.Contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block height.
			BlockHeight: kvm.BlockHeight.Uint64(),
		})

		return nil, nil
	}
}

// opPush1 is a specialized version of pushN
func opPush1(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
	var (
		codeLen = uint64(len(callContext.Contract.Code))
		integer = new(uint256.Int)
	)
	*pc += 1
	if *pc < codeLen {
		callContext.Stack.push(integer.SetUint64(uint64(callContext.Contract.Code[*pc])))
	} else {
		callContext.Stack.push(integer.Clear())
	}
	return nil, nil
}

// make push instruction function
func makePush(size uint64, pushByteSize int) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
		codeLen := len(callContext.Contract.Code)

		startMin := codeLen
		if int(*pc+1) < startMin {
			startMin = int(*pc + 1)
		}

		endMin := codeLen
		if startMin+pushByteSize < endMin {
			endMin = startMin + pushByteSize
		}

		integer := new(uint256.Int)
		callContext.Stack.push(integer.SetBytes(common.RightPadBytes(
			callContext.Contract.Code[startMin:endMin], pushByteSize)))

		*pc += size
		return nil, nil
	}
}

// make dup instruction function
func makeDup(size int64) executionFunc {
	return func(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
		callContext.Stack.dup(int(size))
		return nil, nil
	}
}

// make swap instruction function
func makeSwap(size int64) executionFunc {
	// switch n + 1 otherwise n would be swapped with n
	size++
	return func(pc *uint64, kvm *KVM, callContext *ScopeContext) ([]byte, error) {
		callContext.Stack.swap(int(size))
		return nil, nil
	}
}
