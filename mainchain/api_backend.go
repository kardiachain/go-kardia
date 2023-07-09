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
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// KaiAPIBackend implements kaiapi.Backend and tracers.Backend for full nodes
type KaiAPIBackend struct {
	kai *Kardiachain
	gpo *oracles.Oracle
}

func NewKaiAPIBackend(kai *Kardiachain, gpo *oracles.Oracle) *KaiAPIBackend {
	return &KaiAPIBackend{kai, gpo}
}

func (k *KaiAPIBackend) HeaderByHeight(ctx context.Context, height rpc.BlockHeight) *types.Header {
	// Return the latest block if rpc.LatestBlockHeight or rpc.PendingBlockHeight has been passed in
	if height.Uint64() == rpc.PendingBlockHeight.Uint64() {
		return k.kai.blockchain.CurrentBlock().Header()
	} else if height.Uint64() == rpc.LatestBlockHeight.Uint64() {
		return k.kai.blockchain.GetHeaderByHeight(k.kai.blockchain.CurrentBlock().Header().Height - 1)
	}
	return k.kai.blockchain.GetHeaderByHeight(height.Uint64())
}

func (k *KaiAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) *types.Header {
	return k.kai.blockchain.GetHeaderByHash(hash)
}

func (k *KaiAPIBackend) HeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Header, error) {
	if blockHeight, ok := blockHeightOrHash.Height(); ok {
		return k.HeaderByHeight(ctx, blockHeight), nil
	}
	if hash, ok := blockHeightOrHash.Hash(); ok {
		header := k.kai.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockHeightOrHash.RequireCanonical && rawdb.ReadCanonicalHash(k.kai.chainDb, header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		return header, nil
	}
	return nil, ErrInvalidArguments
}

func (k *KaiAPIBackend) BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block {
	// Return the latest block if rpc.LatestBlockHeight has been passed in
	if height.Uint64() >= rpc.PendingBlockHeight.Uint64() {
		return k.kai.blockchain.CurrentBlock()
	}
	return k.kai.blockchain.GetBlockByHeight(height.Uint64())
}

func (k *KaiAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) *types.Block {
	return k.kai.blockchain.GetBlockByHash(hash)
}

func (k *KaiAPIBackend) BlockByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Block, error) {
	if blockHeight, ok := blockHeightOrHash.Height(); ok {
		return k.BlockByHeight(ctx, blockHeight), nil
	}
	if hash, ok := blockHeightOrHash.Hash(); ok {
		// get block header in order to get height of the block
		header := k.kai.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockHeightOrHash.RequireCanonical && rawdb.ReadCanonicalHash(k.kai.chainDb, header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		block := k.kai.blockchain.GetBlock(hash, header.Height)
		if block == nil {
			return nil, ErrMissingBlockBody
		}
		return block, nil
	}
	return nil, ErrInvalidArguments
}

func (k *KaiAPIBackend) BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo {
	height := rawdb.ReadHeaderHeight(k.kai.chainDb, hash)
	if height == nil {
		return nil
	}
	if *height == 0 {
		return &types.BlockInfo{
			GasUsed:  0,
			Rewards:  new(big.Int).SetInt64(0),
			Receipts: types.Receipts{},
			Bloom:    types.Bloom{},
		}
	}
	return rawdb.ReadBlockInfo(k.kai.chainDb, hash, *height, k.kai.chainConfig)
}

func (k *KaiAPIBackend) StateAndHeaderByHeight(ctx context.Context, height rpc.BlockHeight) (*state.StateDB, *types.Header, error) {
	// Return the latest state if rpc.LatestBlockHeight has been passed in
	header := k.HeaderByHeight(ctx, height)
	if header == nil {
		return nil, nil, ErrHeaderNotFound
	}
	stateDb, err := k.kai.blockchain.StateAt(header.Height)
	return stateDb, header, err
}

func (k *KaiAPIBackend) StateAndHeaderByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*state.StateDB, *types.Header, error) {
	if blockHeight, ok := blockHeightOrHash.Height(); ok {
		return k.StateAndHeaderByHeight(ctx, blockHeight)
	}
	if hash, ok := blockHeightOrHash.Hash(); ok {
		header := k.HeaderByHash(ctx, hash)
		if header == nil {
			return nil, nil, ErrHeaderNotFound
		}
		if blockHeightOrHash.RequireCanonical && rawdb.ReadCanonicalHash(k.kai.chainDb, header.Height) != hash {
			return nil, nil, ErrHashNotCanonical
		}
		stateDb, err := k.kai.blockchain.StateAt(header.Height)
		return stateDb, header, err
	}
	return nil, nil, ErrInvalidArguments
}

func (k *KaiAPIBackend) GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error) {
	vmError := func() error { return nil }

	context := vm.NewKVMContext(msg, header, k.kai.blockchain)
	return kvm.NewKVM(context, blockchain.NewKVMTxContext(msg), state, k.kai.chainConfig, *k.kai.blockchain.GetVMConfig()), vmError, nil
}

// ValidatorsListFromStakingContract returns all validators on staking
// contract at the moment
func (k *KaiAPIBackend) GetValidators() ([]*staking.Validator, error) {
	block := k.kai.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	allValsLen, err := k.kai.staking.GetAllValsLength(st, header, k.kai.blockchain, kvmConfig)
	if err != nil {
		return nil, err
	}
	var (
		one      = big.NewInt(1)
		valsInfo []*staking.Validator
	)
	zero := new(big.Int).SetInt64(0)
	for i := new(big.Int).SetInt64(0); i.Cmp(allValsLen) < 0; i.Add(i, one) {
		valContractAddr, err := k.kai.staking.GetValSmcAddr(st, header, k.kai.blockchain, kvmConfig, i)
		if err != nil {
			return nil, err
		}
		valInfo, err := k.kai.validator.GetInforValidator(st, header, k.kai.blockchain, kvmConfig, valContractAddr)
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
func (k *KaiAPIBackend) GetValidator(valAddr common.Address) (*staking.Validator, error) {
	block := k.kai.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	valContractAddr, err := k.kai.staking.GetValFromOwner(st, header, k.kai.blockchain, kvmConfig, valAddr)
	if err != nil {
		return nil, err
	}
	val, err := k.kai.validator.GetInforValidator(st, header, k.kai.blockchain, kvmConfig, valContractAddr)
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
func (k *KaiAPIBackend) GetDelegationsByValidator(valContractAddr common.Address) ([]*staking.Delegator, error) {
	block := k.kai.blockchain.CurrentBlock()
	st, header, kvmConfig, err := k.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	return k.kai.validator.GetDelegators(st, header, k.kai.blockchain, kvmConfig, valContractAddr)
}

// getValidatorInfoParams returns params for getting validators info on
// staking and validator contract
func (k *KaiAPIBackend) getValidatorInfoParams(block *types.Block) (*state.StateDB, *types.Header, kvm.Config, error) {
	// Blockchain state at head block.
	kvmConfig := kvm.Config{}
	st, err := k.kai.blockchain.State()
	if err != nil {
		log.Error("Fail to get blockchain head state", "err", err)
		return nil, nil, kvmConfig, err
	}

	return st, block.Header(), kvmConfig, nil
}

// filter APIs interface

func (k *KaiAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return k.kai.blockchain.SubscribeLogsEvent(ch)
}

func (k *KaiAPIBackend) SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription {
	return k.kai.blockchain.SubscribeChainHeadEvent(ch)
}

func (k *KaiAPIBackend) SubscribeNewTxsEvent(ch chan<- events.NewTxsEvent) event.Subscription {
	return k.kai.TxPool().SubscribeNewTxsEvent(ch)
}

func (k *KaiAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := k.kai.bloomIndexer.Sections()
	return configs.BloomBitsBlocks, sections
}

func (k *KaiAPIBackend) ChainDb() kaidb.Database {
	return k.kai.DB()
}

func (k *KaiAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
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

func (k *KaiAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, k.kai.bloomRequests)
	}
}

func (k *KaiAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return k.gpo.SuggestPrice(ctx)
}

func (k *KaiAPIBackend) GetTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return rawdb.ReadTransaction(k.kai.chainDb, hash)
}

func (k *KaiAPIBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.BlockContext, *state.StateDB, error) {
	return k.kai.stateAtTransaction(block, txIndex, reexec)
}

func (k *KaiAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return k.kai.txPool.Content()
}

func (k *KaiAPIBackend) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
	return k.kai.txPool.ContentFrom(addr)
}

func (k *KaiAPIBackend) Stats() (pending int, queued int) {
	return k.kai.txPool.Stats()
}

// Backward compatible fix
func (k *KaiAPIBackend) Config() *configs.ChainConfig {
	return k.ChainConfig()
}

func (k *KaiAPIBackend) ChainConfig() *configs.ChainConfig {
	return k.kai.chainConfig
}

func (k *KaiAPIBackend) RPCGasCap() uint64 {
	return configs.GasLimitCap
}

func (k *KaiAPIBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error) {
	return k.kai.stateAtBlock(block, reexec, base, checkLive)
}

func (k *KaiAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return k.kai.txPool.Nonce(addr), nil
}

func (k *KaiAPIBackend) CurrentBlock(ctx context.Context) *types.Header {
	return k.kai.blockchain.CurrentBlock().Header()
}

func (k *KaiAPIBackend) GetValidatorSet(ctx context.Context, blockHeight rpc.BlockHeight) (*types.ValidatorSet, error) {
	storeDb := cstate.NewStore(k.kai.chainDb)
	return storeDb.LoadValidators(blockHeight.Uint64())
}

func (k *KaiAPIBackend) ReadCommit(ctx context.Context, height rpc.BlockHeight) *types.Commit {
	return rawdb.ReadCommit(k.kai.chainDb, height.Uint64())
}
