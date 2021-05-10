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

package gasprice

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/mainchain/blockchain"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/core"
	"github.com/kardiachain/go-kardia/core/rawdb"
	"github.com/kardiachain/go-kardia/core/vm"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type testBackend struct {
	chain *blockchain.BlockChain
}

func (b *testBackend) HeaderByHeight(ctx context.Context, height rpc.BlockHeight) (*types.Header, error) {
	if height == rpc.LatestBlockHeight {
		return b.chain.CurrentBlock().Header(), nil
	}
	return b.chain.GetHeaderByHeight(height.Uint64()), nil
}

func (b *testBackend) BlockByHeight(ctx context.Context, height rpc.BlockHeight) (*types.Block, error) {
	if height == rpc.LatestBlockHeight {
		return b.chain.CurrentBlock(), nil
	}
	return b.chain.GetBlockByHeight(height.Uint64()), nil
}

func (b *testBackend) ChainConfig() *configs.ChainConfig {
	return b.chain.Config()
}

func newTestBackend(t *testing.T) *testBackend {
	var (
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		gspec  = &core.Genesis{
			Config: configs.TestChainConfig,
			Alloc:  core.GenesisAlloc{addr: {Balance: big.NewInt(math.MaxInt64)}},
		}
		signer = types.LatestSigner(gspec.Config)
	)
	db := rawdb.NewMemoryDatabase()
	genesis, _ := gspec.Commit(db)

	// Construct testing chain
	diskdb := rawdb.NewMemoryDatabase()
	gspec.Commit(diskdb)
	chain, err := core.NewBlockChain(diskdb, nil, configs.TestChainConfig, engine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create local chain, %v", err)
	}
	chain.InsertChain(blocks)
	return &testBackend{chain: chain}
}

func (b *testBackend) CurrentHeader() *types.Header {
	return b.chain.CurrentHeader()
}

func (b *testBackend) GetBlockByHeight(number uint64) *types.Block {
	return b.chain.GetBlockByHeight(number)
}

func TestSuggestPrice(t *testing.T) {
	config := Config{
		Blocks:     3,
		Percentile: 60,
		Default:    big.NewInt(configs.OXY),
	}
	backend := newTestBackend(t)
	oracle := NewOracle(backend, config)

	// The gas price sampled is: 32G, 31G, 30G, 29G, 28G, 27G
	got, err := oracle.SuggestPrice(context.Background())
	if err != nil {
		t.Fatalf("Failed to retrieve recommended gas price: %v", err)
	}
	expect := big.NewInt(configs.OXY * int64(30))
	if got.Cmp(expect) != 0 {
		t.Fatalf("Gas price mismatch, want %d, got %d", expect, got)
	}
}
