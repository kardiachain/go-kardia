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
	"fmt"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/log"
	"math/big"

	"sync/atomic"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
)

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(base.StateDB, common.Address, *big.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(base.StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH KVM op code.
	GetHashFunc func(uint64) common.Hash
)

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func run(kvm *KVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	if contract.CodeAddr != nil {
		precompiles := PrecompiledContractsV0
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract, kvm.Context, kvm.StateDB)
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
	Chain  		base.BaseBlockChain
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
	StateDB base.StateDB
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
func NewKVM(ctx Context, statedb base.StateDB, vmConfig Config) *KVM {
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

func (kvm *KVM) IsZeroFee() bool {
	return kvm.vmConfig.IsZeroFee
}

func (kvm *KVM) GetCoinbase() common.Address {
	return kvm.Coinbase
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (kvm *KVM) Call(caller base.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !kvm.Context.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = kvm.GetStateDB().Snapshot()
	)
	if !kvm.GetStateDB().Exist(addr) {
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
		kvm.GetStateDB().CreateAccount(addr)
	}
	kvm.Transfer(kvm.StateDB, caller.Address(), to.Address(), value)

	// Initialise a new contract and set the code that is to be used by the KVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, kvm.GetStateDB().GetCodeHash(addr), kvm.GetStateDB().GetCode(addr))

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
	ret, err = run(kvm, contract, input, false)

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		kvm.GetStateDB().RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
		log.Error(fmt.Sprintf("%v: %v", err.Error(), string(ret)))
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
func (kvm *KVM) CallCode(caller base.ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !kvm.CanTransfer(kvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		snapshot = kvm.GetStateDB().Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, kvm.GetStateDB().GetCodeHash(addr), kvm.GetStateDB().GetCode(addr))

	ret, err = run(kvm, contract, input, false)
	if err != nil {
		kvm.GetStateDB().RevertToSnapshot(snapshot)
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
func (kvm *KVM) DelegateCall(caller base.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = kvm.GetStateDB().Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, kvm.GetStateDB().GetCodeHash(addr), kvm.GetStateDB().GetCode(addr))

	ret, err = run(kvm, contract, input, false)
	if err != nil {
		kvm.GetStateDB().RevertToSnapshot(snapshot)
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
func (kvm *KVM) StaticCall(caller base.ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if kvm.depth > int(CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		to       = AccountRef(addr)
		snapshot = kvm.GetStateDB().Snapshot()
	)
	// Initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, kvm.GetStateDB().GetCodeHash(addr), kvm.GetStateDB().GetCode(addr))

	// We do an AddBalance of zero here, just in order to trigger a touch.
	// This doesn't matter on Mainnet, where all empties are gone at the time of Byzantium,
	// but is the correct thing to do and matters on other networks, in tests, and potential
	// future scenarios
	kvm.GetStateDB().AddBalance(addr, bigZero)

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	ret, err = run(kvm, contract, input, true)
	if err != nil {
		kvm.GetStateDB().RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
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

func (kvm *KVM) createContract (contract *Contract, codeAndHash *codeAndHash) (ret []byte, err error) {
	contractAddress := contract.Address()
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if kvm.depth > int(CallCreateDepth) {
		return nil, ErrDepth
	}
	if !kvm.CanTransfer(kvm.StateDB, contract.caller.Address(), contract.Value()) {
		return nil, ErrInsufficientBalance
	}

	nonce := kvm.StateDB.GetNonce(contract.caller.Address())
	kvm.StateDB.SetNonce(contract.caller.Address(), nonce+1)

	// Ensure there's no existing contract already at the designated address
	contractHash := kvm.StateDB.GetCodeHash(contract.Address())
	if kvm.StateDB.GetNonce(contractAddress) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, ErrContractAddressCollision
	}

	kvm.StateDB.CreateAccount(contractAddress)
	kvm.StateDB.SetNonce(contractAddress, 1)
	kvm.StateDB.SetCode(contractAddress, codeAndHash.code)

	kvm.Transfer(kvm.StateDB, contract.caller.Address(), contractAddress, contract.Value())

	// initialise a new contract and set the code that is to be used by the
	// KVM. The contract is a scoped environment for this execution context
	// only.
	contract.SetCodeOptionalHash(&contractAddress, codeAndHash)

	if kvm.vmConfig.NoRecursion && kvm.depth > 0 {
		return nil, fmt.Errorf("depth is not allowed when no recursion is enabled")
	}

	/* TODO(huny@): Adding tracer later
	if kvm.vmConfig.Debug && kvm.depth == 0 {
		kvm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, code, gas, value)
	}

	start := time.Now()
	*/
	ret, err = run(kvm, contract, nil, false)
	if err != nil {
		return nil, err
	}

	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.

	createDataGas := uint64(len(ret)) * CreateDataGas
	if contract.UseGas(createDataGas) {
		kvm.GetStateDB().SetCode(contractAddress, ret)
	} else {
		return ret, ErrCodeStoreOutOfGas
	}
	return ret, nil
}

// Create creates a new contract using code as deployment code.
func (kvm *KVM) create(caller base.ContractRef, codeAndHash *codeAndHash, gas uint64, value *big.Int, address common.Address) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	snapshot := kvm.GetStateDB().Snapshot()
	contract := NewContract(caller, AccountRef(address), value, gas)
	ret, err = kvm.createContract(contract, codeAndHash)
	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > MaxCodeSize

	// When an error was returned by the KVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining.
	if err != nil || maxCodeSizeExceeded {
		kvm.GetStateDB().RevertToSnapshot(snapshot)
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
	return ret, address, contract.Gas, err
}

func (kvm *KVM) GetStateDB() base.StateDB {
	return kvm.StateDB
}

// Create creates a new contract using code as deployment code.
func (kvm *KVM) Create(caller base.ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	contractAddr = crypto.CreateAddress(caller.Address(), kvm.GetStateDB().GetNonce(caller.Address()))
	return kvm.create(caller, &codeAndHash{code: code}, gas, value, contractAddr)
}

// CreateGenesisContract creates contractAddr with given contractAddr
// Note: this function is only used when creating genesis contract
func (kvm *KVM) CreateGenesisContract(caller base.ContractRef, contractAddr *common.Address, code []byte, gas uint64, value *big.Int) (ret []byte, newContractAddr common.Address, leftOverGas uint64, err error) {
	if contractAddr == nil {
		address := crypto.CreateAddress(caller.Address(), kvm.GetStateDB().GetNonce(caller.Address()))
		contractAddr = &address
	}

	contract := NewContract(caller, AccountRef(*contractAddr), big.NewInt(0), gas)
	ret, err = kvm.createContract(contract, &codeAndHash{code: code})
	if err != nil {
		retStr := string(ret)
		log.Error("err", "ret", retStr, "err", err)
		return ret, *contractAddr, contract.Gas, err
	}
	kvm.StateDB.AddBalance(*contractAddr, value)
	return ret, *contractAddr, leftOverGas, nil
}

// NewKVMContext creates a new context for dual node to call smc in the KVM.
func NewInternalKVMContext(from common.Address, header *types.Header, chain base.BaseBlockChain) Context {
	return Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Origin:      from,
		Coinbase:    header.Coinbase,
		BlockHeight: new(big.Int).SetUint64(header.Height),
		Time:        new(big.Int).Set(header.Time),
		GasLimit:    header.GasLimit,
		GasPrice:    big.NewInt(1),
		Chain: chain,
	}
}

func NewGenesisKVMContext(from common.Address, gasLimit uint64) Context {
	return Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GenesisGetHashFn(),
		Origin:      from,
		Coinbase:    common.Address{},
		BlockHeight: big.NewInt(0),
		Time:        big.NewInt(0),
		GasLimit:    gasLimit,
		GasPrice:    big.NewInt(1),
	}
}

func GenesisGetHashFn() func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		return common.Hash{}
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by height
func GetHashFn(ref *types.Header, chain base.BaseBlockChain) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.Height - 1: ref.LastCommitHash,
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.LastCommitHash, ref.Height-1); header != nil; header = chain.GetHeader(header.LastCommitHash, header.Height-1) {
			cache[header.Height-1] = header.LastCommitHash
			if n == header.Height-1 {
				return header.LastCommitHash
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db base.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db base.StateDB, sender, recipient common.Address, amount *big.Int) {
	//log.Error("transferring", "sender", sender.Hex(), "receiver", recipient.Hex(), "amount", amount.String())
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

/**
	Internal contract execution
 */
const maximumGasUsed = uint64(7000000)

func newInternalKVM(from common.Address, chain base.BaseBlockChain, statedb base.StateDB) *KVM {
	ctx := NewInternalKVMContext(from, chain.CurrentHeader(), chain)
	return NewKVM(ctx, statedb, Config{})
}

// staticCall calls smc and return result in bytes format
func StaticCall(vm *KVM, to common.Address, input []byte) (result []byte, err error) {
	sender := AccountRef(vm.Context.Origin)
	result, _, err = vm.StaticCall(sender, to, input, maximumGasUsed)
	return result, err
}

func InternalCall(vm *KVM, to common.Address, input []byte, value *big.Int) (result []byte, err error) {
	sender := AccountRef(vm.Context.Origin)
	result, _, err = vm.Call(sender, to, input, maximumGasUsed, value)
	return result, err
}

func InternalCreate(vm *KVM, to common.Address, input []byte, value *big.Int) (result []byte, address common.Address, leftOverGas uint64, err error) {
	sender := AccountRef(vm.Context.Origin)
	return vm.CreateGenesisContract(sender, &to, input, maximumGasUsed, value)
}

// EstimateGas estimates spent in order to
func EstimateGas(vm *KVM, to common.Address, input []byte) (uint64, error){
	// Create new call message
	msg := types.NewMessage(vm.Origin, &to, 0, big.NewInt(0), maximumGasUsed, big.NewInt(1), input, false)
	// Apply the transaction to the current state (included in the env)
	gp := new(types.GasPool).AddGas(common.MaxUint64)
	_, gas, _, err := vm.Context.Chain.ApplyMessage(vm, msg, gp)
	if err != nil {
		return 0, err
	}
	// If the timer caused an abort, return an appropriate error message
	if vm.Cancelled() {
		return 0, fmt.Errorf("execution aborted")
	}
	return gas, nil // need to add some bufferGas to prevent out of gas
}
