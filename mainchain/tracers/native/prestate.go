/*
 *  Copyright 2022 KardiaChain
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

package native

import (
	"encoding/json"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/mainchain/tracers"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
)

func init() {
	register("prestateTracer", newPrestateTracer)
}

type prestate = map[common.Address]*account
type account struct {
	Balance string                      `json:"balance"`
	Nonce   uint64                      `json:"nonce"`
	Code    string                      `json:"code"`
	Storage map[common.Hash]common.Hash `json:"storage"`
}

type prestateTracer struct {
	env       *kvm.KVM
	prestate  prestate
	create    bool
	to        common.Address
	interrupt uint32 // Atomic flag to signal execution interruption
	reason    error  // Textual reason for the interruption
}

func newPrestateTracer() tracers.Tracer {
	// First callframe contains tx context info
	// and is populated on start and end.
	return &prestateTracer{prestate: prestate{}}
}

// CaptureStart implements the KVMLogger interface to initialize the tracing operation.
func (t *prestateTracer) CaptureStart(env *kvm.KVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	t.create = create
	t.to = to

	// Compute intrinsic gas
	blockHeight := env.BlockContext.BlockHeight.Uint64()
	isGalaxias := env.ChainConfig().IsGalaxias(&blockHeight)
	intrinsicGas, err := tx_pool.IntrinsicGas(input, create, !isGalaxias)
	if err != nil {
		return
	}

	t.lookupAccount(from)
	t.lookupAccount(to)

	// The recipient balance includes the value transferred.
	toBal := common.MustDecodeBig(t.prestate[to].Balance)
	toBal = new(big.Int).Sub(toBal, value)
	t.prestate[to].Balance = common.EncodeBig(toBal)

	// The sender balance is after reducing: value, gasLimit, intrinsicGas.
	// We need to re-add them to get the pre-tx balance.
	fromBal := common.MustDecodeBig(t.prestate[from].Balance)
	gasPrice := env.TxContext.GasPrice
	consumedGas := new(big.Int).Mul(
		gasPrice,
		new(big.Int).Add(
			new(big.Int).SetUint64(intrinsicGas),
			new(big.Int).SetUint64(gas),
		),
	)
	fromBal.Add(fromBal, new(big.Int).Add(value, consumedGas))
	t.prestate[from].Balance = common.EncodeBig(fromBal)
	t.prestate[from].Nonce--
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *prestateTracer) CaptureEnd(output []byte, gasUsed uint64, _ time.Duration, err error) {
	if t.create {
		// Exclude created contract.
		delete(t.prestate, t.to)
	}
}

// CaptureState implements the KVMLogger interface to trace a single step of VM execution.
func (t *prestateTracer) CaptureState(pc uint64, op kvm.OpCode, gas, cost uint64, scope *kvm.ScopeContext, rData []byte, depth int, err error) {
	stack := scope.Stack
	stackData := stack.Data()
	stackLen := len(stackData)
	switch {
	case stackLen >= 1 && (op == kvm.SLOAD || op == kvm.SSTORE):
		slot := common.Hash(stackData[stackLen-1].Bytes32())
		t.lookupStorage(scope.Contract.Address(), slot)
	case stackLen >= 1 && (op == kvm.EXTCODECOPY || op == kvm.EXTCODEHASH || op == kvm.EXTCODESIZE || op == kvm.BALANCE || op == kvm.SELFDESTRUCT):
		addr := common.Address(stackData[stackLen-1].Bytes20())
		t.lookupAccount(addr)
	case stackLen >= 5 && (op == kvm.DELEGATECALL || op == kvm.CALL || op == kvm.STATICCALL || op == kvm.CALLCODE):
		addr := common.Address(stackData[stackLen-2].Bytes20())
		t.lookupAccount(addr)
	case op == kvm.CREATE:
		addr := scope.Contract.Address()
		nonce := t.env.StateDB.GetNonce(addr)
		t.lookupAccount(crypto.CreateAddress(addr, nonce))
	case stackLen >= 4 && op == kvm.CREATE2:
		offset := stackData[stackLen-2]
		size := stackData[stackLen-3]
		init := scope.Memory.GetCopy(int64(offset.Uint64()), int64(size.Uint64()))
		inithash := crypto.Keccak256(init)
		salt := stackData[stackLen-4]
		t.lookupAccount(crypto.CreateAddress2(scope.Contract.Address(), salt.Bytes32(), inithash))
	}
}

// CaptureFault implements the KVMLogger interface to trace an execution fault.
func (t *prestateTracer) CaptureFault(pc uint64, op kvm.OpCode, gas, cost uint64, _ *kvm.ScopeContext, depth int, err error) {
}

// CaptureEnter is called when KVM enters a new scope (via CALL, CREATE or SELFDESTRUCT).
func (t *prestateTracer) CaptureEnter(typ kvm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

// CaptureExit is called when KVM exits a scope, even if the scope didn't
// execute any code.
func (t *prestateTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
}

// GetResult returns the json-encoded nested list of call traces, and any
// error arising from the encoding or forceful termination (via `Stop`).
func (t *prestateTracer) GetResult() (json.RawMessage, error) {
	res, err := json.Marshal(t.prestate)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *prestateTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}

// lookupAccount fetches details of an account and adds it to the prestate
// if it doesn't exist there.
func (t *prestateTracer) lookupAccount(addr common.Address) {
	if _, ok := t.prestate[addr]; ok {
		return
	}
	t.prestate[addr] = &account{
		Balance: bigToHex(t.env.StateDB.GetBalance(addr)),
		Nonce:   t.env.StateDB.GetNonce(addr),
		Code:    bytesToHex(t.env.StateDB.GetCode(addr)),
		Storage: make(map[common.Hash]common.Hash),
	}
}

// lookupStorage fetches the requested storage slot and adds
// it to the prestate of the given contract. It assumes `lookupAccount`
// has been performed on the contract before.
func (t *prestateTracer) lookupStorage(addr common.Address, key common.Hash) {
	if _, ok := t.prestate[addr].Storage[key]; ok {
		return
	}
	t.prestate[addr].Storage[key] = t.env.StateDB.GetState(addr, key)
}
