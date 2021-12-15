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
	"math/big"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
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
	return nil, nil, 0, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func applyTransaction(msg types.Message, config *configs.ChainConfig, logger log.Logger, bc vm.ChainContext, gp *types.GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg kvm.Config, evm *kvm.EVM) (*types.Receipt, uint64, error) {
	// Create a new context to be used in the EVM environment.
	txContext := vm.NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	logger.Trace("Apply transaction", "hash", tx.Hash().Hex(), "nonce", msg.Nonce(), "from", msg.From().Hex())
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	// Update the state with pending changes
	statedb.Finalise(true)
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx,
	// we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(result.Failed(), *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, result.UsedGas, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *configs.ChainConfig, loggger log.Logger, bc vm.ChainContext, gp *types.GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg kvm.Config) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, &header.Height))
	if err != nil {
		return nil, 0, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := vm.NewEVMBlockContext(header, bc, nil)
	vmenv := kvm.NewEVM(blockContext, kvm.TxContext{}, statedb, config, cfg)
	return applyTransaction(msg, config, loggger, bc, gp, statedb, header, tx, usedGas, cfg, vmenv)
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
	vm         *kvm.EVM
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
func NewStateTransition(kvm *kvm.EVM, msg Message, gp *types.GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		vm:       kvm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    kvm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any KVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(kvm *kvm.EVM, msg Message, gp *types.GasPool) (*kvm.ExecutionResult, error) {
	return NewStateTransition(kvm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(st.msg.From()).Cmp(mgval) < 0 {
		return tx_pool.ErrInsufficientFunds
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
		if nonce < st.msg.Nonce() {
			return tx_pool.ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return tx_pool.ErrNonceTooLow
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

	// Check clauses 1-3, buy gas if everything is correct
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
	if msg.Value().Sign() > 0 && !st.vm.Context.CanTransfer(st.state, msg.From(), msg.Value()) {
		return nil, tx_pool.ErrInsufficientFundsForTransfer
	}

	var (
		ret   []byte
		vmerr error
	)
	if contractCreation {
		ret, _, st.gas, vmerr = st.vm.Create(sender, st.data, st.gas, st.value)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = st.vm.Call(sender, st.to(), st.data, st.gas, st.value)
	}

	st.refundGas()
	st.state.AddBalance(st.vm.Context.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	return &kvm.ExecutionResult{
		UsedGas:    st.gasUsed(),
		Err:        vmerr,
		ReturnData: ret,
	}, nil
}

func (st *StateTransition) refundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

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
