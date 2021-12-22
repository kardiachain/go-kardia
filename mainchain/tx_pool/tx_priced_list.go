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
package tx_pool

import (
	"container/heap"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// priceHeap is a heap.Interface implementation over transactions for retrieving
// price-sorted transactions to discard when the pool fills up.
type priceHeap []*types.Transaction

func (h priceHeap) Len() int      { return len(h) }
func (h priceHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h priceHeap) Less(i, j int) bool {
	// Sort primarily by price, returning the cheaper one
	switch h[i].GasPriceCmp(h[j]) {
	case -1:
		return true
	case 1:
		return false
	}
	// If the prices match, stabilize via nonces (high nonce is worse)
	return h[i].Nonce() > h[j].Nonce()
}

func (h *priceHeap) Push(x interface{}) {
	*h = append(*h, x.(*types.Transaction))
}

func (h *priceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// txPricedList is a price-sorted heap to allow operating on transactions pool
// contents in a price-incrementing way. It's built opon the all transactions
// in txpool but only interested in the remote part. It means only remote transactions
// will be considered for tracking, sorting, eviction, etc.
type txPricedList struct {
	all      *txLookup  // Pointer to the map of all transactions
	remotes  *priceHeap // Heap of prices of all the stored **remote** transactions
	stales   int64      // Number of stale price points to (re-heap trigger)
	reheapMu sync.Mutex // Mutex asserts that only one routine is reheaping the list
}

// newTxPricedList creates a new price-sorted transaction heap.
func newTxPricedList(all *txLookup) *txPricedList {
	return &txPricedList{
		all:     all,
		remotes: new(priceHeap),
	}
}

// Put inserts a new transaction into the heap.
func (l *txPricedList) Put(tx *types.Transaction, local bool) {
	if local {
		return
	}
	heap.Push(l.remotes, tx)
}

// Removed notifies the prices transaction list that an old transaction dropped
// from the pool. The list will just keep a counter of stale objects and update
// the heap if a large enough ratio of transactions go stale.
func (l *txPricedList) Removed(count int) {
	// Bump the stale counter, but exit if still too low (< 25%)
	stales := atomic.AddInt64(&l.stales, int64(count))
	if int(stales) <= len(*l.remotes)/4 {
		return
	}
	// Seems we've reached a critical number of stale transactions, reheap
	l.Reheap()
}

// Cap finds all the transactions below the given price threshold, drops them
// from the priced list and returns them for further removal from the entire pool.
//
// Note: only remote transactions will be considered for eviction.
func (l *txPricedList) Cap(threshold *big.Int) types.Transactions {
	drop := make(types.Transactions, 0, 128) // Remote underpriced transactions to drop
	for len(*l.remotes) > 0 {
		// Discard stale transactions if found during cleanup
		cheapest := (*l.remotes)[0]
		if l.all.GetRemote(cheapest.Hash()) == nil { // Removed or migrated
			heap.Pop(l.remotes)
			l.stales--
			continue
		}
		// Stop the discards if we've reached the threshold
		if cheapest.GasPriceIntCmp(threshold) >= 0 {
			break
		}
		heap.Pop(l.remotes)
		drop = append(drop, cheapest)
	}
	return drop
}

// Underpriced checks whether a transaction is cheaper than (or as cheap as) the
// lowest priced (remote) transaction currently being tracked.
func (l *txPricedList) Underpriced(tx *types.Transaction) bool {
	// Discard stale price points if found at the heap start
	for len(*l.remotes) > 0 {
		head := []*types.Transaction(*l.remotes)[0]
		if l.all.GetRemote(head.Hash()) == nil { // Removed or migrated
			atomic.AddInt64(&l.stales, -1)
			heap.Pop(l.remotes)
			continue
		}
		break
	}
	// Check if the transaction is underpriced or not
	if len(*l.remotes) == 0 {
		return false // There is no remote transaction at all.
	}
	// If the remote transaction is even cheaper than the
	// cheapest one tracked locally, reject it.
	cheapest := []*types.Transaction(*l.remotes)[0]
	return cheapest.GasPriceCmp(tx) >= 0
}

// Discard finds a number of most underpriced transactions, removes them from the
// priced list and returns them for further removal from the entire pool.
//
// Note local transaction won't be considered for eviction.
func (l *txPricedList) Discard(slots int, force bool) (types.Transactions, bool) {
	drop := make(types.Transactions, 0, slots) // Remote underpriced transactions to drop
	for len(*l.remotes) > 0 && slots > 0 {
		// Discard stale transactions if found during cleanup
		tx := heap.Pop(l.remotes).(*types.Transaction)
		if l.all.GetRemote(tx.Hash()) == nil { // Removed or migrated
			atomic.AddInt64(&l.stales, -1)
			continue
		}
		// Non stale transaction found, discard it
		drop = append(drop, tx)
		slots -= numSlots(tx)
	}
	// If we still can't make enough room for the new transaction
	if slots > 0 && !force {
		for _, tx := range drop {
			heap.Push(l.remotes, tx)
		}
		return nil, false
	}
	return drop, true
}

// Reheap forcibly rebuilds the heap based on the current remote transaction set.
func (l *txPricedList) Reheap() {
	l.reheapMu.Lock()
	defer l.reheapMu.Unlock()
	start := time.Now()
	atomic.StoreInt64(&l.stales, 0)
	reheap := make(priceHeap, 0, l.all.CountRemote())

	l.stales, l.remotes = 0, &reheap
	l.all.Range(func(hash common.Hash, tx *types.Transaction, local bool) bool {
		*l.remotes = append(*l.remotes, tx)
		return true
	}, false, true) // Only iterate remotes
	heap.Init(l.remotes)
	reheapTimer.Update(time.Since(start))
}
