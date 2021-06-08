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

package kai

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	networkVersion uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{networkVersion}
}

// Version returns the current KardiaChain protocol version.
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}

// PublicWeb3API provides web3-compatible APIs to access the KardiaChain blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicWeb3API struct {
	kaiService *KardiaService
}

// NewPublicWeb3API creates a new KardiaChain blockchain web3 APIs.
func NewPublicWeb3API(k *KardiaService) *PublicWeb3API {
	return &PublicWeb3API{k}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicWeb3API) GasPrice(ctx context.Context) (*common.Big, error) {
	price, err := s.kaiService.SuggestPrice(ctx)
	return (*common.Big)(price), err
}

// ChainId returns chain ID for the current KardiaChain config.
func (s *PublicWeb3API) ChainId() *common.Big {
	return (*common.Big)(new(big.Int).SetUint64(0))
}

// BlockNumber returns the block height of the chain head.
func (s *PublicWeb3API) BlockNumber() common.Uint64 {
	header := s.kaiService.HeaderByHeight(context.Background(), rpc.LatestBlockHeight) // latest header should always be available
	return common.Uint64(header.Height)
}

// GetHeaderByNumber returns the requested canonical block header.
// * When blockNr is math.MaxUint64 - 1 the chain head is returned.
// * When blockNr is math.MaxUint64 - 2 the pending chain head is returned.
func (s *PublicWeb3API) GetHeaderByNumber(ctx context.Context, height rpc.BlockHeight) (map[string]interface{}, error) {
	header := s.kaiService.HeaderByHeight(ctx, height)
	if header != nil {
		response := s.rpcMarshalHeader(ctx, header)
		if height == rpc.PendingBlockHeight {
			// Pending header need to nil out a few fields
			for _, field := range []string{"hash", "miner"} {
				response[field] = nil
			}
		}
		return response, nil
	}
	return nil, ErrHeaderNotFound
}

// GetHeaderByHash returns the requested header by hash.
func (s *PublicWeb3API) GetHeaderByHash(ctx context.Context, hash common.Hash) map[string]interface{} {
	header := s.kaiService.HeaderByHash(ctx, hash)
	if header != nil {
		return s.rpcMarshalHeader(ctx, header)
	}
	return nil
}

// GetBlockByNumber returns the requested canonical block.
// * When blockNr is -1 the chain head is returned.
// * When blockNr is -2 the pending chain head is returned.
// * When fullTx is true all transactions in the block are returned, otherwise
//   only the transaction hash is returned.
func (s *PublicWeb3API) GetBlockByNumber(ctx context.Context, height rpc.BlockHeight, fullTx bool) (map[string]interface{}, error) {
	block := s.kaiService.BlockByHeight(ctx, height)
	if block != nil {
		response, err := s.rpcMarshalBlock(ctx, block, true, fullTx)
		if err == nil && height == rpc.PendingBlockHeight {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, ErrBlockNotFound
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicWeb3API) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block := s.kaiService.BlockByHash(ctx, hash)
	if block != nil {
		return s.rpcMarshalBlock(ctx, block, true, fullTx)
	}
	return nil, ErrBlockNotFound
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block height. The rpc.LatestBlockHeight and rpc.PendingBlockHeight meta
// block heights are also allowed.
func (s *PublicWeb3API) GetBalance(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Big, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*common.Big)(state.GetBalance(address)), state.Error()
}

// GetCode returns the code stored at the given address in the state for the given block height.
func (s *PublicWeb3API) GetCode(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *common.Uint64  `json:"gas"`
	GasPrice *common.Big     `json:"gasPrice"`
	Value    *common.Big     `json:"value"`
	Data     *common.Bytes   `json:"data"`
}

// Call executes the given transaction on the state for the given block height.
// Note, this function doesn't make and changes in the state/blockchain and is
// useful to execute and retrieve values.
func (s *PublicWeb3API) Call(ctx context.Context, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	result, err := DoCall(ctx, s.kaiService, args, blockHeightOrHash, kvm.Config{}, configs.DefaultTimeOutForStaticCall*time.Second, configs.GasLimitCap)
	if err != nil {
		return nil, err
	}
	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, newRevertError(result)
	}
	return result.Return(), result.Err
}

func DoCall(ctx context.Context, s APIBackend, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash, kvmCfg kvm.Config,
	timeout time.Duration, globalGasCap uint64) (*kvm.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing KVM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := s.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
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

	// Get a new instance of the KVM.
	msg := args.ToMessage(globalGasCap)
	kvm, vmError, err := s.GetKVM(ctx, msg, state, header)
	if err != nil {
		return nil, err
	}
	// Wait for the context to be done and cancel the KVM. Even if the
	// KVM has finished, cancelling may be done (repeatedly)
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

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicWeb3API) EstimateGas(ctx context.Context, args CallArgs, blockHeightOrHash *rpc.BlockHeightOrHash) (common.Uint64, error) {
	bHeightOrHash := rpc.BlockHeightOrHashWithHeight(rpc.PendingBlockHeight)
	if blockHeightOrHash != nil {
		bHeightOrHash = *blockHeightOrHash
	}
	return DoEstimateGas(ctx, s.kaiService, args, bHeightOrHash, configs.GasLimitCap)
}

func DoEstimateGas(ctx context.Context, b APIBackend, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash, gasCap uint64) (common.Uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  uint64 = configs.TxGas - 1
		hi  uint64
		cap uint64
	)
	// Use zero address if sender unspecified.
	if args.From == nil {
		args.From = new(common.Address)
	}
	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= configs.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Retrieve the block to act as the gas ceiling
		block, err := b.BlockByHeightOrHash(ctx, blockHeightOrHash)
		if err != nil {
			return 0, err
		}
		if block == nil {
			return 0, errors.New("block not found")
		}
		hi = block.GasLimit()
	}
	// Recap the highest gas limit with account's available balance.
	if args.GasPrice != nil && args.GasPrice.ToInt().BitLen() != 0 {
		state, _, err := b.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
		if err != nil {
			return 0, err
		}
		balance := state.GetBalance(*args.From) // from can't be nil
		available := new(big.Int).Set(balance)
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available) >= 0 {
				return 0, errors.New("insufficient funds for transfer")
			}
			available.Sub(available, args.Value.ToInt())
		}
		allowance := new(big.Int).Div(available, args.GasPrice.ToInt())

		// If the allowance is larger than maximum uint64, skip checking
		if allowance.IsUint64() && hi > allowance.Uint64() {
			transfer := args.Value
			if transfer == nil {
				transfer = new(common.Big)
			}
			log.Warn("Gas estimation capped by limited funds", "original", hi, "balance", balance,
				"sent", transfer.ToInt(), "gasprice", args.GasPrice.ToInt(), "fundable", allowance)
			hi = allowance.Uint64()
		}
	}
	// Recap the highest gas allowance with specified gascap.
	if gasCap != 0 && hi > gasCap {
		log.Warn("Caller gas above allowance, capping", "requested", hi, "cap", gasCap)
		hi = gasCap
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, *kvm.ExecutionResult, error) {
		args.Gas = (*common.Uint64)(&gas)

		result, err := DoCall(ctx, b, args, blockHeightOrHash, kvm.Config{}, 0, gasCap)
		if err != nil {
			if errors.Is(err, tx_pool.ErrIntrinsicGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return result.Failed(), result, nil
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		failed, _, err := executable(mid)

		// If the error is not nil(consensus error), it means the provided message
		// call or transaction will never be accepted no matter how much gas it is
		// assigned. Return the error directly, don't struggle any more.
		if err != nil {
			return 0, err
		}
		if failed {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		failed, result, err := executable(hi)
		if err != nil {
			return 0, err
		}
		if failed {
			if result != nil && result.Err != kvm.ErrOutOfGas {
				if len(result.Revert()) > 0 {
					return 0, newRevertError(result)
				}
				return 0, result.Err
			}
			// Otherwise, the specified gas cap is too low
			return 0, fmt.Errorf("gas required exceeds allowance (%d)", cap)
		}
	}
	return common.Uint64(hi), nil
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockHeight      *common.Big     `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              common.Uint64   `json:"gas"`
	GasPrice         *common.Big     `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            common.Bytes    `json:"input"`
	Nonce            common.Uint64   `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *common.Uint64  `json:"transactionIndex"`
	Value            *common.Big     `json:"value"`
	Type             common.Uint64   `json:"type"`
	ChainID          *common.Big     `json:"chainId,omitempty"`
	V                *common.Big     `json:"v"`
	R                *common.Big     `json:"r"`
	S                *common.Big     `json:"s"`
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	kaiService *KardiaService
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(k *KardiaService) *PublicTransactionPoolAPI {
	// The signer used by the API should always be the 'latest' known one because we expect
	// signers to be backwards-compatible with old transactions.
	return &PublicTransactionPoolAPI{k}
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*RPCTransaction, error) {
	// Try to return an already finalized transaction
	tx, blockHash, blockHeight, index := s.kaiService.GetTransaction(ctx, hash)
	if tx != nil {
		return newRPCTransaction(tx, blockHash, blockHeight, index), nil
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.kaiService.TxPool().Get(hash); tx != nil {
		return newRPCPendingTransaction(tx), nil
	}

	// Transaction unknown, return as such
	return nil, nil
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (common.Bytes, error) {
	// Retrieve a finalized transaction, or a pooled otherwise
	tx, _, _, _ := s.kaiService.GetTransaction(ctx, hash)
	if tx == nil {
		if tx = s.kaiService.TxPool().Get(hash); tx == nil {
			// Transaction not found anywhere, abort
			return nil, ErrTransactionHashNotFound
		}
	}
	// Serialize to RLP and return
	return rlp.EncodeToBytes(tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	tx, blockHash, blockHeight, index := s.kaiService.GetTransaction(ctx, hash)
	if tx == nil {
		return nil, ErrTransactionHashNotFound
	}
	// get receipts from db
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, blockHash)
	if blockInfo == nil {
		return nil, ErrBlockInfoNotFound
	}
	if len(blockInfo.Receipts) <= int(index) {
		return nil, nil
	}
	receipt := blockInfo.Receipts[index]

	// Derive the sender
	from, _ := types.Sender(types.HomesteadSigner{}, tx)
	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       common.Uint64(blockHeight),
		"transactionHash":   hash,
		"transactionIndex":  common.Uint64(index),
		"from":              from,
		"to":                tx.To(),
		"gasUsed":           common.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": common.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
	}
	bloom, err := UnmarshalLogsBloom(&blockInfo.Bloom)
	if err == nil {
		fields["logsBloom"] = bloom
	}
	web3Logs := make([]*types.LogForWeb3, len(receipt.Logs))
	for i := range receipt.Logs {
		web3Logs[i] = &types.LogForWeb3{
			Log: *receipt.Logs[i],
		}
	}
	fields["logs"] = web3Logs

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = common.Bytes(receipt.PostState)
	} else {
		fields["status"] = common.Uint(receipt.Status)
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Uint64, error) {
	// Ask transaction pool for the nonce which includes pending transactions
	if blockHeight, ok := blockHeightOrHash.Height(); ok && blockHeight == rpc.PendingBlockHeight {
		nonce := s.kaiService.txPool.Nonce(address)
		return (*common.Uint64)(&nonce), nil
	}
	// Resolve block number and use its state to ask for the nonce
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	nonce := state.GetNonce(address)
	return (*common.Uint64)(&nonce), state.Error()
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, input common.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(input, &tx); err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), s.kaiService.TxPool().AddLocal(tx)
}
