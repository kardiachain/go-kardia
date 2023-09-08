// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package kaiapi implements the general Ethereum API functions.
package kaiapi

import (
	"context"
	"math/big"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/staking"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// Blockchain API
	BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block
	BlockByHash(ctx context.Context, hash common.Hash) *types.Block
	BlockByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Block, error)
	BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo

	ChainConfig() *configs.ChainConfig

	GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error)
	GetValidators() ([]*staking.Validator, error)
	GetValidator(valAddr common.Address) (*staking.Validator, error)
	GetDelegationsByValidator(valAddr common.Address) ([]*staking.Delegator, error)
	GetValidatorSet(ctx context.Context, height rpc.BlockHeight) (*types.ValidatorSet, error)

	HeaderByHeight(ctx context.Context, height rpc.BlockHeight) *types.Header
	HeaderByHash(ctx context.Context, hash common.Hash) *types.Header
	HeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Header, error)

	StateAndHeaderByHeight(ctx context.Context, height rpc.BlockHeight) (*state.StateDB, *types.Header, error)
	StateAndHeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*state.StateDB, *types.Header, error)

	SuggestPrice(ctx context.Context) (*big.Int, error)

	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription

	// Filter API
	BloomStatus() (uint64, uint64)
	GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription

	// KVMLogger API
	GetTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64)
	RPCGasCap() uint64
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.BlockContext, *state.StateDB, error)

	// Txpool API
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions)
	Stats() (pending int, queued int)
}

// GetAPIs gather all APIs defined in /internal/kaiapi
func GetAPIs(apiBackend Backend) []rpc.API {
	return []rpc.API{}
}
