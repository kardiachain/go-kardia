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

package filters

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/kai/storage/kvstore"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

type testBackend struct {
	mux        *event.TypeMux
	db         types.StoreDB
	sections   uint64
	txFeed     event.Feed
	logsFeed   event.Feed
	rmLogsFeed event.Feed
	chainFeed  event.Feed
}

func (b *testBackend) ChainDb() types.StoreDB {
	return b.db
}

func (b *testBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) *types.Header {
	var (
		hash common.Hash
		num  uint64
	)
	if blockNr == rpc.LatestBlockNumber {
		hash = b.db.ReadHeadBlockHash()
		number := b.db.ReadHeaderHeight(hash)
		if number == nil {
			return nil
		}
		num = *number
	} else {
		num = uint64(blockNr)
		hash = b.db.ReadCanonicalHash(num)
	}
	return b.db.ReadHeader(num)
}

func (b *testBackend) HeaderByHash(ctx context.Context, hash common.Hash) *types.Header {
	number := b.db.ReadHeaderHeight(hash)
	if number == nil {
		return nil
	}
	return b.db.ReadHeader(*number)
}

func (b *testBackend) BlockInfoByBlockHash(ctx context.Context, hash common.Hash) *types.BlockInfo {
	height := b.db.ReadHeaderHeight(hash)
	if height == nil {
		return nil
	}
	return b.db.ReadBlockInfo(hash, *height)
}

func (b *testBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	if blockInfo := b.BlockInfoByBlockHash(ctx, hash); blockInfo != nil {
		return blockInfo.Receipts, nil
	}
	return nil, nil
}

func (b *testBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	number := b.db.ReadHeaderHeight(hash)
	if number == nil {
		return nil, nil
	}
	receipts, _ := b.GetReceipts(ctx, hash)
	if receipts == nil {
		return nil, nil
	}

	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *testBackend) SubscribeNewTxsEvent(ch chan<- events.NewTxsEvent) event.Subscription {
	return b.txFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.logsFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeChainEvent(ch chan<- events.ChainEvent) event.Subscription {
	return b.chainFeed.Subscribe(ch)
}

func (b *testBackend) BloomStatus() (uint64, uint64) {
	return configs.BloomBitsBlocks, b.sections
}

func (b *testBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	requests := make(chan chan *bloombits.Retrieval)

	go session.Multiplex(16, 0, requests)
	go func() {
		for {
			// Wait for a service request or a shutdown
			select {
			case <-ctx.Done():
				return

			case request := <-requests:
				task := <-request

				task.Bitsets = make([][]byte, len(task.Sections))
				for i, section := range task.Sections {
					if rand.Int()%4 != 0 { // Handle occasional missing deliveries
						head := b.db.ReadCanonicalHash((section+1)*configs.BloomBitsBlocks - 1)
						task.Bitsets[i], _ = kvstore.ReadBloomBits(b.db.DB(), task.Bit, section, head)
					}
				}
				request <- task
			}
		}
	}()
}

// TestBlockSubscription tests if a block subscription returns block hashes for posted chain events.
// It creates multiple subscriptions:
// - one at the start and should receive all posted chain events and a second (blockHashes)1
// - one that is created after a cutoff moment and uninstalled after a second cutoff moment (blockHashes[cutoff1:cutoff2])
// - one that is created after the second cutoff moment (blockHashes[cutoff2:])
//func TestBlockSubscription(t *testing.T) {
//	t.Parallel()
//
//	var (
//		db          = storage.NewMemoryDatabase()
//		backend     = &testBackend{db: db}
//		api         = NewPublicFilterAPI(backend)
//		genesis     = new(genesis.Genesis).MustCommit(db)
//		chain, _    = events.GenerateChain(configs.TestChainConfig, genesis, ethash.NewFaker(), db, 10, func(i int, gen *events.BlockGen) {})
//		chainEvents []events.ChainEvent
//	)
//
//	for _, blk := range chain {
//		chainEvents = append(chainEvents, events.ChainEvent{Hash: blk.Hash(), Block: blk})
//	}
//
//	chan0 := make(chan *types.Header)
//	sub0 := api.events.SubscribeNewHeads(chan0)
//	chan1 := make(chan *types.Header)
//	sub1 := api.events.SubscribeNewHeads(chan1)
//
//	go func() { // simulate client
//		i1, i2 := 0, 0
//		for i1 != len(chainEvents) || i2 != len(chainEvents) {
//			select {
//			case header := <-chan0:
//				if chainEvents[i1].Hash != header.Hash() {
//					t.Errorf("sub0 received invalid hash on index %d, want %x, got %x", i1, chainEvents[i1].Hash, header.Hash())
//				}
//				i1++
//			case header := <-chan1:
//				if chainEvents[i2].Hash != header.Hash() {
//					t.Errorf("sub1 received invalid hash on index %d, want %x, got %x", i2, chainEvents[i2].Hash, header.Hash())
//				}
//				i2++
//			}
//		}
//
//		sub0.Unsubscribe()
//		sub1.Unsubscribe()
//	}()
//
//	time.Sleep(1 * time.Second)
//	for _, e := range chainEvents {
//		backend.chainFeed.Send(e)
//	}
//
//	<-sub0.Err()
//	<-sub1.Err()
//}

// TestLogFilterCreation test whether a given filter criteria makes sense.
// If not it must return an error.
func TestLogFilterCreation(t *testing.T) {
	var (
		db      = storage.NewMemoryDatabase()
		backend = &testBackend{db: db}
		api     = NewPublicFilterAPI(backend)

		testCases = []struct {
			crit    FilterCriteria
			success bool
		}{
			// defaults
			{FilterCriteria{}, true},
			// valid block number range
			{FilterCriteria{FromBlock: 1, ToBlock: 2}, true},
			// "mined" block range to pending
			{FilterCriteria{FromBlock: 1, ToBlock: rpc.LatestBlockNumber.Uint64()}, true},
			// new mined and pending blocks
			{FilterCriteria{FromBlock: rpc.LatestBlockNumber.Uint64(), ToBlock: rpc.PendingBlockNumber.Uint64()}, true},
			// from block "higher" than to block
			{FilterCriteria{FromBlock: 2, ToBlock: 1}, false},
			// from block "higher" than to block
			{FilterCriteria{FromBlock: rpc.LatestBlockNumber.Uint64(), ToBlock: 100}, false},
			// from block "higher" than to block
			{FilterCriteria{FromBlock: rpc.PendingBlockNumber.Uint64(), ToBlock: 100}, false},
			// from block "higher" than to block
			{FilterCriteria{FromBlock: rpc.PendingBlockNumber.Uint64(), ToBlock: rpc.LatestBlockNumber.Uint64()}, true},
		}
	)

	for i, test := range testCases {
		_, err := api.NewFilter(test.crit)
		if test.success && err != nil {
			t.Errorf("expected filter creation for case %d to success, got %v", i, err)
		}
		if !test.success && err == nil {
			t.Errorf("expected testcase %d to fail with an error", i)
		}
	}
}

// TestInvalidLogFilterCreation tests whether invalid filter log criteria results in an error
// when the filter is created.
func TestInvalidLogFilterCreation(t *testing.T) {
	t.Parallel()

	var (
		db      = storage.NewMemoryDatabase()
		backend = &testBackend{db: db}
		api     = NewPublicFilterAPI(backend)
	)

	// different situations where log filter creation should fail.
	// Reason: fromBlock > toBlock
	testCases := []FilterCriteria{
		0: {FromBlock: rpc.PendingBlockNumber.Uint64(), ToBlock: 100},
		1: {FromBlock: rpc.LatestBlockNumber.Uint64(), ToBlock: 100},
	}

	for i, test := range testCases {
		if _, err := api.NewFilter(test); err == nil {
			t.Errorf("Expected NewFilter for case #%d to fail", i)
		}
	}
}

func TestInvalidGetLogsRequest(t *testing.T) {
	var (
		db        = storage.NewMemoryDatabase()
		backend   = &testBackend{db: db}
		api       = NewPublicFilterAPI(backend)
		blockHash = common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	)

	// Reason: Cannot specify both BlockHash and FromBlock/ToBlock)
	testCases := []FilterCriteria{
		0: {BlockHash: &blockHash, FromBlock: 100},
		1: {BlockHash: &blockHash, ToBlock: 500},
		2: {BlockHash: &blockHash, FromBlock: rpc.LatestBlockNumber.Uint64()},
	}

	for i, test := range testCases {
		if _, err := api.GetLogs(context.Background(), test); err == nil {
			t.Errorf("Expected Logs for case #%d to fail", i)
		}
	}
}

// TestLogFilter tests whether log filters match the correct logs that are posted to the event feed.
func TestLogFilter(t *testing.T) {
	t.Parallel()

	var (
		db      = storage.NewMemoryDatabase()
		backend = &testBackend{db: db}
		api     = NewPublicFilterAPI(backend)

		firstAddr      = common.HexToAddress("0x1111111111111111111111111111111111111111")
		secondAddr     = common.HexToAddress("0x2222222222222222222222222222222222222222")
		thirdAddress   = common.HexToAddress("0x3333333333333333333333333333333333333333")
		notUsedAddress = common.HexToAddress("0x9999999999999999999999999999999999999999")
		firstTopic     = common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
		secondTopic    = common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
		notUsedTopic   = common.HexToHash("0x9999999999999999999999999999999999999999999999999999999999999999")

		// posted twice, once as regular logs and once as pending logs.
		allLogs = []*types.Log{
			{Address: firstAddr},
			{Address: firstAddr, Topics: []common.Hash{firstTopic}, BlockHeight: 1},
			{Address: secondAddr, Topics: []common.Hash{firstTopic}, BlockHeight: 1},
			{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockHeight: 2},
			{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockHeight: 3},
		}

		testCases = []struct {
			crit     FilterCriteria
			expected []*types.Log
			id       rpc.ID
		}{
			// match all
			0: {FilterCriteria{}, allLogs, ""},
			// match none due to no matching addresses
			1: {FilterCriteria{Addresses: []common.Address{{}, notUsedAddress}, Topics: [][]common.Hash{nil}}, []*types.Log{}, ""},
			// match logs based on addresses, ignore topics
			2: {FilterCriteria{Addresses: []common.Address{firstAddr}}, allLogs[:2], ""},
			// match none due to no matching topics (match with address)
			3: {FilterCriteria{Addresses: []common.Address{secondAddr}, Topics: [][]common.Hash{{notUsedTopic}}}, []*types.Log{}, ""},
			// match logs based on addresses and topics
			4: {FilterCriteria{Addresses: []common.Address{thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, allLogs[3:5], ""},
			// match logs based on multiple addresses and "or" topics
			5: {FilterCriteria{Addresses: []common.Address{secondAddr, thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, allLogs[2:5], ""},
			// all "mined" logs with block num >= 2
			6: {FilterCriteria{FromBlock: 2, ToBlock: rpc.LatestBlockNumber.Uint64()}, allLogs[3:], ""},
			// all "mined" logs
			7: {FilterCriteria{ToBlock: rpc.LatestBlockNumber.Uint64()}, allLogs, ""},
			// all "mined" logs with 1>= block num <=2 and topic secondTopic
			8: {FilterCriteria{FromBlock: 1, ToBlock: 2, Topics: [][]common.Hash{{secondTopic}}}, allLogs[3:4], ""},
			// all "mined" and pending logs with topic firstTopic
			9: {FilterCriteria{FromBlock: rpc.LatestBlockNumber.Uint64(), ToBlock: rpc.PendingBlockNumber.Uint64(), Topics: [][]common.Hash{{firstTopic}}}, []*types.Log{}, ""},
			// match all logs due to wildcard topic
			10: {FilterCriteria{Topics: [][]common.Hash{nil}}, allLogs[1:], ""},
		}
	)

	// create all filters
	for i := range testCases {
		testCases[i].id, _ = api.NewFilter(testCases[i].crit)
	}

	// raise events
	time.Sleep(1 * time.Second)
	if nsend := backend.logsFeed.Send(allLogs); nsend == 0 {
		t.Fatal("Logs event not delivered")
	}

	for i, tt := range testCases {
		var fetched []*types.Log
		timeout := time.Now().Add(1 * time.Second)
		for { // fetch all expected logs
			results, err := api.GetFilterChanges(tt.id)
			if err != nil {
				t.Fatalf("Unable to fetch logs: %v", err)
			}

			fetched = append(fetched, results.([]*types.Log)...)
			if len(fetched) >= len(tt.expected) {
				break
			}
			// check timeout
			if time.Now().After(timeout) {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		if len(fetched) != len(tt.expected) {
			t.Errorf("invalid number of logs for case %d, want %d log(s), got %d", i, len(tt.expected), len(fetched))
			return
		}

		for l := range fetched {
			if fetched[l].Removed {
				t.Errorf("expected log not to be removed for log %d in case %d", l, i)
			}
			if !reflect.DeepEqual(fetched[l], tt.expected[l]) {
				t.Errorf("invalid log on index %d for case %d", l, i)
			}
		}
	}
}
