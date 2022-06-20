/*
 *  Copyright 2021 KardiaChain
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
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/tracers"
)

func init() {
	register("callTracer", newCallTracer)
}

type callFrame struct {
	Type                       string      `json:"type"`
	CallType                   string      `json:"callType,omitempty"`
	From                       string      `json:"from"`
	Init                       string      `json:"init,omitempty"`
	CreatedContractAddressHash string      `json:"createdContractAddressHash,omitempty"`
	CreatedContractCode        string      `json:"createdContractCode,omitempty"`
	To                         string      `json:"to,omitempty"`
	Input                      string      `json:"input,omitempty"`
	Output                     string      `json:"output,omitempty"`
	Error                      string      `json:"error,omitempty"`
	TraceAddress               []int       `json:"traceAddress"`
	Value                      string      `json:"value,omitempty"`
	Gas                        string      `json:"gas"`
	GasUsed                    string      `json:"gasUsed"`
	Calls                      []callFrame `json:"calls,omitempty"`

	// optional CREATE fields

}

type callTracer struct {
	env       *kvm.KVM
	callstack []callFrame
	interrupt uint32 // Atomic flag to signal execution interruption
	reason    error  // Textual reason for the interruption
	name      string // to distinguish the native call tracer from the replay tracer
}

// newCallTracer returns a native go tracer which tracks
// call frames of a tx, and implements kvm.KVMLogger.
func newCallTracer() tracers.Tracer {
	// First callframe contains tx context info
	// and is populated on start and end.
	return &callTracer{
		callstack: make([]callFrame, 1),
		name:      "callTracer",
	}
}

// CaptureStart implements the KVMLogger interface to initialize the tracing operation.
func (t *callTracer) CaptureStart(env *kvm.KVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	t.callstack[0] = callFrame{
		Type:         "CALL",
		From:         addrToHex(from),
		To:           addrToHex(to),
		Input:        bytesToHex(input),
		Gas:          uintToHex(gas),
		Value:        bigToHex(value),
		TraceAddress: []int{},
	}
	if create {
		t.callstack[0].Type = "CREATE"
	}
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *callTracer) CaptureEnd(output []byte, gasUsed uint64, _ time.Duration, err error) {
	t.callstack[0].GasUsed = uintToHex(gasUsed)
	if err != nil {
		t.callstack[0].Error = err.Error()
		if err.Error() == "execution reverted" && len(output) > 0 {
			t.callstack[0].Output = bytesToHex(output)
		}
	} else {
		t.callstack[0].Output = bytesToHex(output)
	}
}

// CaptureState implements the KVMLogger interface to trace a single step of VM execution.
func (t *callTracer) CaptureState(pc uint64, op kvm.OpCode, gas, cost uint64, scope *kvm.ScopeContext, rData []byte, depth int, err error) {
}

// CaptureFault implements the KVMLogger interface to trace an execution fault.
func (t *callTracer) CaptureFault(pc uint64, op kvm.OpCode, gas, cost uint64, _ *kvm.ScopeContext, depth int, err error) {
}

// CaptureEnter is called when KVM enters a new scope (via call, create or selfdestruct).
func (t *callTracer) CaptureEnter(typ kvm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	// Skip if tracing was interrupted
	if atomic.LoadUint32(&t.interrupt) > 0 {
		t.env.Cancel()
		return
	}

	call := callFrame{
		Type:  typ.String(),
		From:  addrToHex(from),
		To:    addrToHex(to),
		Input: bytesToHex(input),
		Gas:   uintToHex(gas),
		Value: bigToHex(value),
	}
	t.callstack = append(t.callstack, call)
}

// CaptureExit is called when KVM exits a scope, even if the scope didn't
// execute any code.
func (t *callTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	size := len(t.callstack)
	if size <= 1 {
		return
	}
	// pop call
	call := t.callstack[size-1]
	t.callstack = t.callstack[:size-1]
	size -= 1

	call.GasUsed = uintToHex(gasUsed)
	if err == nil {
		call.Output = bytesToHex(output)
	} else {
		call.Error = err.Error()
		if call.Type == "CREATE" || call.Type == "CREATE2" {
			call.To = ""
		}
	}
	t.callstack[size-1].Calls = append(t.callstack[size-1].Calls, call)
}

// GetResult returns the json-encoded nested list of call traces, and any
// error arising from the encoding or forceful termination (via `Stop`).
func (t *callTracer) GetResult() (json.RawMessage, error) {
	if len(t.callstack) != 1 {
		return nil, errors.New("incorrect number of top-level calls")
	}
	var (
		res []byte
		err error
	)
	if t.name == "replayTracer" {
		res, err = json.Marshal(t.formatReplayedStack())
	} else {
		res, err = json.Marshal(t.callstack[0])
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *callTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}

func bytesToHex(s []byte) string {
	return "0x" + common.Bytes2Hex(s)
}

func bigToHex(n *big.Int) string {
	if n == nil {
		return ""
	}
	return "0x" + n.Text(16)
}

func uintToHex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func addrToHex(a common.Address) string {
	return strings.ToLower(a.Hex())
}

func (t *callTracer) formatReplayedStack() []callFrame {
	replayedStack := sequence(t.callstack[0], []callFrame{}, []int{})
	for i := range replayedStack {
		formatCallType(&replayedStack[i])
	}
	return replayedStack
}

func sequence(call callFrame, callSequence []callFrame, traceAddress []int) []callFrame {
	subcalls := call.Calls
	call.Calls = nil

	call.TraceAddress = traceAddress

	newCallSequence := append(callSequence, call)
	if subcalls != nil {
		for i := range subcalls {
			newCallSequence = sequence(subcalls[i], newCallSequence, append(traceAddress, i))
		}
	}
	return newCallSequence
}

func formatCallType(in *callFrame) {
	switch in.Type {
	case "CALL", "DELEGATECALL", "STATICCALL":
		in.CallType = strings.ToLower(in.Type)
		in.Type = "call"
	case "CREATE", "CREATE2":
		in.CallType = ""
		in.Type = "create"
		in.Init = in.Input
		in.CreatedContractCode = in.Output
		in.CreatedContractAddressHash = in.To
		in.Input = ""
		in.Output = ""
		in.To = ""
	case "SELFDESTRUCT":
		in.CallType = strings.ToLower(in.Type)
		in.Type = "selfdestruct"
	}
}
