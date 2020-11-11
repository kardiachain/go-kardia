/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-ethereum library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package kvm

import (
	"math/big"
	"time"

	"sync/atomic"

	"github.com/holiman/uint256"
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/types"
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
func run(kvm *KVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	if contract.CodeAddr != nil {
		precompiles := PrecompiledContractsV0
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract)
		}
	}
	return kvm.interpreter.Run(contract, input, readOnly)
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
	BlockHeight *big.Int       // Provides information for HEIGHT
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

// Cancel cancels any running KVM operation. This may be called concurrently and
// it's safe to be called multiple times.
func (kvm *KVM) Cancel() {
	atomic.StoreInt32(&kvm.abort, 1)
}

// Cancelled returns true if Cancel has been called
func (kvm *KVM) Cancelled() bool {
	return atomic.LoadInt32(&kvm.abort) == 1
}

// GetVmConfig returns kvm's config
func (kvm *KVM) GetVmConfig() Config {
	return kvm.vmConfig
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
	if kvm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if value.Sign() != 0 && !kvm.Context.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = kvm.StateDB.Snapshot()
	)
	if !kvm.StateDB.Exist(addr) {
		precompiles := PrecompiledContractsV0
		if precompiles[addr] == nil && value.Sign() == 0 {
			// Calling a non existing account, don't do antything, but ping the tracer
			if kvm.vmConfig.Debug && kvm.depth == 0 {
				kvm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				kvm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			return nil, gas, nil
		}
		kvm.StateDB.CreateAccount(addr)
	}
	kvm.Transfer(kvm.StateDB, caller.Address(), to.Address(), value)

	// Capture the tracer start/end events in debug mode
	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
		defer func(startGas uint64, startTime time.Time) { // Lazy evaluation of the parameters
			kvm.vmConfig.Tracer.CaptureEnd(ret, startGas-gas, time.Since(startTime), err)
		}(gas, time.Now())
	}

	// Initialise a new contract and set the code that is to be used by the KVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	ret, err = run(kvm, contract, input, false)

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			gas = 0
		}
	}
	return ret, gas, err
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
	if kvm.depth > int(configs.CallCreateDepth) {
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

	ret, err = run(kvm, contract, input, false)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			gas = 0
		}
	}
	return ret, gas, err
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
	if kvm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = kvm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, kvm.StateDB.GetCodeHash(addr), kvm.StateDB.GetCode(addr))

	ret, err = run(kvm, contract, input, false)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			gas = 0
		}
	}
	return ret, gas, err
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
	if kvm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
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

	// We do an AddBalance of zero here, just in order to trigger a touch.
	// This doesn't matter on Mainnet, where all empties are gone at the time of Byzantium,
	// but is the correct thing to do and matters on other networks, in tests, and potential
	// future scenarios
	kvm.StateDB.AddBalance(addr, big0)

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	ret, err = run(kvm, contract, input, true)
	if err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			gas = 0
		}
	}
	return ret, gas, err
}

type codeAndHash struct {
	code []byte
	hash common.Hash
}

func (c *codeAndHash) Hash() common.Hash {
	if c.hash == (common.Hash{}) {
		c.hash = crypto.Keccak256Hash(c.code)
	}
	return c.hash
}

// Create creates a new contract using code as deployment code.
func (kvm *KVM) create(caller ContractRef, codeAndHash *codeAndHash, gas uint64, value *big.Int, address common.Address) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if kvm.depth > int(configs.CallCreateDepth) {
		return nil, common.Address{}, gas, ErrDepth
	}
	if !kvm.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, common.Address{}, gas, ErrInsufficientBalance
	}

	nonce := kvm.StateDB.GetNonce(caller.Address())
	kvm.StateDB.SetNonce(caller.Address(), nonce+1)

	// Ensure there's no existing contract already at the designated address
	contractHash := kvm.StateDB.GetCodeHash(address)
	if kvm.StateDB.GetNonce(address) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, common.Address{}, 0, ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := kvm.StateDB.Snapshot()
	kvm.StateDB.CreateAccount(address)
	kvm.StateDB.SetNonce(address, 1)

	kvm.Transfer(kvm.StateDB, caller.Address(), address, value)

	// initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(address), value, gas)
	contract.SetCodeOptionalHash(&address, codeAndHash)

	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, address, gas, nil
	}

	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, codeAndHash.code, gas, value)
	}
	start := time.Now()

	ret, err = run(kvm, contract, nil, false)

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > configs.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * configs.CreateDataGas
		if contract.UseGas(createDataGas) {
			kvm.StateDB.SetCode(address, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	if maxCodeSizeExceeded || err != nil {
		kvm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = ErrMaxCodeSizeExceeded
	}

	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	return ret, address, contract.Gas, err
}

// Create creates a new contract using code as deployment code.
func (kvm *KVM) Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	contractAddr = crypto.CreateAddress(caller.Address(), kvm.StateDB.GetNonce(caller.Address()))
	return kvm.create(caller, &codeAndHash{code: code}, gas, value, contractAddr)
}

// Create2 creates a new contract using code as deployment code.
//
// The different between Create2 with Create is Create2 uses sha3(msg.sender ++ salt ++ init_code)[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (kvm *KVM) Create2(caller ContractRef, code []byte, gas uint64, endowment *big.Int, salt *uint256.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	codeAndHash := &codeAndHash{code: code}
	contractAddr = crypto.CreateAddress2(caller.Address(), common.Hash(salt.Bytes32()), code)
	return kvm.create(caller, codeAndHash, gas, endowment, contractAddr)
}

// CreateGenesisContractAddress creates a new contract using Genesis Deployer address.
func (kvm *KVM) CreateGenesisContractAddress(caller ContractRef, code []byte, gas uint64, value *big.Int, genesisContractAddr common.Address) (err error) {
	_, _, _, vmerr := kvm.create(caller, &codeAndHash{code: code}, gas, value, genesisContractAddr)
	if vmerr != nil {
		return vmerr
	}
	return nil
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

	AddRefund(uint64)
	SubRefund(uint64)
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
	AddPreimage(common.Hash, []byte)
}
