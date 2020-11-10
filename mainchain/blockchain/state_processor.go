/*
 *  Copyright 2018 KardiaChain
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

package blockchain

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	// ErrGasLimitReached is returned by the gas pool if the amount of gas required
	// by a transaction is higher than what's left in the block.
	ErrGasLimitReached = errors.New("gas limit reached")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")

	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	logger log.Logger
	bc     *BlockChain // Canonical block chain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(logger log.Logger, bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		logger: logger,
		bc:     bc,
	}
}

// Process processes the state changes according to the Kardia rules by running
// the transaction messages using the statedb.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg kvm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(types.GasPool).AddGas(block.GasLimit())
	)
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.logger, p.bc, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	return receipts, allLogs, *usedGas, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(logger log.Logger, bc vm.ChainContext, gp *types.GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg kvm.Config) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(types.HomesteadSigner{})
	if err != nil {
		return nil, 0, err
	}
	logger.Trace("Apply transaction", "hash", tx.Hash().Hex(), "nonce", msg.Nonce(), "from", msg.From().Hex())
	// Create a new context to be used in the KVM environment
	context := vm.NewKVMContext(msg, header, bc)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := kvm.NewKVM(context, statedb, cfg)
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	statedb.Finalise(true)
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx,
	// we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, result.Failed(), *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, result.UsedGas, result.Err
}

/*
The State Transitioning Model
A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all all the necessary work to work out a valid new state root.
1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *types.GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      kvm.StateDB
	vm         *kvm.KVM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(vm *kvm.KVM, msg Message, gp *types.GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		vm:       vm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    vm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any KVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(vm *kvm.KVM, msg Message, gp *types.GasPool) (*kvm.ExecutionResult, error) {
	return NewStateTransition(vm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return kvm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(st.msg.From()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(st.msg.From(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.From())
		// FIXME(kiendn): nonce does not need to check too high, it can be depended on what user input
		// as long as nonce is not less than or equals current state then it wil be fined.

		//if nonce < st.msg.Nonce() {
		//	return ErrNonceTooHigh
		//} else
		if nonce > st.msg.Nonce() {
			//return tx_pool.ErrNonceTooLow
			return fmt.Errorf("nonce too low - current nonce is %v sender %v sender's nonce %v", nonce, st.msg.From().Hex(), st.msg.Nonce())
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (*kvm.ExecutionResult, error) {
	// First check this message satisfies all consensus rules before
	// applying the message. The rules include these clauses
	//
	// 1. the nonce of the message caller is correct
	// 2. caller has enough balance to cover transaction fee(gaslimit * gasprice)
	// 3. the amount of gas required is available in the block
	// 4. the purchased gas is enough to cover intrinsic usage
	// 5. there is no overflow when calculating intrinsic gas
	// 6. caller has enough balance to cover asset transfer for **topmost** call

	if err := st.preCheck(); err != nil {
		return nil, err
	}
	msg := st.msg
	sender := kvm.AccountRef(msg.From())
	contractCreation := msg.To() == nil

	// Check clauses 4-5, subtract intrinsic gas if everything is correct
	gas, err := tx_pool.IntrinsicGas(st.data, contractCreation)
	if err != nil {
		return nil, err
	}
	if st.gas < gas {
		return nil, tx_pool.ErrIntrinsicGas
	}
	st.gas -= gas

	// Check clause 6
	if msg.Value().Sign() > 0 && !st.vm.CanTransfer(st.state, msg.From(), msg.Value()) {
		return nil, tx_pool.ErrInsufficientFundsForTransfer
	}

	var (
		vmerr error
		ret   []byte
	)
	if contractCreation {
		ret, _, st.gas, vmerr = st.vm.Create(sender, st.data, st.gas, st.value)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = st.vm.Call(sender, st.to(), st.data, st.gas, st.value)
	}

	// For logging purpose
	if vmerr != nil {
		log.Error("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == kvm.ErrInsufficientBalance {
			return nil, vmerr
		}
	}

	// If IsZeroFee is true then refund all gas that sender spend in current transaction
	if st.vm.GetVmConfig().IsZeroFee {
		st.refundGas(true) // refundAll
	} else {
		st.refundGas(false) // !refundAll
		st.state.AddBalance(st.vm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))
	}

	return &kvm.ExecutionResult{
		UsedGas:    st.gasUsed(),
		Err:        vmerr,
		ReturnData: ret,
	}, nil
}

func (st *StateTransition) refundGas(refundAll bool) {
	if refundAll {
		st.gas = st.initialGas
	} else {
		// Apply refund counter, capped to half of the used gas.
		refund := st.gasUsed() / 2
		if refund > st.state.GetRefund() {
			refund = st.state.GetRefund()
		}
		st.gas += refund
	}
	// Return KAI for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(st.msg.From(), remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
