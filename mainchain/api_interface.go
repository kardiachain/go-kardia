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
	"errors"

	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	ErrHeaderNotFound   = errors.New("header for hash not found")
	ErrInvalidArguments = errors.New("invalid arguments; neither block nor hash specified")
	ErrHashNotCanonical = errors.New("hash is not currently canonical")
	ErrMissingBlockBody = errors.New("block body is missing")
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
}

func (k *KardiaService) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return k.blockchain.CurrentBlock().Header()
	}
	return k.blockchain.GetHeader(common.Hash{}, number.Uint64())
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
		return k.blockchain.CurrentBlock()
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
	height := k.DB().ReadHeaderNumber(hash)
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
