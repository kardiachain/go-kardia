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
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
)

type KaiAPIBackend struct {
	kaiService *KardiaService
}

func (k *KaiAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) *types.Header {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return k.kaiService.blockchain.CurrentBlock().Header()
	}
	return k.kaiService.blockchain.GetHeader(common.Hash{}, number.Uint64())
}

func (k *KaiAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) *types.Header {
	return k.kaiService.blockchain.GetHeaderByHash(hash)
}

func (k *KaiAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.HeaderByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := k.kaiService.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && k.kaiService.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (k *KaiAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) *types.Block {
	// Return the latest block if rpc.LatestBlockNumber has been passed in
	if number == rpc.LatestBlockNumber {
		return k.kaiService.blockchain.CurrentBlock()
	}
	return k.kaiService.blockchain.GetBlockByHeight(number.Uint64())
}

func (k *KaiAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) *types.Block {
	return k.kaiService.blockchain.GetBlockByHash(hash)
}

func (k *KaiAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.BlockByNumber(ctx, blockNr), nil
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		// get block header in order to get height of the block
		header := k.kaiService.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && k.kaiService.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := k.kaiService.blockchain.GetBlock(hash, header.Height)
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (k *KaiAPIBackend) BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo {
	height := k.kaiService.DB().ReadHeaderNumber(hash)
	if height == nil {
		return nil
	}
	return k.kaiService.DB().ReadBlockInfo(hash, *height)
}

func (k *KaiAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Return the latest state if rpc.LatestBlockNumber has been passed in
	header := k.HeaderByNumber(ctx, number)
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := k.kaiService.BlockChain().StateAt(header.Height)
	return stateDb, header, err
}

func (k *KaiAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return k.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := k.HeaderByHash(ctx, hash)
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && k.kaiService.blockchain.DB().ReadCanonicalHash(header.Height) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := k.kaiService.BlockChain().StateAt(header.Height)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}
