/*
 *  Copyright 2020 KardiaChain
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
	"math/big"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type APIBackend interface {
	// Blockchain API
	BlockByNumber(ctx context.Context, number rpc.BlockNumber) *types.Block
	BlockByHash(ctx context.Context, hash common.Hash) *types.Block
	BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error)
	BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo

	GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error)
	GetValidators() ([]*staking.Validator, error)
	GetValidator(valAddr common.Address) (*staking.Validator, error)
	GetValidatorCommission(valAddr common.Address) (uint64, error)
	GetDelegationsByValidator(valAddr common.Address) ([]*staking.Delegator, error)

	HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header
	HeaderByHash(ctx context.Context, hash common.Hash) *types.Header
	HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error)

	StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)

	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription

	// Filter API
	BloomStatus() (uint64, uint64)
	GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
}

func (k *KardiaService) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return k.blockchain.GetHeaderByHeight(k.blockchain.CurrentBlock().Header().Height - 1)
	}
	return k.blockchain.GetHeaderByHeight(number.Uint64())
}

func (k *KardiaService) HeaderByHash(ctx context.Context, hash common.Hash) *types.Header {
	return k.blockchain.GetHeaderByHash(hash)
}

func (k *KardiaService) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.HeaderByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := k.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && k.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		return header, nil
	}
	return nil, ErrInvalidArguments
}

func (k *KardiaService) BlockByNumber(ctx context.Context, number rpc.BlockNumber) *types.Block {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return k.blockchain.GetBlockByHeight(k.blockchain.CurrentBlock().Height() - 1)
	}
	return k.blockchain.GetBlockByHeight(number.Uint64())
}

func (k *KardiaService) BlockByHash(ctx context.Context, hash common.Hash) *types.Block {
	return k.blockchain.GetBlockByHash(hash)
}

func (k *KardiaService) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.BlockByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		// get block header in order to get height of the block
		header := k.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && k.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		block := k.blockchain.GetBlock(hash, header.Height)
		if block == nil {
			return nil, ErrMissingBlockBody
		}
		return block, nil
	}
	return nil, ErrInvalidArguments
}

func (k *KardiaService) BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo {
	height := k.DB().ReadHeaderHeight(hash)
	if height == nil {
		return nil
	}
	return k.DB().ReadBlockInfo(hash, *height)
}

func (k *KardiaService) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Return the latest state if rpc.LatestBlockNumber has been passed in
	header := k.HeaderByNumber(ctx, number)
	if header == nil {
		return nil, nil, ErrHeaderNotFound
	}
	stateDb, err := k.BlockChain().StateAt(header.Height)
	return stateDb, header, err
}

func (k *KardiaService) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := k.HeaderByHash(ctx, hash)
		if header == nil {
			return nil, nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && k.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, nil, ErrHashNotCanonical
		}
		stateDb, err := k.BlockChain().StateAt(header.Height)
		return stateDb, header, err
	}
	return nil, nil, ErrInvalidArguments
}

func (k *KardiaService) GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error) {
	vmError := func() error { return nil }

	context := vm.NewKVMContext(msg, header, k.BlockChain())
	return kvm.NewKVM(context, state, *k.blockchain.GetVMConfig()), vmError, nil
}

// ValidatorsListFromStakingContract returns all validators on staking
// contract at the moment
func (k *KardiaService) GetValidators() ([]*staking.Validator, error) {
	block := k.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	allValsLen, err := k.staking.GetAllValsLength(st, header, k.blockchain, kvmConfig)
	if err != nil {
		return nil, err
	}
	var (
		one      = big.NewInt(1)
		valsInfo []*staking.Validator
	)
	zero := new(big.Int).SetInt64(0)
	for i := new(big.Int).SetInt64(0); i.Cmp(allValsLen) < 0; i.Add(i, one) {
		valContractAddr, err := k.staking.GetValSmcAddr(st, header, k.blockchain, kvmConfig, i)
		if err != nil {
			return nil, err
		}
		valInfo, err := k.validator.GetInforValidator(st, header, k.blockchain, kvmConfig, valContractAddr)
		if err != nil {
			return nil, err
		}
		if valInfo.Tokens.Cmp(zero) == 1 {
			valInfo.Delegators, err = k.GetDelegationsByValidator(valContractAddr)
			if err != nil {
				return nil, err
			}
		}
		valInfo.ValStakingSmc = valContractAddr
		valsInfo = append(valsInfo, valInfo)
	}
	return valsInfo, nil
}

// ValidatorsListFromStakingContract returns info of one validator on staking
// contract based on his address
func (k *KardiaService) GetValidator(valAddr common.Address) (*staking.Validator, error) {
	block := k.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	valContractAddr, err := k.staking.GetValFromOwner(st, header, k.blockchain, kvmConfig, valAddr)
	if err != nil {
		return nil, err
	}
	val, err := k.validator.GetInforValidator(st, header, k.blockchain, kvmConfig, valContractAddr)
	if err != nil {
		return nil, err
	}
	zero := new(big.Int).SetInt64(0)
	if val.Tokens.Cmp(zero) == 1 {
		val.Delegators, err = k.GetDelegationsByValidator(valContractAddr)
		if err != nil {
			return nil, err
		}
	}
	val.ValStakingSmc = valContractAddr
	return val, nil
}

// GetDelegationsByValidator returns delegations info of one validator on staking contract based on their contract addresses
func (k *KardiaService) GetDelegationsByValidator(valContractAddr common.Address) ([]*staking.Delegator, error) {
	block := k.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	return k.validator.GetDelegators(st, header, k.blockchain, kvmConfig, valContractAddr)
}

// getValidatorInfoParams returns params for getting validators info on
// staking and validator contract
func (k *KardiaService) getValidatorInfoParams(block *types.Block) (*state.StateDB, *types.Header, kvm.Config, error) {
	// Blockchain state at head block.
	kvmConfig := kvm.Config{}
	st, err := k.blockchain.State()
	if err != nil {
		k.logger.Error("Fail to get blockchain head state", "err", err)
		return nil, nil, kvmConfig, err
	}

	return st, block.Header(), kvmConfig, nil
}

// filter APIs interface

func (k *KardiaService) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return k.blockchain.SubscribeLogsEvent(ch)
}

func (k *KardiaService) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return k.blockchain.SubscribeChainHeadEvent(ch)
}

func (k *KardiaService) SubscribeNewTxsEvent(ch chan<- events.NewTxsEvent) event.Subscription {
	return k.TxPool().SubscribeNewTxsEvent(ch)
}

func (k *KardiaService) BloomStatus() (uint64, uint64) {
	sections, _, _ := k.bloomIndexer.Sections()
	return configs.BloomBitsBlocks, sections
}

func (k *KardiaService) ChainDb() types.StoreDB {
	return k.BlockChain().DB()
}

func (k *KardiaService) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	blockInfo := k.BlockInfoByBlockHash(ctx, hash)
	if blockInfo == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(blockInfo.Receipts))
	for i, receipt := range blockInfo.Receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (k *KardiaService) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, k.bloomRequests)
	}
}
