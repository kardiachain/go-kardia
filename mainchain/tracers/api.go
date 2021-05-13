package tracers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	mkvm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*kvm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}

// txTraceContext is the contextual infos about a transaction before it gets run.
type txTraceContext struct {
	index int         // Index of the transaction within the block
	hash  common.Hash // Hash of the transaction
	block common.Hash // Hash of the block containing the transaction
}

const (
	// defaultTraceTimeout is the amount of time a single transaction can execute
	// by default before being forcefully aborted.
	defaultTraceTimeout = 5 * time.Second

	// defaultTraceReexec is the number of blocks the tracer is willing to go back
	// and reexecute to produce missing historical state necessary to run a specific
	// trace.
	defaultTraceReexec = uint64(128)
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	HeaderByHash(ctx context.Context, hash common.Hash) *types.Header
	HeaderByHeight(ctx context.Context, height rpc.BlockHeight) *types.Header
	BlockByHash(ctx context.Context, hash common.Hash) *types.Block
	BlockByHeight(ctx context.Context, height rpc.BlockHeight) *types.Block
	BlockByHeightOrHash(ctx context.Context, blockHeightOrHash rpc.BlockHeightOrHash) (*types.Block, error)
	GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64)
	ChainDb() types.StoreDB
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.Context, *state.StateDB, error)
}

// VerifyKaiAPI provides APIs to access Kai full node-related information.
type TracerAPI struct {
	b Backend
}

// NewTracerAPI creates a new API definition for the tracing methods of the KardiaChain service.
func NewTracerAPI(backend Backend) *TracerAPI {
	return &TracerAPI{b: backend}
}

// TraceTransaction returns the structured logs created during the execution of EVM
// and returns them as a JSON object.
func (t *TracerAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *TraceConfig) (interface{}, error) {
	tx, blockHash, blockHeight, index := t.b.GetTransaction(ctx, hash)
	if tx == nil {
		return nil, errors.New("tx for hash not found")
	}
	// It shouldn't happen in practice.
	if blockHeight == 0 {
		return nil, errors.New("genesis is not traceable")
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	height := rpc.BlockHeight(blockHeight)
	block, err := t.b.BlockByHeightOrHash(ctx, rpc.BlockHeightOrHash{
		BlockHeight:      &height,
		BlockHash:        &blockHash,
		RequireCanonical: false,
	})
	if err != nil {
		return nil, err
	}
	msg, vmctx, statedb, err := t.b.StateAtTransaction(ctx, block, int(index), reexec)
	if err != nil {
		return nil, err
	}
	txctx := &txTraceContext{
		index: int(index),
		hash:  hash,
		block: blockHash,
	}
	return t.traceTx(ctx, msg, txctx, vmctx, statedb, config)
}

// traceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (t *TracerAPI) traceTx(ctx context.Context, message blockchain.Message, txctx *txTraceContext, vmctx kvm.Context, statedb *state.StateDB, config *TraceConfig) (interface{}, error) {
	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer    kvm.Tracer
		err       error
		txContext = mkvm.NewKVMTxContext(message.From(), message.GasPrice())
	)
	switch {
	case config != nil && config.Tracer != nil:
		// Define a meaningful timeout of a single transaction trace
		timeout := defaultTraceTimeout
		if config.Timeout != nil {
			if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
				return nil, err
			}
		}
		// Construct the JavaScript tracer to execute with
		if tracer, err = New(*config.Tracer, txContext); err != nil {
			return nil, err
		}
		// Handle timeouts and RPC cancellations
		deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
		go func() {
			<-deadlineCtx.Done()
			if deadlineCtx.Err() == context.DeadlineExceeded {
				tracer.(*Tracer).Stop(errors.New("execution timeout"))
			}
		}()
		defer cancel()

	case config == nil:
		tracer = kvm.NewStructLogger(nil)

	default:
		tracer = kvm.NewStructLogger(config.LogConfig)
	}
	// Run the transaction with tracing enabled.
	vmenv := kvm.NewKVM(vmctx, txContext, statedb, kvm.Config{Debug: true, Tracer: tracer})

	// Call Prepare to clear out the statedb access list
	statedb.Prepare(txctx.hash, txctx.block, txctx.index)

	result, err := blockchain.ApplyMessage(vmenv, message, new(types.GasPool).AddGas(message.Gas()))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	// Depending on the tracer type, format and return the output.
	switch tracer := tracer.(type) {
	case *kvm.StructLogger:
		return &kvm.ExecutionResult{
			UsedGas:    result.UsedGas,
			Err:        result.Err,
			ReturnData: common.CopyBytes(result.ReturnData),
		}, nil

	case *Tracer:
		return tracer.GetResult()

	default:
		panic(fmt.Sprintf("bad tracer type %T", tracer))
	}
}
