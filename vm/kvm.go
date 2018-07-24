package vm

import (
	"math/big"

	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/crypto"
	"github.com/kardiachain/go-kardia/params"
	"github.com/kardiachain/go-kardia/types"
)

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH KVM op code.
	GetHashFunc func(uint64) common.Hash
)

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func run(kvm *KVM, contract *Contract, input []byte) ([]byte, error) {
	if contract.CodeAddr != nil {
		precompiles := PrecompiledContractsV0
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract)
		}
	}
	return kvm.interpreter.Run(contract, input)
}

// Context provides the KVM with auxiliary information. Once provided
// it shouldn't be modified.
type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient kai to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers kai from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Message information
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockHeight uint64         // Provides information for HEIGHT
	Time        *big.Int       // Provides information for TIME
}

// KVM is the Kardia Virtual Machine base object and provides
// the necessary tools to run a contract on the given state with
// the provided context. It should be noted that any error
// generated through any of the calls should be considered a
// revert-state-and-consume-all-gas operation, no checks on
// specific errors should ever be performed. The interpreter makes
// sure that any errors generated are to be considered faulty code.
//
// The KVM should never be reused and is not thread safe.
type KVM struct {
	// Context provides auxiliary blockchain related information
	Context
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int

	// virtual machine configuration options used to initialise the
	// kvm.
	vmConfig Config
	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *Interpreter
	// abort is used to abort the EVM calling operations
	// NOTE: must be set atomically
	abort int32
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64
}

// NewKVM returns a new KVM. The returned KVM is not thread safe and should
// only ever be used *once*.
func NewKVM(ctx Context, statedb StateDB, vmConfig Config) *KVM {
	kvm := &KVM{
		Context:  ctx,
		StateDB:  statedb,
		vmConfig: vmConfig,
	}
	kvm.interpreter = NewInterpreter(kvm, vmConfig)

	return kvm
}

// Create creates a new contract using code as deployment code.
func (kvm *KVM) Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if kvm.depth > int(params.CallCreateDepth) {
		return nil, common.Address{}, gas, ErrDepth
	}
	if !kvm.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, common.Address{}, gas, ErrInsufficientBalance
	}
	// Ensure there's no existing contract already at the designated address
	nonce := kvm.StateDB.GetNonce(caller.Address())
	kvm.StateDB.SetNonce(caller.Address(), nonce+1)

	contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	contractHash := kvm.StateDB.GetCodeHash(contractAddr)
	if kvm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, common.Address{}, 0, ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := kvm.StateDB.Snapshot()
	kvm.StateDB.CreateAccount(contractAddr)
	kvm.StateDB.SetNonce(contractAddr, 1)

	kvm.Transfer(kvm.StateDB, caller.Address(), contractAddr, value)

	// initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)

	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, contractAddr, gas, nil
	}

	/* TODO(huny@): Adding tracer later
	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, code, gas, value)
	}

	start := time.Now()
	*/
	ret, err = run(kvm, contract, nil)

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > params.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * params.CreateDataGas
		if contract.UseGas(createDataGas) {
			kvm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	if maxCodeSizeExceeded || err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = errMaxCodeSizeExceeded
	}
	/* TODO(huny@): Add tracer later
	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	*/
	return ret, contractAddr, contract.Gas, err
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (kvm *KVM) Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !kvm.Context.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = kvm.StateDB.Snapshot()
	)
	if !kvm.StateDB.Exist(addr) {
		precompiles := PrecompiledContractsV0
		if precompiles[addr] == nil && value.Sign() == 0 {
			/* TODO(huny@): Add tracer later
			// Calling a non existing account, don't do antything, but ping the tracer
			if kvm.vmConfig.Debug && kvm.depth == 0 {
				kvm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				kvm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			*/
			return nil, gas, nil
		}
		kvm.StateDB.CreateAccount(addr)
	}
	kvm.Transfer(kvm.StateDB, caller.Address(), to.Address(), value)

	// Initialise a new contract and set the code that is to be used by the KVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	/* TODO(huny@): Add tracer later
	start := time.Now()

	// Capture the tracer start/end events in debug mode
	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)

		defer func() { // Lazy evaluation of the parameters
			kvm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
		}()
	}
	*/
	ret, err = run(kvm, contract, input)

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (kvm *KVM) CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !kvm.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		snapshot = kvm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	ret, err = run(kvm, contract, input)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (kvm *KVM) DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = kvm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	ret, err = run(kvm, contract, input)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (kvm *KVM) StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Make sure the readonly is only set if we aren't in readonly yet
	// this makes also sure that the readonly flag isn't removed for
	// child calls.
	if !kvm.interpreter.readOnly {
		kvm.interpreter.readOnly = true
		defer func() { kvm.interpreter.readOnly = false }()
	}

	var (
		to       = AccountRef(addr)
		snapshot = kvm.StateDB.Snapshot()
	)
	// Initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	ret, err = run(kvm, contract, input)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

//================================================================================================
// Interfaces
//=================================================================================================

// StateDB is an KVM database for full state querying.
type StateDB interface {
	CreateAccount(common.Address)

	AddBalance(common.Address, *big.Int)
	SubBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int

	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	GetRefund() uint64

	Suicide(common.Address) bool
	HasSuicided(common.Address) bool

	RevertToSnapshot(int)
	Snapshot() int

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(common.Address) bool

	// Empty returns whether the given account is empty. Empty
	// is defined as (balance = nonce = code = 0).
	Empty(common.Address) bool

	AddLog(*types.Log)

	AddRefund(uint64)
}
