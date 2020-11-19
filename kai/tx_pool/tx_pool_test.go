// Package tx_pool
package tx_pool

import (
	"math/big"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/blockchain"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/types"
)

func TestNewTxPool(t *testing.T) {
	type args struct {
		config   TxPoolConfig
		chainCfg *configs.ChainConfig
		chain    blockchain.Blockchain
	}
	tests := []struct {
		name string
		args args
		want *txPool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTxPool(tt.args.config, tt.args.chainCfg, tt.args.chain); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTxPool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_AddLocal(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		tx *types.Transaction
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if err := pool.AddLocal(tt.args.tx); (err != nil) != tt.wantErr {
				t.Errorf("AddLocal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_txPool_AddLocals(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		txs []*types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.AddLocals(tt.args.txs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddLocals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_AddRemote(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		tx *types.Transaction
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if err := pool.AddRemote(tt.args.tx); (err != nil) != tt.wantErr {
				t.Errorf("AddRemote() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_txPool_AddRemotes(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		txs []*types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.AddRemotes(tt.args.txs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddRemotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_AddRemotesSync(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		txs []*types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.AddRemotesSync(tt.args.txs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddRemotesSync() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Content(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   map[common.Address]types.Transactions
		want1  map[common.Address]types.Transactions
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, got1 := pool.Content()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Content() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Content() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_txPool_EnableTxsAvailable(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_GasPrice(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   *big.Int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.GasPrice(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GasPrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Get(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		hash common.Hash
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *types.Transaction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.Get(tt.args.hash); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_GetBlockChain(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   blockchain.Blockchain
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.GetBlockChain(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBlockChain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_GetPendingData(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   []*types.Transaction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.GetPendingData(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPendingData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Locals(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   []common.Address
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.Locals(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Locals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Nonce(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		addr common.Address
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   uint64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.Nonce(tt.args.addr); got != tt.want {
				t.Errorf("Nonce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Pending(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[common.Address]types.Transactions
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, err := pool.Pending()
			if (err != nil) != tt.wantErr {
				t.Errorf("Pending() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Pending() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_PendingSize(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.PendingSize(); got != tt.want {
				t.Errorf("PendingSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_ProposeTransactions(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   []*types.Transaction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.ProposeTransactions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ProposeTransactions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_SetGasPrice(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		price *big.Int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_State(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   *state.StateDB
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.State(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("State() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Stats(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   int
		want1  int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, got1 := pool.Stats()
			if got != tt.want {
				t.Errorf("Stats() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Stats() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_txPool_Status(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		hashes []common.Hash
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []TxStatus
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.Status(tt.args.hashes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_Stop(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_SubscribeNewTxsEvent(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		ch chan<- events.NewTxsEvent
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   event.Subscription
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.SubscribeNewTxsEvent(tt.args.ch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SubscribeNewTxsEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_TxsAvailable(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   <-chan struct{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.TxsAvailable(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TxsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_add(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		tx    *types.Transaction
		local bool
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantReplaced bool
		wantErr      bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			gotReplaced, err := pool.add(tt.args.tx, tt.args.local)
			if (err != nil) != tt.wantErr {
				t.Errorf("add() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotReplaced != tt.wantReplaced {
				t.Errorf("add() gotReplaced = %v, want %v", gotReplaced, tt.wantReplaced)
			}
		})
	}
}

func Test_txPool_addTxs(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		txs   []*types.Transaction
		local bool
		sync  bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.addTxs(tt.args.txs, tt.args.local, tt.args.sync); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addTxs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_addTxsLocked(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		txs   []*types.Transaction
		local bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []error
		want1  *accountSet
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, got1 := pool.addTxsLocked(tt.args.txs, tt.args.local)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addTxsLocked() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("addTxsLocked() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_txPool_demoteUnexecutables(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_enqueueTx(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		hash common.Hash
		tx   *types.Transaction
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, err := pool.enqueueTx(tt.args.hash, tt.args.tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("enqueueTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("enqueueTx() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_journalTx(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		from common.Address
		tx   *types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_local(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   map[common.Address]types.Transactions
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.local(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("local() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_loop(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_notifyTxsAvailable(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_promoteExecutables(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		accounts []common.Address
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*types.Transaction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.promoteExecutables(tt.args.accounts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("promoteExecutables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_promoteTx(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		addr common.Address
		hash common.Hash
		tx   *types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.promoteTx(tt.args.addr, tt.args.hash, tt.args.tx); got != tt.want {
				t.Errorf("promoteTx() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_queueTxEvent(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		tx *types.Transaction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_removeTx(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		hash       common.Hash
		outofbound bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_requestPromoteExecutables(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		set *accountSet
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   chan struct{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.requestPromoteExecutables(tt.args.set); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("requestPromoteExecutables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_requestReset(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		oldHead *types.Header
		newHead *types.Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   chan struct{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if got := pool.requestReset(tt.args.oldHead, tt.args.newHead); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("requestReset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txPool_reset(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		oldHead *types.Header
		newHead *types.Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_runReorg(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		done          chan struct{}
		reset         *txPoolResetRequest
		dirtyAccounts *accountSet
		eventsPool    map[common.Address]*txSortedMap
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_scheduleReorgLoop(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_stats(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   int
		want1  int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			got, got1 := pool.stats()
			if got != tt.want {
				t.Errorf("stats() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("stats() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_txPool_truncatePending(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_truncateQueue(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
		})
	}
}

func Test_txPool_validateTx(t *testing.T) {
	type fields struct {
		config               TxPoolConfig
		chainCfg             *configs.ChainConfig
		chain                blockchain.Blockchain
		gasPrice             *big.Int
		txFeed               event.Feed
		scope                event.SubscriptionScope
		signer               types.Signer
		mu                   sync.RWMutex
		currentState         *state.StateDB
		pendingNonces        *txNoncer
		currentMaxGas        uint64
		locals               *accountSet
		journal              *txJournal
		pending              map[common.Address]*txList
		queue                map[common.Address]*txList
		beats                map[common.Address]time.Time
		all                  *txLookup
		priced               *txPricedList
		chainHeadCh          chan events.ChainHeadEvent
		chainHeadSub         event.Subscription
		reqResetCh           chan *txPoolResetRequest
		reqPromoteCh         chan *accountSet
		queueTxEventCh       chan *types.Transaction
		reorgDoneCh          chan chan struct{}
		reorgShutdownCh      chan struct{}
		wg                   sync.WaitGroup
		notifiedTxsAvailable bool
		txsAvailable         chan struct{}
	}
	type args struct {
		tx    *types.Transaction
		local bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &txPool{
				config:               tt.fields.config,
				chainCfg:             tt.fields.chainCfg,
				chain:                tt.fields.chain,
				gasPrice:             tt.fields.gasPrice,
				txFeed:               tt.fields.txFeed,
				scope:                tt.fields.scope,
				signer:               tt.fields.signer,
				mu:                   tt.fields.mu,
				currentState:         tt.fields.currentState,
				pendingNonces:        tt.fields.pendingNonces,
				currentMaxGas:        tt.fields.currentMaxGas,
				locals:               tt.fields.locals,
				journal:              tt.fields.journal,
				pending:              tt.fields.pending,
				queue:                tt.fields.queue,
				beats:                tt.fields.beats,
				all:                  tt.fields.all,
				priced:               tt.fields.priced,
				chainHeadCh:          tt.fields.chainHeadCh,
				chainHeadSub:         tt.fields.chainHeadSub,
				reqResetCh:           tt.fields.reqResetCh,
				reqPromoteCh:         tt.fields.reqPromoteCh,
				queueTxEventCh:       tt.fields.queueTxEventCh,
				reorgDoneCh:          tt.fields.reorgDoneCh,
				reorgShutdownCh:      tt.fields.reorgShutdownCh,
				wg:                   tt.fields.wg,
				notifiedTxsAvailable: tt.fields.notifiedTxsAvailable,
				txsAvailable:         tt.fields.txsAvailable,
			}
			if err := pool.validateTx(tt.args.tx, tt.args.local); (err != nil) != tt.wantErr {
				t.Errorf("validateTx() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
