package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kai/accounts"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"

	"github.com/holiman/uint256"
)

const (
	callTimeout    = 5 * time.Minute
	NotImplemented = "the method is currently not implemented: %s"
	// defaultTraceReexec is the number of blocks the tracer is willing to go back
	// and reexecute to produce missing historical state necessary to run a specific
	// trace.
	defaultTraceReexec = uint64(128)
)

const (
	CALL               = "call"
	CALLCODE           = "callcode"
	DELEGATECALL       = "delegatecall"
	STATICCALL         = "staticcall"
	CREATE             = "create"
	SUICIDE            = "suicide"
	REWARD             = "reward"
	TraceTypeTrace     = "trace"
	TraceTypeStateDiff = "stateDiff"
	TraceTypeVmTrace   = "vmTrace"
)

// TraceCallResult is the response to `trace_call` method
type TraceCallResult struct {
	Output          common.Bytes                         `json:"output"`
	StateDiff       map[common.Address]*StateDiffAccount `json:"stateDiff"`
	Trace           []*ParityTrace                       `json:"trace"`
	VmTrace         *VmTrace                             `json:"vmTrace"`
	TransactionHash *common.Hash                         `json:"transactionHash,omitempty"`
}

// StateDiffAccount is the part of `trace_call` response that is under "stateDiff" tag
type StateDiffAccount struct {
	Balance interface{}                            `json:"balance"` // Can be either string "=" or mapping "*" => {"from": "hex", "to": "hex"}
	Code    interface{}                            `json:"code"`
	Nonce   interface{}                            `json:"nonce"`
	Storage map[common.Hash]map[string]interface{} `json:"storage"`
}

type StateDiffBalance struct {
	From *common.Big `json:"from"`
	To   *common.Big `json:"to"`
}

type StateDiffCode struct {
	From common.Bytes `json:"from"`
	To   common.Bytes `json:"to"`
}

type StateDiffNonce struct {
	From common.Uint64 `json:"from"`
	To   common.Uint64 `json:"to"`
}

type StateDiffStorage struct {
	From common.Hash `json:"from"`
	To   common.Hash `json:"to"`
}

// VmTrace is the part of `trace_call` response that is under "vmTrace" tag
type VmTrace struct {
	Code common.Bytes `json:"code"`
	Ops  []*VmTraceOp `json:"ops"`
}

// VmTraceOp is one element of the vmTrace ops trace
type VmTraceOp struct {
	Cost int        `json:"cost"`
	Ex   *VmTraceEx `json:"ex"`
	Pc   int        `json:"pc"`
	Sub  *VmTrace   `json:"sub"`
	Op   string     `json:"op,omitempty"`
	Idx  string     `json:"idx,omitempty"`
}

type VmTraceEx struct {
	Mem   *VmTraceMem   `json:"mem"`
	Push  []string      `json:"push"`
	Store *VmTraceStore `json:"store"`
	Used  int           `json:"used"`
}

type VmTraceMem struct {
	Data string `json:"data"`
	Off  int    `json:"off"`
}

type VmTraceStore struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

// OpenEthereum-style tracer
type OeTracer struct {
	r            *TraceCallResult
	traceAddr    []int
	traceStack   []*ParityTrace
	precompile   bool // Whether the last CaptureStart was called with `precompile = true`
	compat       bool // Bug for bug compatibility mode
	lastVmOp     *VmTraceOp
	lastOp       kvm.OpCode
	lastMemOff   uint64
	lastMemLen   uint64
	memOffStack  []uint64
	memLenStack  []uint64
	lastOffStack *VmTraceOp
	vmOpStack    []*VmTraceOp // Stack of vmTrace operations as call depth increases
	idx          []string     // Prefix for the "idx" inside operations, for easier navigation
}

func (ot *OeTracer) CaptureStart(env *kvm.KVM, depth int, from common.Address, to common.Address, precompile bool, create bool, calltype kvm.CallType, input []byte, gas uint64, value *big.Int, code []byte) {
	//fmt.Printf("CaptureStart depth %d, from %x, to %x, create %t, input %x, gas %d, value %d, precompile %t\n", depth, from, to, create, input, gas, value, precompile)
	if ot.r.VmTrace != nil {
		var vmTrace *VmTrace
		if depth > 0 {
			var vmT *VmTrace
			if len(ot.vmOpStack) > 0 {
				vmT = ot.vmOpStack[len(ot.vmOpStack)-1].Sub
			} else {
				vmT = ot.r.VmTrace
			}
			if !ot.compat {
				ot.idx = append(ot.idx, fmt.Sprintf("%d-", len(vmT.Ops)-1))
			}
		}
		if ot.lastVmOp != nil {
			vmTrace = &VmTrace{Ops: []*VmTraceOp{}}
			ot.lastVmOp.Sub = vmTrace
			ot.vmOpStack = append(ot.vmOpStack, ot.lastVmOp)
		} else {
			vmTrace = ot.r.VmTrace
		}
		if create {
			vmTrace.Code = common.CopyBytes(input)
			if ot.lastVmOp != nil {
				ot.lastVmOp.Cost += int(gas)
			}
		} else {
			vmTrace.Code = code
		}
	}
	if precompile && depth > 0 && value.Sign() <= 0 {
		ot.precompile = true
		return
	}
	if gas > 500000000 {
		gas = 500000001 - (0x8000000000000000 - gas)
	}
	trace := &ParityTrace{}
	if create {
		trResult := &CreateTraceResult{}
		trace.Type = CREATE
		trResult.Address = new(common.Address)
		copy(trResult.Address[:], to.Bytes())
		trace.Result = trResult
	} else {
		trace.Result = &TraceResult{}
		trace.Type = CALL
	}
	if depth > 0 {
		topTrace := ot.traceStack[len(ot.traceStack)-1]
		traceIdx := topTrace.Subtraces
		ot.traceAddr = append(ot.traceAddr, traceIdx)
		topTrace.Subtraces++
		if calltype == kvm.DELEGATECALLT {
			switch action := topTrace.Action.(type) {
			case *CreateTraceAction:
				value = action.Value.ToInt()
			case *CallTraceAction:
				value = action.Value.ToInt()
			}
		}
		if calltype == kvm.STATICCALLT {
			value = big.NewInt(0)
		}
	}
	trace.TraceAddress = make([]int, len(ot.traceAddr))
	copy(trace.TraceAddress, ot.traceAddr)
	if create {
		action := CreateTraceAction{}
		action.From = from
		action.Gas.ToInt().SetUint64(gas)
		action.Init = common.CopyBytes(input)
		action.Value.ToInt().Set(value)
		trace.Action = &action
	} else {
		action := CallTraceAction{}
		switch calltype {
		case kvm.CALLT:
			action.CallType = CALL
		case kvm.CALLCODET:
			action.CallType = CALLCODE
		case kvm.DELEGATECALLT:
			action.CallType = DELEGATECALL
		case kvm.STATICCALLT:
			action.CallType = STATICCALL
		}
		action.From = from
		action.To = to
		action.Gas.ToInt().SetUint64(gas)
		action.Input = common.CopyBytes(input)
		action.Value.ToInt().Set(value)
		trace.Action = &action
	}
	ot.r.Trace = append(ot.r.Trace, trace)
	ot.traceStack = append(ot.traceStack, trace)
}

func (ot *OeTracer) CaptureEnd(depth int, output []byte, startGas, endGas uint64, t time.Duration, err error) {
	if ot.r.VmTrace != nil {
		if len(ot.vmOpStack) > 0 {
			ot.lastOffStack = ot.vmOpStack[len(ot.vmOpStack)-1]
			ot.vmOpStack = ot.vmOpStack[:len(ot.vmOpStack)-1]
		}
		if !ot.compat && depth > 0 {
			ot.idx = ot.idx[:len(ot.idx)-1]
		}
		if depth > 0 {
			ot.lastMemOff = ot.memOffStack[len(ot.memOffStack)-1]
			ot.memOffStack = ot.memOffStack[:len(ot.memOffStack)-1]
			ot.lastMemLen = ot.memLenStack[len(ot.memLenStack)-1]
			ot.memLenStack = ot.memLenStack[:len(ot.memLenStack)-1]
		}
	}
	if ot.precompile {
		ot.precompile = false
		return
	}
	if depth == 0 {
		ot.r.Output = common.CopyBytes(output)
	}
	ignoreError := false
	topTrace := ot.traceStack[len(ot.traceStack)-1]
	if ot.compat {
		ignoreError = depth == 0 && topTrace.Type == CREATE
	}
	if err != nil && !ignoreError {
		if err == kvm.ErrExecutionReverted {
			topTrace.Error = "Reverted"
			switch topTrace.Type {
			case CALL:
				topTrace.Result.(*TraceResult).GasUsed = new(common.Big)
				topTrace.Result.(*TraceResult).GasUsed.ToInt().SetUint64(startGas - endGas)
				topTrace.Result.(*TraceResult).Output = common.CopyBytes(output)
			case CREATE:
				topTrace.Result.(*CreateTraceResult).GasUsed = new(common.Big)
				topTrace.Result.(*CreateTraceResult).GasUsed.ToInt().SetUint64(startGas - endGas)
				topTrace.Result.(*CreateTraceResult).Code = common.CopyBytes(output)
			}
		} else {
			topTrace.Result = nil
			switch err {
			case kvm.ErrInvalidJump:
				topTrace.Error = "Bad jump destination"
			case kvm.ErrContractAddressCollision, kvm.ErrCodeStoreOutOfGas, kvm.ErrOutOfGas, kvm.ErrGasUintOverflow:
				topTrace.Error = "Out of gas"
			case kvm.ErrWriteProtection:
				topTrace.Error = "Mutable Call In Static Context"
			default:
				switch err.(type) {
				case *kvm.ErrStackUnderflow:
					topTrace.Error = "Stack underflow"
				case *kvm.ErrInvalidOpCode:
					topTrace.Error = "Bad instruction"
				default:
					topTrace.Error = err.Error()
				}
			}
		}
	} else {
		if len(output) > 0 {
			switch topTrace.Type {
			case CALL:
				topTrace.Result.(*TraceResult).Output = common.CopyBytes(output)
			case CREATE:
				topTrace.Result.(*CreateTraceResult).Code = common.CopyBytes(output)
			}
		}
		switch topTrace.Type {
		case CALL:
			topTrace.Result.(*TraceResult).GasUsed = new(common.Big)
			topTrace.Result.(*TraceResult).GasUsed.ToInt().SetUint64(startGas - endGas)
		case CREATE:
			topTrace.Result.(*CreateTraceResult).GasUsed = new(common.Big)
			topTrace.Result.(*CreateTraceResult).GasUsed.ToInt().SetUint64(startGas - endGas)
		}
	}
	ot.traceStack = ot.traceStack[:len(ot.traceStack)-1]
	if depth > 0 {
		ot.traceAddr = ot.traceAddr[:len(ot.traceAddr)-1]
	}
}

func (ot *OeTracer) CaptureState(env *kvm.KVM, pc uint64, op kvm.OpCode, gas, cost uint64, scope *kvm.ScopeContext, rData []byte, opDepth int, err error) {
	memory := scope.Memory
	st := scope.Stack

	if ot.r.VmTrace != nil {
		var vmTrace *VmTrace
		if len(ot.vmOpStack) > 0 {
			vmTrace = ot.vmOpStack[len(ot.vmOpStack)-1].Sub
		} else {
			vmTrace = ot.r.VmTrace
		}
		if ot.lastVmOp != nil && ot.lastVmOp.Ex != nil {
			// Set the "push" of the last operation
			var showStack int
			switch {
			case ot.lastOp >= kvm.PUSH1 && ot.lastOp <= kvm.PUSH32:
				showStack = 1
			case ot.lastOp >= kvm.SWAP1 && ot.lastOp <= kvm.SWAP16:
				showStack = int(ot.lastOp-kvm.SWAP1) + 2
			case ot.lastOp >= kvm.DUP1 && ot.lastOp <= kvm.DUP16:
				showStack = int(ot.lastOp-kvm.DUP1) + 2
			}
			switch ot.lastOp {
			case kvm.CALLDATALOAD, kvm.SLOAD, kvm.MLOAD, kvm.CALLDATASIZE, kvm.LT, kvm.GT, kvm.DIV, kvm.SDIV, kvm.SAR, kvm.AND, kvm.EQ, kvm.CALLVALUE, kvm.ISZERO,
				kvm.ADD, kvm.EXP, kvm.CALLER, kvm.SHA3, kvm.SUB, kvm.ADDRESS, kvm.GAS, kvm.MUL, kvm.RETURNDATASIZE, kvm.NOT, kvm.SHR, kvm.SHL,
				kvm.EXTCODESIZE, kvm.SLT, kvm.OR, kvm.NUMBER, kvm.PC, kvm.TIMESTAMP, kvm.BALANCE, kvm.SELFBALANCE, kvm.MULMOD, kvm.ADDMOD,
				kvm.BLOCKHASH, kvm.BYTE, kvm.XOR, kvm.ORIGIN, kvm.CODESIZE, kvm.MOD, kvm.SIGNEXTEND, kvm.GASLIMIT, kvm.SGT, kvm.GASPRICE,
				kvm.MSIZE, kvm.EXTCODEHASH, kvm.SMOD, kvm.CHAINID, kvm.COINBASE:
				showStack = 1
			}
			for i := showStack - 1; i >= 0; i-- {
				ot.lastVmOp.Ex.Push = append(ot.lastVmOp.Ex.Push, st.Back(i).String())
			}
			// Set the "mem" of the last operation
			var setMem bool
			switch ot.lastOp {
			case kvm.MSTORE, kvm.MSTORE8, kvm.MLOAD, kvm.RETURNDATACOPY, kvm.CALLDATACOPY, kvm.CODECOPY:
				setMem = true
			}
			if setMem && ot.lastMemLen > 0 {
				cpy := memory.GetCopy(int64(ot.lastMemOff), int64(ot.lastMemLen))
				if len(cpy) == 0 {
					cpy = make([]byte, ot.lastMemLen)
				}
				ot.lastVmOp.Ex.Mem = &VmTraceMem{Data: fmt.Sprintf("0x%0x", cpy), Off: int(ot.lastMemOff)}
			}
		}
		if ot.lastOffStack != nil {
			ot.lastOffStack.Ex.Used = int(gas)
			ot.lastOffStack.Ex.Push = []string{st.Back(0).String()}
			if ot.lastMemLen > 0 && memory != nil {
				cpy := memory.GetCopy(int64(ot.lastMemOff), int64(ot.lastMemLen))
				if len(cpy) == 0 {
					cpy = make([]byte, ot.lastMemLen)
				}
				ot.lastOffStack.Ex.Mem = &VmTraceMem{Data: fmt.Sprintf("0x%0x", cpy), Off: int(ot.lastMemOff)}
			}
			ot.lastOffStack = nil
		}
		if ot.lastOp == kvm.STOP && op == kvm.STOP && len(ot.vmOpStack) == 0 {
			// Looks like OE is "optimising away" the second STOP
			return
		}
		ot.lastVmOp = &VmTraceOp{Ex: &VmTraceEx{}}
		vmTrace.Ops = append(vmTrace.Ops, ot.lastVmOp)
		if !ot.compat {
			var sb strings.Builder
			for _, idx := range ot.idx {
				sb.WriteString(idx)
			}
			ot.lastVmOp.Idx = fmt.Sprintf("%s%d", sb.String(), len(vmTrace.Ops)-1)
		}
		ot.lastOp = op
		ot.lastVmOp.Cost = int(cost)
		ot.lastVmOp.Pc = int(pc)
		ot.lastVmOp.Ex.Push = []string{}
		ot.lastVmOp.Ex.Used = int(gas) - int(cost)
		if !ot.compat {
			ot.lastVmOp.Op = op.String()
		}
		switch op {
		case kvm.MSTORE, kvm.MLOAD:
			ot.lastMemOff = st.Back(0).Uint64()
			ot.lastMemLen = 32
		case kvm.MSTORE8:
			ot.lastMemOff = st.Back(0).Uint64()
			ot.lastMemLen = 1
		case kvm.RETURNDATACOPY, kvm.CALLDATACOPY, kvm.CODECOPY:
			ot.lastMemOff = st.Back(0).Uint64()
			ot.lastMemLen = st.Back(2).Uint64()
		case kvm.STATICCALL, kvm.DELEGATECALL:
			ot.memOffStack = append(ot.memOffStack, st.Back(4).Uint64())
			ot.memLenStack = append(ot.memLenStack, st.Back(5).Uint64())
		case kvm.CALL, kvm.CALLCODE:
			ot.memOffStack = append(ot.memOffStack, st.Back(5).Uint64())
			ot.memLenStack = append(ot.memLenStack, st.Back(6).Uint64())
		case kvm.CREATE, kvm.CREATE2:
			// Effectively disable memory output
			ot.memOffStack = append(ot.memOffStack, 0)
			ot.memLenStack = append(ot.memLenStack, 0)
		case kvm.SSTORE:
			ot.lastVmOp.Ex.Store = &VmTraceStore{Key: st.Back(0).String(), Val: st.Back(1).String()}
		}
		if ot.lastVmOp.Ex.Used < 0 {
			ot.lastVmOp.Ex = nil
		}
	}
}

func (ot *OeTracer) CaptureFault(env *kvm.KVM, pc uint64, op kvm.OpCode, gas, cost uint64, scope *kvm.ScopeContext, opDepth int, err error) {
}

func (ot *OeTracer) CaptureSelfDestruct(from common.Address, to common.Address, value *big.Int) {
	trace := &ParityTrace{}
	trace.Type = SUICIDE
	action := &SuicideTraceAction{}
	action.Address = from
	action.RefundAddress = to
	action.Balance.ToInt().Set(value)
	trace.Action = action
	topTrace := ot.traceStack[len(ot.traceStack)-1]
	traceIdx := topTrace.Subtraces
	ot.traceAddr = append(ot.traceAddr, traceIdx)
	topTrace.Subtraces++
	trace.TraceAddress = make([]int, len(ot.traceAddr))
	copy(trace.TraceAddress, ot.traceAddr)
	ot.traceAddr = ot.traceAddr[:len(ot.traceAddr)-1]
	ot.r.Trace = append(ot.r.Trace, trace)
}

func (ot *OeTracer) CaptureAccountRead(account common.Address) error {
	return nil
}
func (ot *OeTracer) CaptureAccountWrite(account common.Address) error {
	return nil
}

// Implements core/state/StateWriter to provide state diffs
type StateDiff struct {
	sdMap map[common.Address]*StateDiffAccount
}

func (sd *StateDiff) UpdateAccountData(address common.Address, original, account *accounts.Account) error {
	if _, ok := sd.sdMap[address]; !ok {
		sd.sdMap[address] = &StateDiffAccount{Storage: make(map[common.Hash]map[string]interface{})}
	}
	return nil
}

func (sd *StateDiff) UpdateAccountCode(address common.Address, incarnation uint64, codeHash common.Hash, code []byte) error {
	if _, ok := sd.sdMap[address]; !ok {
		sd.sdMap[address] = &StateDiffAccount{Storage: make(map[common.Hash]map[string]interface{})}
	}
	return nil
}

func (sd *StateDiff) DeleteAccount(address common.Address, original *accounts.Account) error {
	if _, ok := sd.sdMap[address]; !ok {
		sd.sdMap[address] = &StateDiffAccount{Storage: make(map[common.Hash]map[string]interface{})}
	}
	return nil
}

func (sd *StateDiff) WriteAccountStorage(address common.Address, incarnation uint64, key *common.Hash, original, value *uint256.Int) error {
	if *original == *value {
		return nil
	}
	accountDiff := sd.sdMap[address]
	if accountDiff == nil {
		accountDiff = &StateDiffAccount{Storage: make(map[common.Hash]map[string]interface{})}
		sd.sdMap[address] = accountDiff
	}
	m := make(map[string]interface{})
	m["*"] = &StateDiffStorage{From: common.BytesToHash(original.Bytes()), To: common.BytesToHash(value.Bytes())}
	accountDiff.Storage[*key] = m
	return nil
}

func (sd *StateDiff) CreateContract(address common.Address) error {
	if _, ok := sd.sdMap[address]; !ok {
		sd.sdMap[address] = &StateDiffAccount{Storage: make(map[common.Hash]map[string]interface{})}
	}
	return nil
}

// CompareStates uses the addresses accumulated in the sdMap and compares balances, nonces, and codes of the accounts, and fills the rest of the sdMap
func (sd *StateDiff) CompareStates(initialIbs, ibs *state.StateDB) {
	var toRemove []common.Address
	for addr, accountDiff := range sd.sdMap {
		initialExist := initialIbs.Exist(addr)
		exist := ibs.Exist(addr)
		if initialExist {
			if exist {
				var allEqual = len(accountDiff.Storage) == 0
				fromBalance := initialIbs.GetBalance(addr)
				toBalance := ibs.GetBalance(addr)
				if fromBalance.Cmp(toBalance) == 0 {
					accountDiff.Balance = "="
				} else {
					m := make(map[string]*StateDiffBalance)
					m["*"] = &StateDiffBalance{From: (*common.Big)(fromBalance), To: (*common.Big)(toBalance)}
					accountDiff.Balance = m
					allEqual = false
				}
				fromCode := initialIbs.GetCode(addr)
				toCode := ibs.GetCode(addr)
				if bytes.Equal(fromCode, toCode) {
					accountDiff.Code = "="
				} else {
					m := make(map[string]*StateDiffCode)
					m["*"] = &StateDiffCode{From: fromCode, To: toCode}
					accountDiff.Code = m
					allEqual = false
				}
				fromNonce := initialIbs.GetNonce(addr)
				toNonce := ibs.GetNonce(addr)
				if fromNonce == toNonce {
					accountDiff.Nonce = "="
				} else {
					m := make(map[string]*StateDiffNonce)
					m["*"] = &StateDiffNonce{From: common.Uint64(fromNonce), To: common.Uint64(toNonce)}
					accountDiff.Nonce = m
					allEqual = false
				}
				if allEqual {
					toRemove = append(toRemove, addr)
				}
			} else {
				{
					m := make(map[string]*common.Big)
					m["-"] = (*common.Big)(initialIbs.GetBalance(addr))
					accountDiff.Balance = m
				}
				{
					m := make(map[string]common.Bytes)
					m["-"] = initialIbs.GetCode(addr)
					accountDiff.Code = m
				}
				{
					m := make(map[string]common.Uint64)
					m["-"] = common.Uint64(initialIbs.GetNonce(addr))
					accountDiff.Nonce = m
				}
			}
		} else if exist {
			{
				m := make(map[string]*common.Big)
				m["+"] = (*common.Big)(ibs.GetBalance(addr))
				accountDiff.Balance = m
			}
			{
				m := make(map[string]common.Bytes)
				m["+"] = ibs.GetCode(addr)
				accountDiff.Code = m
			}
			{
				m := make(map[string]common.Uint64)
				m["+"] = common.Uint64(ibs.GetNonce(addr))
				accountDiff.Nonce = m
			}
			// Transform storage
			for _, sm := range accountDiff.Storage {
				str := sm["*"].(*StateDiffStorage)
				delete(sm, "*")
				sm["+"] = &str.To
			}
		} else {
			toRemove = append(toRemove, addr)
		}
	}
	for _, addr := range toRemove {
		delete(sd.sdMap, addr)
	}
}

func (api *TraceAPIImpl) ReplayTransaction(ctx context.Context, txHash common.Hash, traceTypes []string) (*TraceCallResult, error) {
	tx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	chainConfig := api.backend.Config()

	blockHeight, ok := api.backend.TxnLookup(ctx, txHash)
	if !ok {
		return nil, nil
	}
	block := api.backend.BlockByHeight(ctx, rpc.BlockHeight(blockHeight))
	if block == nil {
		return nil, err
	}
	var txnIndex uint64
	for i, transaction := range block.Transactions() {
		if transaction.Hash() == txHash {
			txnIndex = uint64(i)
			break
		}
	}

	bn := common.Uint64(blockHeight)

	parentHeight := bn
	if parentHeight > 0 {
		parentHeight -= 1
	}

	// Returns an array of trace arrays, one trace array for each transaction
	traces, err := api.callManyTransactions(ctx, tx, block.Transactions(), traceTypes, block.LastBlockHash(),
		rpc.BlockHeight(parentHeight), block.Header(), int(txnIndex),
		types.MakeSigner(chainConfig, &blockHeight), chainConfig.Rules(new(big.Int).SetUint64(blockHeight)))
	if err != nil {
		return nil, err
	}

	var traceTypeTrace, traceTypeStateDiff, traceTypeVmTrace bool
	for _, traceType := range traceTypes {
		switch traceType {
		case TraceTypeTrace:
			traceTypeTrace = true
		case TraceTypeStateDiff:
			traceTypeStateDiff = true
		case TraceTypeVmTrace:
			traceTypeVmTrace = true
		default:
			return nil, fmt.Errorf("unrecognized trace type: %s", traceType)
		}
	}
	result := &TraceCallResult{}

	for txno, trace := range traces {
		// We're only looking for a specific transaction
		if txno == int(txnIndex) {
			result.Output = trace.Output
			if traceTypeTrace {
				result.Trace = trace.Trace
			}
			if traceTypeStateDiff {
				result.StateDiff = trace.StateDiff
			}
			if traceTypeVmTrace {
				result.VmTrace = trace.VmTrace
			}

			return trace, nil
		}
	}
	return result, nil

}

func (api *TraceAPIImpl) ReplayBlockTransactions(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash, traceTypes []string) ([]*TraceCallResult, error) {
	tx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	chainConfig := api.backend.Config()
	block, err := api.backend.BlockByHeightOrHash(ctx, blockHeightOrHash)
	if err != nil {
		return nil, fmt.Errorf("could not find block %v, error: %s", blockHeightOrHash, err)
	}
	blockHeight := block.Height()
	parentHeight := blockHeight
	if parentHeight > 0 {
		parentHeight -= 1
	}

	var traceTypeTrace, traceTypeStateDiff, traceTypeVmTrace bool
	for _, traceType := range traceTypes {
		switch traceType {
		case TraceTypeTrace:
			traceTypeTrace = true
		case TraceTypeStateDiff:
			traceTypeStateDiff = true
		case TraceTypeVmTrace:
			traceTypeVmTrace = true
		default:
			return nil, fmt.Errorf("unrecognized trace type: %s", traceType)
		}
	}

	// Returns an array of trace arrays, one trace array for each transaction
	traces, err := api.callManyTransactions(ctx, tx, block.Transactions(), traceTypes, block.LastBlockHash(),
		rpc.BlockHeight(parentHeight), block.Header(), -1, /* all tx indices */
		types.MakeSigner(chainConfig, &blockHeight), chainConfig.Rules(new(big.Int).SetUint64(blockHeight)))
	if err != nil {
		return nil, err
	}

	result := make([]*TraceCallResult, len(traces))
	for i, trace := range traces {
		tr := &TraceCallResult{}
		tr.Output = trace.Output
		if traceTypeTrace {
			tr.Trace = trace.Trace
		} else {
			tr.Trace = []*ParityTrace{}
		}
		if traceTypeStateDiff {
			tr.StateDiff = trace.StateDiff
		}
		if traceTypeVmTrace {
			tr.VmTrace = trace.VmTrace
		}
		result[i] = tr
		txhash := block.Transactions()[i].Hash()
		tr.TransactionHash = &txhash
	}

	return result, nil
}

// Call implements trace_call.
func (api *TraceAPIImpl) Call(ctx context.Context, args kaiapi.TransactionArgs, traceTypes []string,
	blockHeightOrHash *rpc.BlockHeightOrHash) (*TraceCallResult, error) {
	tx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	chainConfig := api.backend.Config()

	if blockHeightOrHash == nil {
		var num = rpc.LatestBlockHeight
		blockHeightOrHash = &rpc.BlockHeightOrHash{BlockHeight: &num}
	}

	block, err := api.backend.BlockByHeightOrHash(ctx, *blockHeightOrHash)
	if err != nil {
		return nil, err
	}
	ibs, err := api.backend.StateAtBlock(ctx, block, defaultTraceReexec, nil, true)
	if err != nil {
		return nil, err
	}

	header := block.Header()

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, set up a context with a timeout.
	var cancel context.CancelFunc
	if callTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	traceResult := &TraceCallResult{Trace: []*ParityTrace{}}
	var traceTypeTrace, traceTypeStateDiff, traceTypeVmTrace bool
	for _, traceType := range traceTypes {
		switch traceType {
		case TraceTypeTrace:
			traceTypeTrace = true
		case TraceTypeStateDiff:
			traceTypeStateDiff = true
		case TraceTypeVmTrace:
			traceTypeVmTrace = true
		default:
			return nil, fmt.Errorf("unrecognized trace type: %s", traceType)
		}
	}
	if traceTypeVmTrace {
		traceResult.VmTrace = &VmTrace{Ops: []*VmTraceOp{}}
	}
	var ot OeTracer
	ot.compat = api.compatibility
	if traceTypeTrace || traceTypeVmTrace {
		ot.r = traceResult
		ot.traceAddr = []int{}
	}

	// Get a new instance of the KVM.
	msg := args.ToMessage(api.gasCap)

	blockCtx := blockchain.NewKVMBlockContext(header, api.backend)
	txCtx := blockchain.NewKVMTxContext(msg)
	blockCtx.GasLimit = math.MaxUint64

	vm := kvm.NewKVM(blockCtx, txCtx, ibs, chainConfig, kvm.Config{Debug: traceTypeTrace, Tracer: &noopTracer{}, OETracer: &ot})

	// Wait for the context to be done and cancel the kvm. Even if the
	// KVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		vm.Cancel()
	}()

	gp := new(types.GasPool).AddGas(msg.Gas())
	var execResult *kvm.ExecutionResult
	ibs.Prepare(common.Hash{}, common.Hash{}, 0)
	execResult, err = blockchain.ApplyMessage(vm, msg, gp)
	if err != nil {
		return nil, err
	}
	traceResult.Output = common.CopyBytes(execResult.ReturnData)
	if traceTypeStateDiff {
		sdMap := make(map[common.Address]*StateDiffAccount)
		traceResult.StateDiff = sdMap
		sd := &StateDiff{sdMap: sdMap}
		ibs.Finalise(false)
		// Create initial IntraBlockState, we will compare it with ibs (IntraBlockState after the transaction)
		initialIbs, err := api.backend.StateAtBlock(ctx, block, defaultTraceReexec, nil, true)
		if err != nil {
			return nil, err
		}
		sd.CompareStates(initialIbs, ibs)
	}

	// If the timer caused an abort, return an appropriate error message
	if vm.Cancelled() {
		return nil, fmt.Errorf("execution aborted (timeout = %v)", callTimeout)
	}

	return traceResult, nil
}

// CallMany implements trace_callMany.
func (api *TraceAPIImpl) CallMany(ctx context.Context, calls json.RawMessage, parentHeightOrHash *rpc.BlockHeightOrHash) ([]*TraceCallResult, error) {
	dbtx, err := api.kv.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback()

	var callParams []kaiapi.TransactionArgs
	dec := json.NewDecoder(bytes.NewReader(calls))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('[') {
		return nil, fmt.Errorf("expected array of [callparam, tracetypes]")
	}
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
		if tok != json.Delim('[') {
			return nil, fmt.Errorf("expected [callparam, tracetypes]")
		}
		callParams = append(callParams, kaiapi.TransactionArgs{})
		args := &callParams[len(callParams)-1]
		if err = dec.Decode(args); err != nil {
			return nil, err
		}
		if err = dec.Decode(&args.TraceTypes); err != nil {
			return nil, err
		}
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
		if tok != json.Delim(']') {
			return nil, fmt.Errorf("expected end of [callparam, tracetypes]")
		}
	}
	tok, err = dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array of [callparam, tracetypes]")
	}

	if parentHeightOrHash == nil {
		var num = rpc.LatestBlockHeight
		parentHeightOrHash = &rpc.BlockHeightOrHash{BlockHeight: &num}
	}

	msgs := make([]types.Message, len(callParams))
	for i, args := range callParams {
		msgs[i] = args.ToMessage(api.gasCap)
	}
	return api.doCallMany(ctx, dbtx, msgs, callParams, parentHeightOrHash, nil, -1 /* all tx indices */)
}

func (api *TraceAPIImpl) doCallMany(ctx context.Context, dbtx kvstore.Tx, msgs []types.Message, callParams []kaiapi.TransactionArgs,
	parentHeightOrHash *rpc.BlockHeightOrHash, header *types.Header, txIndexNeeded int) ([]*TraceCallResult, error) {
	chainConfig := api.backend.Config()

	if parentHeightOrHash == nil {
		var num = rpc.LatestBlockHeight
		parentHeightOrHash = &rpc.BlockHeightOrHash{BlockHeight: &num}
	}
	parentBlock, err := api.backend.BlockByHeightOrHash(ctx, *parentHeightOrHash)
	if err != nil {
		return nil, err
	}

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, set up a context with a timeout.
	var cancel context.CancelFunc
	if callTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()
	results := []*TraceCallResult{}

	useParent := false
	if header == nil {
		header = parentBlock.Header()
		useParent = true
	}

	for txIndex, msg := range msgs {
		if err := common.Stopped(ctx.Done()); err != nil {
			return nil, err
		}
		traceResult := &TraceCallResult{Trace: []*ParityTrace{}}
		var traceTypeTrace, traceTypeStateDiff, traceTypeVmTrace bool
		args := callParams[txIndex]
		for _, traceType := range args.TraceTypes {
			switch traceType {
			case TraceTypeTrace:
				traceTypeTrace = true
			case TraceTypeStateDiff:
				traceTypeStateDiff = true
			case TraceTypeVmTrace:
				traceTypeVmTrace = true
			default:
				return nil, fmt.Errorf("unrecognized trace type: %s", traceType)
			}
		}
		vmConfig := kvm.Config{}
		if (traceTypeTrace && (txIndexNeeded == -1 || txIndex == txIndexNeeded)) || traceTypeVmTrace {
			var ot OeTracer
			ot.compat = api.compatibility
			ot.r = traceResult
			ot.idx = []string{fmt.Sprintf("%d-", txIndex)}
			if traceTypeTrace && (txIndexNeeded == -1 || txIndex == txIndexNeeded) {
				ot.traceAddr = []int{}
			}
			if traceTypeVmTrace {
				traceResult.VmTrace = &VmTrace{Ops: []*VmTraceOp{}}
			}
			vmConfig.Debug = true
			vmConfig.OETracer = &ot
			vmConfig.Tracer = &noopTracer{}
		}

		// Get a new instance of the kvm.
		blockCtx := blockchain.NewKVMBlockContext(header, api.backend)
		txCtx := blockchain.NewKVMTxContext(msg)
		blockCtx.GasLimit = math.MaxUint64
		if useParent {
			blockCtx.GasLimit = math.MaxUint64
		}
		ibs, err := api.backend.StateAtBlock(ctx, parentBlock, defaultTraceReexec, nil, true)
		if err != nil {
			return nil, err
		}
		// Create initial IntraBlockState, we will compare it with ibs (IntraBlockState after the transaction)

		evm := kvm.NewKVM(blockCtx, txCtx, ibs, chainConfig, vmConfig)

		gp := new(types.GasPool).AddGas(msg.Gas())
		var execResult *kvm.ExecutionResult
		if args.TxHash != nil {
			ibs.Prepare(*args.TxHash, header.Hash(), txIndex)
		} else {
			ibs.Prepare(common.Hash{}, header.Hash(), txIndex)
		}
		execResult, err = blockchain.ApplyMessage(evm, msg, gp)
		if err != nil {
			return nil, fmt.Errorf("first run for txIndex %d error: %w", txIndex, err)
		}
		traceResult.Output = common.CopyBytes(execResult.ReturnData)
		if traceTypeStateDiff {
			initialIbs, err := api.backend.StateAtBlock(ctx, parentBlock, defaultTraceReexec, nil, true)
			if err != nil {
				return nil, err
			}
			sdMap := make(map[common.Address]*StateDiffAccount)
			traceResult.StateDiff = sdMap
			sd := &StateDiff{sdMap: sdMap}
			ibs.Finalise(false)
			sd.CompareStates(initialIbs, ibs)
			if _, err = ibs.Commit(false); err != nil {
				return nil, err
			}
		} else {
			ibs.Finalise(false)
			if _, err = ibs.Commit(false); err != nil {
				return nil, err
			}
		}
		if !traceTypeTrace {
			traceResult.Trace = []*ParityTrace{}
		}
		results = append(results, traceResult)
	}
	return results, nil
}

// RawTransaction implements trace_rawTransaction.
func (api *TraceAPIImpl) RawTransaction(ctx context.Context, txHash common.Hash, traceTypes []string) ([]interface{}, error) {
	var stub []interface{}
	return stub, fmt.Errorf(NotImplemented, "trace_rawTransaction")
}
