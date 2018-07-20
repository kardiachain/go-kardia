package vm

import (
	"math/big"

	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/params"
	"github.com/kardiachain/go-kardia/types"
)

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH KVM op code.
	GetHashFunc func(uint64) common.Hash
)

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

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// virtual machine configuration options used to initialise the
	// evm.
	vmConfig Config
	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *Interpreter
}

// NewKVM returns a new KVM. The returned KVM is not thread safe and should
// only ever be used *once*.
func NewKVM(ctx Context, statedb StateDB, chainConfig *params.ChainConfig, vmConfig Config) *KVM {
	kvm := &KVM{
		Context:     ctx,
		StateDB:     statedb,
		vmConfig:    vmConfig,
		chainConfig: chainConfig,
	}
	kvm.interpreter = NewInterpreter(kvm, vmConfig)

	return kvm
}

// ChainConfig returns the environment's chain configuration
func (kvm *KVM) ChainConfig() *params.ChainConfig { return kvm.chainConfig }

//================================================================================================
// Interfaces
//=================================================================================================

// StateDB is an KVM database for full state querying.
type StateDB interface {
	GetBalance(common.Address) *big.Int

	GetCode(common.Address) []byte

	GetCodeSize(common.Address) int

	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)

	AddLog(*types.Log)

	AddRefund(uint64)
}
