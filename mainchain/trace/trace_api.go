package trace

import (
	"context"
	"encoding/json"

	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"

	jsoniter "github.com/json-iterator/go"
)

// TraceAPI RPC interface into tracing API
type TraceAPI interface {
	// Ad-hoc (see ./trace_adhoc.go)
	ReplayBlockTransactions(ctx context.Context, blockNr rpc.BlockHeightOrHash, traceTypes []string) ([]*TraceCallResult, error)
	ReplayTransaction(ctx context.Context, txHash common.Hash, traceTypes []string) (*TraceCallResult, error)
	Call(ctx context.Context, call kaiapi.TransactionArgs, types []string, blockNr *rpc.BlockHeightOrHash) (*TraceCallResult, error)
	CallMany(ctx context.Context, calls json.RawMessage, blockNr *rpc.BlockHeightOrHash) ([]*TraceCallResult, error)
	RawTransaction(ctx context.Context, txHash common.Hash, traceTypes []string) ([]interface{}, error)

	// Filtering (see ./trace_filtering.go)
	Transaction(ctx context.Context, txHash common.Hash) (ParityTraces, error)
	Get(ctx context.Context, txHash common.Hash, txIndicies []common.Uint64) (*ParityTrace, error)
	Block(ctx context.Context, blockNr rpc.BlockHeight) (ParityTraces, error)
	Filter(ctx context.Context, req TraceFilterRequest, stream *jsoniter.Stream) error
}

// TraceAPIImpl is implementation of the TraceAPI interface based on remote Db access
type TraceAPIImpl struct {
	backend       Backend
	kv            kvstore.RoDB
	maxTraces     uint64
	gasCap        uint64
	compatibility bool // Bug-for-bug compatibility with OpenEthereum
}

// NewTraceAPI returns NewTraceAPI instance
func NewTraceAPI(backend Backend, kv kvstore.RoDB, cfg *node.Config) *TraceAPIImpl {
	return &TraceAPIImpl{
		backend:       backend,
		kv:            kv,
		maxTraces:     cfg.TraceAPIConfig.MaxTraces,
		gasCap:        cfg.TraceAPIConfig.Gascap,
		compatibility: cfg.TraceAPIConfig.TraceCompatibility,
	}
}
