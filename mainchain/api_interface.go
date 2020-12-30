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

	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type APIBackend interface {
	// Blockchain API
	HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header
	HeaderByHash(ctx context.Context, hash common.Hash) *types.Header
	HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error)
	BlockByNumber(ctx context.Context, number rpc.BlockNumber) *types.Block
	BlockByHash(ctx context.Context, hash common.Hash) *types.Block
	BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error)
	BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo
	StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)
	GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error)
	GetValidators() ([]*staking.Validator, error)
	GetValidator(valAddr common.Address) (*staking.Validator, error)
	GetValidatorCommission(valAddr common.Address) (uint64, error)
	GetDelegationsByValidator(valAddr common.Address) ([]*staking.Delegator, error)
}

func (s *KardiaService) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return s.blockchain.CurrentBlock().Header()
	}
	return s.blockchain.GetHeader(common.Hash{}, number.Uint64())
}

func (s *KardiaService) HeaderByHash(ctx context.Context, hash common.Hash) *types.Header {
	return s.blockchain.GetHeaderByHash(hash)
}

func (s *KardiaService) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return s.HeaderByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := s.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && s.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		return header, nil
	}
	return nil, ErrInvalidArguments
}

func (s *KardiaService) BlockByNumber(ctx context.Context, number rpc.BlockNumber) *types.Block {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return s.blockchain.CurrentBlock()
	}
	return s.blockchain.GetBlockByHeight(number.Uint64())
}

func (s *KardiaService) BlockByHash(ctx context.Context, hash common.Hash) *types.Block {
	return s.blockchain.GetBlockByHash(hash)
}

func (s *KardiaService) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return s.BlockByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		// get block header in order to get height of the block
		header := s.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && s.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, ErrHashNotCanonical
		}
		block := s.blockchain.GetBlock(hash, header.Height)
		if block == nil {
			return nil, ErrMissingBlockBody
		}
		return block, nil
	}
	return nil, ErrInvalidArguments
}

func (s *KardiaService) BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo {
	height := s.DB().ReadHeaderHeight(hash)
	if height == nil {
		return nil
	}
	return s.DB().ReadBlockInfo(hash, *height)
}

func (s *KardiaService) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Return the latest state if rpc.LatestBlockNumber has been passed in
	header := s.HeaderByNumber(ctx, number)
	if header == nil {
		return nil, nil, ErrHeaderNotFound
	}
	stateDb, err := s.BlockChain().StateAt(header.Height)
	return stateDb, header, err
}

func (s *KardiaService) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return s.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := s.HeaderByHash(ctx, hash)
		if header == nil {
			return nil, nil, ErrHeaderNotFound
		}
		if blockNrOrHash.RequireCanonical && s.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, nil, ErrHashNotCanonical
		}
		stateDb, err := s.BlockChain().StateAt(header.Height)
		return stateDb, header, err
	}
	return nil, nil, ErrInvalidArguments
}

func (s *KardiaService) GetKVM(ctx context.Context, msg types.Message, state *state.StateDB, header *types.Header) (*kvm.KVM, func() error, error) {
	vmError := func() error { return nil }

	context := vm.NewKVMContext(msg, header, s.BlockChain())
	return kvm.NewKVM(context, state, *s.blockchain.GetVMConfig()), vmError, nil
}

// ValidatorsListFromStakingContract returns all validators on staking
// contract at the moment
func (s *KardiaService) GetValidators() ([]*staking.Validator, error) {
	block := s.blockchain.CurrentBlock()
	st, header, kvmConfig, err := s.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	allValsLen, err := s.staking.GetAllValsLength(st, header, s.blockchain, kvmConfig)
	if err != nil {
		return nil, err
	}
	var (
		one      = big.NewInt(1)
		valsInfo []*staking.Validator
	)
	zero := new(big.Int).SetInt64(0)
	for i := new(big.Int).SetInt64(0); i.Cmp(allValsLen) < 0; i.Add(i, one) {
		valContractAddr, err := s.staking.GetValSmcAddr(st, header, s.blockchain, kvmConfig, i)
		if err != nil {
			return nil, err
		}
		valInfo, err := s.validator.GetInforValidator(st, header, s.blockchain, kvmConfig, valContractAddr)
		if err != nil {
			return nil, err
		}
		if valInfo.Tokens.Cmp(zero) == 1 {
			valInfo.Delegators, err = s.GetDelegationsByValidator(valContractAddr)
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
func (s *KardiaService) GetValidator(valAddr common.Address) (*staking.Validator, error) {
	block := s.blockchain.CurrentBlock()
	st, header, kvmConfig, err := s.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	valContractAddr, err := s.staking.GetValFromOwner(st, header, s.blockchain, kvmConfig, valAddr)
	if err != nil {
		return nil, err
	}
	val, err := s.validator.GetInforValidator(st, header, s.blockchain, kvmConfig, valContractAddr)
	if err != nil {
		return nil, err
	}
	zero := new(big.Int).SetInt64(0)
	if val.Tokens.Cmp(zero) == 1 {
		val.Delegators, err = s.GetDelegationsByValidator(valContractAddr)
		if err != nil {
			return nil, err
		}
	}
	val.ValStakingSmc = valContractAddr
	return val, nil
}

// GetDelegationsByValidator returns delegations info of one validator on staking contract based on their contract addresses
func (s *KardiaService) GetDelegationsByValidator(valContractAddr common.Address) ([]*staking.Delegator, error) {
	block := s.blockchain.CurrentBlock()
	st, header, kvmConfig, err := s.getValidatorInfoParams(block)
	if err != nil {
		return nil, err
	}
	return s.validator.GetDelegators(st, header, s.blockchain, kvmConfig, valContractAddr)
}

// getValidatorInfoParams returns params for getting validators info on
// staking and validator contract
func (s *KardiaService) getValidatorInfoParams(block *types.Block) (*state.StateDB, *types.Header, kvm.Config, error) {
	// Blockchain state at head block.
	kvmConfig := kvm.Config{}
	st, err := s.blockchain.State()
	if err != nil {
		s.logger.Error("Fail to get blockchain head state", "err", err)
		return nil, nil, kvmConfig, err
	}

	return st, block.Header(), kvmConfig, nil
}
