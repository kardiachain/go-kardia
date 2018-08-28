package blockchain

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"
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
	bc *BlockChain // Canonical block chain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		bc: bc,
	}
}

// Process processes the state changes according to the Kardia rules by running
// the transaction messages using the statedb.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	return receipts, allLogs, *usedGas, nil
}

func ApplyTransactionsToAccountState(txs []*types.Transaction, accounts types.AccountStates) (types.AccountStates, error) {
	// converts to map.
	accountMap := make(map[common.Address]*big.Int)
	for _, account := range accounts {
		accountMap[*account.Addr] = account.Balance
	}
	// applying txs.
	for _, tx := range txs {
		from, err := types.Sender(tx)
		if err != nil {
			return nil, err
		}
		to := tx.To()

		fromBalance, found := accountMap[from]
		if !found {
			return nil, fmt.Errorf("sender account %v is not found when applying transaction", from)
		}
		toBalance, found := accountMap[*to]
		if !found {
			//return nil, fmt.Errorf("receiver account %v is not found when applying transaction", *to)
			balance := big.NewInt(0)
			accounts = append(accounts, &types.BlockAccount{Addr: to, Balance: balance})
			accountMap[*to] = balance.Add(balance, tx.Value())
		}

		fromBalance.Sub(fromBalance, tx.Value())
		toBalance.Add(toBalance, tx.Value())
	}

	newState := make(types.AccountStates, len(accounts))

	for i, oldAccount := range accounts {
		newBalance := accountMap[*oldAccount.Addr]
		newState[i] = &types.BlockAccount{Addr: oldAccount.Addr, Balance: newBalance}
	}

	return newState, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage()
	if err != nil {
		return nil, 0, err
	}
	// Create a new context to be used in the KVM environment
	context := NewKVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewKVM(context, statedb, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	// Update the state with pending changes
	root := statedb.IntermediateRoot(true).Bytes()
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx,
	// we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
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
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	kvm        *vm.KVM
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

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = configs.TxGasContractCreation
	} else {
		gas = configs.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/configs.TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * configs.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/configs.TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * configs.TxDataZeroGas
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(kvm *vm.KVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		kvm:      kvm,
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
func ApplyMessage(kvm *vm.KVM, msg Message, gp *GasPool) ([]byte, uint64, bool, error) {
	return NewStateTransition(kvm, msg, gp).TransitionDb()
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
		return vm.ErrOutOfGas
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
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, failed bool, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := vm.AccountRef(msg.From())
	contractCreation := msg.To() == nil

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation)
	if err != nil {
		return nil, 0, false, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err
	}

	var (
		kvm = st.kvm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)
	if contractCreation {
		ret, _, st.gas, vmerr = kvm.Create(sender, st.data, st.gas, st.value)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = kvm.Call(sender, st.to(), st.data, st.gas, st.value)
	}
	if vmerr != nil {
		log.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, vmerr
		}
	}
	st.refundGas()
	st.state.AddBalance(st.kvm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	return ret, st.gasUsed(), vmerr != nil, err
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

// GasPool tracks the amount of gas available during execution of the transactions
// in a block. The zero value is a pool with zero gas available.
type GasPool uint64

// AddGas makes gas available for execution.
func (gp *GasPool) AddGas(amount uint64) *GasPool {
	if uint64(*gp) > math.MaxUint64-amount {
		panic("gas pool pushed above uint64")
	}
	*(*uint64)(gp) += amount
	return gp
}

// SubGas deducts the given amount from the pool if enough gas is
// available and returns an error otherwise.
func (gp *GasPool) SubGas(amount uint64) error {
	if uint64(*gp) < amount {
		return ErrGasLimitReached
	}
	*(*uint64)(gp) -= amount
	return nil
}

// Gas returns the amount of gas remaining in the pool.
func (gp *GasPool) Gas() uint64 {
	return uint64(*gp)
}

func (gp *GasPool) String() string {
	return fmt.Sprintf("%d", *gp)
}
