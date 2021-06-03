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
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
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
func (s *PublicWeb3API) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	price, err := s.kaiService.SuggestPrice(ctx)
	return (*hexutil.Big)(price), err
}

// ChainId returns chain ID for the current KardiaChain config.
func (s *PublicWeb3API) ChainId() *common.Big {
	return (*common.Big)(new(big.Int).SetUint64(s.kaiService.config.ChainId))
}

// BlockNumber returns the block number of the chain head.
func (s *PublicWeb3API) BlockNumber() hexutil.Uint64 {
	header := s.kaiService.HeaderByHeight(context.Background(), rpc.LatestBlockHeight) // latest header should always be available
	return hexutil.Uint64(header.Height)
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
// given block number. The rpc.LatestBlockHeight and rpc.PendingBlockHeight meta
// block numbers are also allowed.
func (s *PublicWeb3API) GetBalance(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Big, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*common.Big)(state.GetBalance(address)), state.Error()
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
}

// Call executes the given transaction on the state for the given block height.
// Note, this function doesn't make and changes in the state/blockchain and is
// useful to execute and retrieve values.
func (s *PublicWeb3API) Call(ctx context.Context, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash) (hexutil.Bytes, error) {
	result, err := s.DoCall(ctx, args, blockHeightOrHash, kvm.Config{}, configs.DefaultTimeOutForStaticCall, configs.GasLimitCap)
	if err != nil {
		return nil, err
	}
	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, newRevertError(result)
	}
	return result.Return(), result.Err
}

func (s *PublicWeb3API) DoCall(ctx context.Context, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash, kvmCfg kvm.Config,
	timeout time.Duration, globalGasCap uint64) (*kvm.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
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
	kvm, vmError, err := s.kaiService.GetKVM(ctx, msg, state, header)
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

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *hexutil.Uint64 `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	Type             hexutil.Uint64  `json:"type"`
	ChainID          *hexutil.Big    `json:"chainId,omitempty"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
}
