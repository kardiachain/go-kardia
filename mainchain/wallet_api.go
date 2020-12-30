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

package kai

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/kardiachain/go-kardia/node"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/rpc"
)

// WalletAPI provides APIs support for extenal Wallet
type WalletAPI struct {
	kaiService *KardiaService
}

// WalletAPI provides APIs support for extenal Wallet
type NetAPI struct {
	kaiService *KardiaService
}

// NewPublicWalletAPI creates a new Kai protocol API for full nodes.
func NewNetAPI(kaiService *KardiaService) *NetAPI {
	return &NetAPI{kaiService}
}

func (w *NetAPI) Version() (string, error) {
	return strconv.FormatInt(node.MainChainID, 10), nil
}

// NewPublicWalletAPI creates a new Kai protocol API for full nodes.
func NewPublicWalletAPI(kaiService *KardiaService) *WalletAPI {
	return &WalletAPI{kaiService}
}

func (w *WalletAPI) ChainId() (*common.Big, error) {
	return (*common.Big)(new(big.Int).SetInt64(int64(node.MainChainID))), nil
}

// BlockNumber returns current block number
func (w *WalletAPI) BlockNumber() uint64 {
	return w.kaiService.blockchain.CurrentBlock().Height()
}

// GetBlockByNumber returns block by block number
func (w *WalletAPI) GetBlockByNumber(ctx context.Context, blockNumber rpc.BlockNumber, fullTx bool) (*BlockJSON, error) {
	block := w.kaiService.BlockByNumber(ctx, blockNumber)
	if block != nil {
		blockInfo := w.kaiService.BlockInfoByBlockHash(ctx, block.Hash())
		return NewBlockJSON(block, blockInfo), nil
	}
	return nil, nil
}

// GetBalance returns address's balance
func (w *WalletAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*common.Big, error) {
	state, _, err := w.kaiService.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*common.Big)(state.GetBalance(address)), nil
}

// Call execute a contract method call only against
// state on the local node. No tx is generated and submitted
// onto the blockchain
func (w *WalletAPI) Call(ctx context.Context, args types.CallArgsJSON, blockNrOrHash rpc.BlockNumberOrHash) (common.Bytes, error) {
	result, err := w.doCall(ctx, args, blockNrOrHash, kvm.Config{}, configs.DefaultTimeOutForStaticCall*time.Second)
	if err != nil {
		return nil, err
	}
	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, newRevertError(result)
	}
	return result.Return(), result.Err
}

// doCall is an interface to make smart contract call against the state of local node
// No tx is generated or submitted to the blockchain
func (w *WalletAPI) doCall(ctx context.Context, args types.CallArgsJSON, blockNrOrHash rpc.BlockNumberOrHash, vmCfg kvm.Config, timeout time.Duration) (*kvm.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing KVM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := w.kaiService.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Create new call message
	msg := args.ToMessage()

	// Get a new instance of the KVM.
	kvm, vmError, err := w.kaiService.GetKVM(ctx, msg, state, header)
	if err != nil {
		return nil, err
	}

	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		kvm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(types.GasPool).AddGas(common.MaxUint64)
	result, err := blockchain.ApplyMessage(kvm, msg, gp)
	if err := vmError(); err != nil {
		return nil, err
	}

	// If the timer caused an abort, return an appropriate error message
	if kvm.Cancelled() {
		return nil, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}
	if err != nil {
		return result, fmt.Errorf("err: %w (supplied gas %d)", err, msg.Gas())
	}

	return result, nil
}

// SendTransaction decode encoded data into tx and then add tx into pool
func (a *PublicTransactionAPI) SendTransaction(txs string) (string, error) {
	log.Debug("SendRawTransaction", "data", txs)
	tx := new(types.Transaction)
	encodedTx := common.FromHex(txs)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}.Hex(), err
	}
	return tx.Hash().Hex(), a.s.TxPool().AddLocal(tx)
}
