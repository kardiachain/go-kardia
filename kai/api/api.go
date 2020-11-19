// Package api
package api

import (
	"context"
	"time"

	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/rpc"
	"github.com/kardiachain/go-kardiamain/types"
)

type API interface {
	Call(ctx context.Context, args types.CallArgsJSON, blockNrOrHash rpc.BlockNumberOrHash, vmCfg kvm.Config, timeout time.Duration) (*kvm.ExecutionResult, error)
}
type LightNodeAPI interface {
	API
	// Account API
	Balance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (string, error)
	Nonce(address string) (uint64, error)
	GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (common.Bytes, error)
	GetStorageAt(ctx context.Context, address common.Address, key string, blockNrOrHash rpc.BlockNumberOrHash) (common.Bytes, error)

	// Node API
	BlockNumber() uint64
	GetBlockHeaderByNumber(ctx context.Context, blockNumber rpc.BlockNumber) interface{}
	GetBlockHeaderByHash(ctx context.Context, blockHash string) interface{}
	GetBlockByNumber(ctx context.Context, blockNumber rpc.BlockNumber) interface{}
	GetBlockByHash(ctx context.Context, blockHash string) interface{}

	// Transaction
	//SendRawTransaction(ctx context.Context, txs string) (string, error)
	//PendingTransactions() ([]*PublicTransaction, error)
	//GetTransaction(hash string) (*PublicTransaction, error)
	//GetTransactionReceipt(ctx context.Context, hash string) (*PublicReceipt, error)
	//EstimateGas(ctx context.Context, args types.CallArgsJSON, blockNrOrHash rpc.BlockNumberOrHash) (uint64, error)
}

type FullNodeAPI interface {
	LightNodeAPI
}
