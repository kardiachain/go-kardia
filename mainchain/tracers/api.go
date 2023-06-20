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

package tracers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/tracers/logger"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// blockByHeightAndHash is the wrapper of the chain access function offered by
// the backend. It will return an error if the block is not found.
//
// Note this function is friendly for the light client which can only retrieve the
// historical(before the CHT) header/block by number.
func (t *TracerAPI) blockByHeightAndHash(ctx context.Context, number rpc.BlockHeight, hash common.Hash) (*types.Block, error) {
	block, err := t.blockByHeight(ctx, number)
	if err != nil {
		return nil, err
	}
	if block.Hash() == hash {
		return block, nil
	}
	return t.blockByHash(ctx, hash)
}

// blockByHeight is the wrapper of the chain access function offered by the backend.
// It will return an error if the block is not found.
func (t *TracerAPI) blockByHeight(ctx context.Context, number rpc.BlockHeight) (*types.Block, error) {
	block := t.b.BlockByHeight(ctx, number)
	if block == nil {
		return nil, fmt.Errorf("block #%d not found", number)
	}
	return block, nil
}

// blockByHash is the wrapper of the chain access function offered by the backend.
// It will return an error if the block is not found.
func (t *TracerAPI) blockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	block := t.b.BlockByHash(ctx, hash)
	if block == nil {
		return nil, fmt.Errorf("block %s not found", hash.Hex())
	}
	return block, nil
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*logger.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}

// TraceCallConfig is the config for traceCall API. It holds one more
// field to override the state for tracing.
type TraceCallConfig struct {
	*logger.LogConfig
	Tracer         *string
	Timeout        *string
	Reexec         *uint64
	StateOverrides *kaiapi.StateOverride
}

// StdTraceConfig holds extra parameters to standard-json trace functions.
type StdTraceConfig struct {
	logger.LogConfig
	Reexec *uint64
	TxHash common.Hash
}

// // txTraceResult is the result of a single transaction trace.
// type txTraceResult struct {
// 	Result interface{} `json:"result,omitempty"` // Trace results produced by the tracer
// 	Error  string      `json:"error,omitempty"`  // Trace failure produced by the tracer
// }

// // blockTraceTask represents a single block trace task when an entire chain is
// // being traced.
// type blockTraceTask struct {
// 	statedb *state.StateDB   // Intermediate state prepped for tracing
// 	block   *types.Block     // Block to trace the transactions from
// 	rootref common.Hash      // Trie root reference held for this task
// 	results []*txTraceResult // Trace results procudes by the task
// }

// // blockTraceResult represets the results of tracing a single block when an entire
// // chain is being traced.
// type blockTraceResult struct {
// 	Block  common.Uint64    `json:"block"`  // Block number corresponding to this trace
// 	Hash   common.Hash      `json:"hash"`   // Block hash corresponding to this trace
// 	Traces []*txTraceResult `json:"traces"` // Trace results produced by the task
// }

// // txTraceTask represents a single transaction trace task when an entire block
// // is being traced.
// type txTraceTask struct {
// 	statedb *state.StateDB // Intermediate state prepped for tracing
// 	index   int            // Transaction offset in the block
// }

// // txTraceContext is the contextual infos about a transaction before it gets run.
// type txTraceContext struct {
// 	index int         // Index of the transaction within the block
// 	hash  common.Hash // Hash of the transaction
// 	block common.Hash // Hash of the block containing the transaction
// }

const (
	// defaultTraceTimeout is the amount of time a single transaction can execute
	// by default before being forcefully aborted.
	defaultTraceTimeout = 15 * time.Second

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
	GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64)
	Config() *configs.ChainConfig
	RPCGasCap() uint64
	ChainConfig() *configs.ChainConfig
	ChainDb() kaidb.Database
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.BlockContext, *state.StateDB, error)
}

// TracerAPI provides APIs to access Kai full node-related information.
type TracerAPI struct {
	b Backend
}

// NewTracerAPI creates a new API definition for the tracing methods of the KardiaChain service.
func NewTracerAPI(backend Backend) *TracerAPI {
	return &TracerAPI{b: backend}
}

type chainContext struct {
	api *TracerAPI
	ctx context.Context
}

func (context *chainContext) Config() *configs.ChainConfig {
	return context.api.b.Config()
}

func (context *chainContext) GetHeader(hash common.Hash, height uint64) *types.Header {
	header := context.api.b.HeaderByHeight(context.ctx, rpc.BlockHeight(height))
	if header.Hash() == hash {
		return header
	}
	header = context.api.b.HeaderByHash(context.ctx, hash)
	return header
}

// chainContext constructs the context reader which is used by the KVM for reading
// the necessary chain context.
func (t *TracerAPI) chainContext(ctx context.Context) vm.ChainContext {
	return &chainContext{api: t, ctx: ctx}
}

// TraceTransaction returns the structured logs created during the execution of KVM
// and returns them as a JSON object.
func (t *TracerAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *TraceConfig) (interface{}, error) {
	_, blockHash, blockHeight, index := t.b.GetTransaction(ctx, hash)

	// It shouldn't happen in practice.
	if blockHeight == 0 {
		return nil, errors.New("genesis is not traceable")
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	block, err := t.blockByHeightAndHash(ctx, rpc.BlockHeight(blockHeight), blockHash)
	if err != nil {
		return nil, err
	}
	msg, vmctx, statedb, err := t.b.StateAtTransaction(ctx, block, int(index), reexec)
	if err != nil {
		return nil, err
	}
	txctx := &Context{
		BlockHash: blockHash,
		TxIndex:   int(index),
		TxHash:    hash,
	}
	return t.traceTx(ctx, msg, txctx, vmctx, statedb, config)
}

// TraceCall lets you trace a given eth_call. It collects the structured logs
// created during the execution of KVM if the given transaction was added on
// top of the provided block and returns them as a JSON object.
// You can provide -2 as a block number to trace on top of the pending block.
func (t *TracerAPI) TraceCall(ctx context.Context, args kaiapi.TransactionArgs, blockNrOrHash rpc.BlockHeightOrHash, config *TraceCallConfig) (interface{}, error) {
	// Try to retrieve the specified block
	var (
		err   error
		block *types.Block
	)
	if hash, ok := blockNrOrHash.Hash(); ok {
		block, err = t.blockByHash(ctx, hash)
	} else if height, ok := blockNrOrHash.Height(); ok {
		block, err = t.blockByHeight(ctx, height)
	} else {
		return nil, errors.New("invalid arguments; neither block nor hash specified")
	}
	if err != nil {
		return nil, err
	}
	// try to recompute the state
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	statedb, err := t.b.StateAtBlock(ctx, block, reexec, nil, true)
	if err != nil {
		return nil, err
	}
	// Apply the customized state rules if required.
	if config != nil {
		if err := config.StateOverrides.Apply(statedb); err != nil {
			return nil, err
		}
	}
	// Execute the trace
	msg := args.ToMessage(t.b.RPCGasCap())
	vmctx := blockchain.NewKVMBlockContext(block.Header(), t.chainContext(ctx), nil)

	var traceConfig *TraceConfig
	if config != nil {
		traceConfig = &TraceConfig{
			LogConfig: config.LogConfig,
			Tracer:    config.Tracer,
			Timeout:   config.Timeout,
			Reexec:    config.Reexec,
		}
	}
	return t.traceTx(ctx, msg, new(Context), vmctx, statedb, traceConfig)
}

// traceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (t *TracerAPI) traceTx(ctx context.Context, message blockchain.Message, txctx *Context, vmctx kvm.BlockContext, statedb *state.StateDB, config *TraceConfig) (interface{}, error) {
	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer    kvm.KVMLogger
		err       error
		txContext = blockchain.NewKVMTxContext(message)
	)
	switch {
	case config == nil:
		tracer = logger.NewStructLogger(nil)
	case config.Tracer != nil:
		// Define a meaningful timeout of a single transaction trace
		timeout := defaultTraceTimeout
		if config.Timeout != nil {
			if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
				return nil, err
			}
		}
		// Construct the JavaScript tracer to execute with
		if t, err := New(*config.Tracer, txctx); err != nil {
			return nil, err
		} else {
			deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
			go func() {
				<-deadlineCtx.Done()
				if errors.Is(deadlineCtx.Err(), context.DeadlineExceeded) {
					t.Stop(errors.New("execution timeout"))
				}
			}()
			defer cancel()
			tracer = t
		}
	default:
		tracer = logger.NewStructLogger(config.LogConfig)
	}
	// Run the transaction with tracing enabled.
	vmenv := kvm.NewKVM(vmctx, txContext, statedb, t.b.ChainConfig(), kvm.Config{Debug: true, Tracer: tracer})

	// Call Prepare to clear out the statedb access list
	statedb.Prepare(txctx.TxHash, txctx.BlockHash, txctx.TxIndex)

	result, err := blockchain.ApplyMessage(vmenv, message, new(types.GasPool).AddGas(message.Gas()))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	// Depending on the tracer type, format and return the output.
	switch tracer := tracer.(type) {
	case *logger.StructLogger:
		// If the result contains a revert reason, return it.
		returnVal := fmt.Sprintf("%x", result.Return())
		if len(result.Revert()) > 0 {
			returnVal = fmt.Sprintf("%x", result.Revert())
		}
		reason, _ := result.UnpackRevertReason()
		return &kaiapi.ExecutionResult{
			Gas:          result.UsedGas,
			Failed:       result.Failed(),
			ReturnValue:  returnVal,
			RevertReason: reason,
			StructLogs:   kaiapi.FormatLogs(tracer.StructLogs()),
		}, nil

	case Tracer:
		return tracer.GetResult()

	default:
		panic(fmt.Sprintf("bad tracer type %T", tracer))
	}
}
