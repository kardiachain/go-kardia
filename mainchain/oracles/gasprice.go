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

package oracles

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

const sampleNumber = 3 // Number of transactions sampled in a block

var DefaultMaxPrice = big.NewInt(500 * configs.OXY)

type Config struct {
	Blocks     int
	Percentile int
	Default    *big.Int `toml:",omitempty"`
	MaxPrice   *big.Int `toml:",omitempty"`
}

func DefaultOracleConfig() *Config {
	return &Config{
		Blocks:     10,
		Percentile: 5,
		Default:    big.NewInt(1 * configs.OXY),
		MaxPrice:   DefaultMaxPrice,
	}
}

// OracleBackend includes all necessary background APIs for oracle.
type OracleBackend interface {
	HeaderByHeight(ctx context.Context, height rpc.BlockHeight) *types.Header
	BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block
}

// Oracle recommends gas prices based on the content of recent
// blocks. Suitable for both light and full clients.
type Oracle struct {
	backend   OracleBackend
	lastHead  common.Hash
	lastPrice *big.Int
	maxPrice  *big.Int
	cacheLock sync.RWMutex
	fetchLock sync.Mutex

	checkBlocks int
	percentile  int
}

// NewGasPriceOracle returns a new gasprice oracle which can recommend suitable
// oracles for newly created transaction.
func NewGasPriceOracle(backend OracleBackend, params *Config) *Oracle {
	blocks := params.Blocks
	if blocks < 1 {
		blocks = 1
		log.Warn("Sanitizing invalid gasprice oracle sample blocks", "provided", params.Blocks, "updated", blocks)
	}
	percent := params.Percentile
	if percent < 0 {
		percent = 0
		log.Warn("Sanitizing invalid gasprice oracle sample percentile", "provided", params.Percentile, "updated", percent)
	}
	if percent > 100 {
		percent = 100
		log.Warn("Sanitizing invalid gasprice oracle sample percentile", "provided", params.Percentile, "updated", percent)
	}
	maxPrice := params.MaxPrice
	if maxPrice == nil || maxPrice.Int64() <= 0 {
		maxPrice = DefaultMaxPrice
		log.Warn("Sanitizing invalid gasprice oracle price cap", "provided", params.MaxPrice, "updated", maxPrice)
	}
	return &Oracle{
		backend:     backend,
		lastPrice:   params.Default,
		maxPrice:    maxPrice,
		checkBlocks: blocks,
		percentile:  percent,
	}
}

// SuggestPrice returns a gasprice so that newly created transaction can
// have a very high chance to be included in the following blocks.
func (gpo *Oracle) SuggestPrice(ctx context.Context) (*big.Int, error) {
	head := gpo.backend.HeaderByHeight(ctx, rpc.LatestBlockHeight)
	headHash := head.Hash()

	// If the latest gasprice is still available, return it.
	gpo.cacheLock.RLock()
	lastHead, lastPrice := gpo.lastHead, gpo.lastPrice
	gpo.cacheLock.RUnlock()
	if headHash == lastHead {
		return lastPrice, nil
	}
	gpo.fetchLock.Lock()
	defer gpo.fetchLock.Unlock()

	// Try checking the cache again, maybe the last fetch fetched what we need
	gpo.cacheLock.RLock()
	lastHead, lastPrice = gpo.lastHead, gpo.lastPrice
	gpo.cacheLock.RUnlock()
	if headHash == lastHead {
		return lastPrice, nil
	}
	var (
		sent, exp int
		height    = head.Height
		result    = make(chan getBlockPricesResult, gpo.checkBlocks)
		quit      = make(chan struct{})
		txPrices  []*big.Int
	)
	for sent < gpo.checkBlocks && height > 0 {
		go gpo.getBlockPrices(ctx, types.HomesteadSigner{}, height, sampleNumber, result, quit)
		sent++
		exp++
		height--
	}
	for exp > 0 {
		res := <-result
		if res.err != nil {
			close(quit)
			return lastPrice, res.err
		}
		exp--
		// Nothing returned. There are two special cases here:
		// - The block is empty
		// - All the transactions included are sent by the miner itself.
		// In these cases, use the latest calculated price for samping.
		if len(res.prices) == 0 {
			res.prices = []*big.Int{lastPrice}
		}
		// Besides, in order to collect enough data for sampling, if nothing
		// meaningful returned, try to query more blocks. But the maximum
		// is 2*checkBlocks.
		if len(res.prices) == 1 && len(txPrices)+1+exp < gpo.checkBlocks*2 && height > 0 {
			go gpo.getBlockPrices(ctx, types.HomesteadSigner{}, height, sampleNumber, result, quit)
			sent++
			exp++
			height--
		}
		txPrices = append(txPrices, res.prices...)
	}
	price := lastPrice
	if len(txPrices) > 0 {
		sort.Sort(bigIntArray(txPrices))
		price = txPrices[(len(txPrices)-1)*gpo.percentile/100]
	}
	if price.Cmp(gpo.maxPrice) > 0 {
		price = new(big.Int).Set(gpo.maxPrice)
	}
	gpo.cacheLock.Lock()
	gpo.lastHead = headHash
	gpo.lastPrice = price
	gpo.cacheLock.Unlock()
	return price, nil
}

type getBlockPricesResult struct {
	prices []*big.Int
	err    error
}

type transactionsByGasPrice []*types.Transaction

func (t transactionsByGasPrice) Len() int           { return len(t) }
func (t transactionsByGasPrice) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t transactionsByGasPrice) Less(i, j int) bool { return t[i].GasPriceCmp(t[j]) < 0 }

// getBlockPrices calculates the lowest transaction gas price in a given block
// and sends it to the result channel. If the block is empty or all transactions
// are sent by the miner itself(it doesn't make any sense to include this kind of
// transaction prices for sampling), nil gasprice is returned.
func (gpo *Oracle) getBlockPrices(ctx context.Context, signer types.Signer, blockNum uint64, limit int, result chan getBlockPricesResult, quit chan struct{}) {
	block := gpo.backend.BlockByHeight(ctx, rpc.BlockHeight(blockNum))
	if block == nil {
		select {
		case result <- getBlockPricesResult{nil, fmt.Errorf("failed to get block %v", rpc.BlockHeight(blockNum))}:
		case <-quit:
		}
		return
	}
	blockTxs := block.Transactions()
	txs := make([]*types.Transaction, len(blockTxs))
	copy(txs, blockTxs)
	sort.Sort(transactionsByGasPrice(txs))

	var prices []*big.Int
	for _, tx := range txs {
		if tx.GasPriceIntCmp(common.Big1) <= 0 {
			continue
		}
		_, err := types.Sender(signer, tx)
		if err == nil {
			prices = append(prices, tx.GasPrice())
			if len(prices) >= limit {
				break
			}
		}
	}
	select {
	case result <- getBlockPricesResult{prices, nil}:
	case <-quit:
	}
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
