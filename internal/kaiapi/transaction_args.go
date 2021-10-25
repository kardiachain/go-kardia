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

package kaiapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/math"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *common.Uint64  `json:"gas"`
	GasPrice *common.Big     `json:"gasPrice"`
	Value    *common.Big     `json:"value"`
	Nonce    *common.Uint64  `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	Data  *common.Bytes `json:"data"`
	Input *common.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	ChainID *common.Big `json:"chainId,omitempty"`
}

// from retrieves the transaction sender address.
func (arg *TransactionArgs) from() common.Address {
	if arg.From == nil {
		return common.Address{}
	}
	return *arg.From
}

// data retrieves the transaction calldata. Input field is preferred.
func (arg *TransactionArgs) data() []byte {
	if arg.Input != nil {
		return *arg.Input
	}
	if arg.Data != nil {
		return *arg.Data
	}
	return nil
}

// setDefaults fills in default values for unspecified tx fields.
func (args *TransactionArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.Value == nil {
		args.Value = new(common.Big)
	}
	if args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.from())
		if err != nil {
			return err
		}
		args.Nonce = (*common.Uint64)(&nonce)
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`)
	}
	if args.To == nil && len(args.data()) == 0 {
		return errors.New(`contract creation without any data provided`)
	}
	// Estimate the gas usage if necessary.
	if args.Gas == nil {
		// These fields are immutable during the estimation, safe to
		// pass the pointer directly.
		data := args.data()
		callArgs := TransactionArgs{
			From:     args.From,
			To:       args.To,
			GasPrice: args.GasPrice,
			Value:    args.Value,
			Data:     (*common.Bytes)(&data),
		}
		pendingBlockHeight := rpc.BlockHeightOrHashWithHeight(rpc.PendingBlockHeight)
		estimated, err := DoEstimateGas(ctx, b, callArgs, pendingBlockHeight, b.RPCGasCap())
		if err != nil {
			return err
		}
		args.Gas = &estimated
		log.Trace("Estimate gas usage automatically", "gas", args.Gas)
	}
	if args.ChainID == nil {
		id := (*common.Big)(b.ChainConfig().ChainID)
		args.ChainID = id
	}
	return nil
}

// ToMessage converts the transaction arguments to the Message type used by the
// core kvm. This method is used in calls and traces that do not require a real
// live transaction.
func (args *TransactionArgs) ToMessage(globalGasCap uint64) types.Message {
	// Set sender address or use zero address if none specified.
	addr := args.from()

	// Set default gas & gas price if none were set
	gas := globalGasCap
	if gas == 0 {
		gas = uint64(math.MaxUint64 / 2)
	}
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != 0 && globalGasCap < gas {
		log.Warn("Caller gas above allowance, capping", "requested", gas, "cap", globalGasCap)
		gas = globalGasCap
	}
	var (
		gasPrice *big.Int
	)

	if args.GasPrice != nil {
		// User specified the legacy gas field, convert to 1559 gas typing
		gasPrice = args.GasPrice.ToInt()
	} else {
		// Backfill the legacy gasPrice for EVM execution, unless we're all zeroes
		gasPrice = new(big.Int)
	}
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	data := args.data()
	msg := types.NewMessage(addr, args.To, 0, value, gas, gasPrice, data, true)
	return msg
}

// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type ExecutionResult struct {
	Gas          uint64         `json:"gas"`
	Failed       bool           `json:"failed"`
	ReturnValue  string         `json:"returnValue"`
	RevertReason string         `json:"revertReason"`
	StructLogs   []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   string             `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// FormatLogs formats KVM returned structured logs for json output
func FormatLogs(logs []kvm.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.ErrorString(),
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = stackValue.Text(16)
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}

func toTransactionArgs(args TransactionArgs, b Backend) types.CallArgsJSON {
	to := args.To.Hex()
	return types.CallArgsJSON{
		From:     args.From.Hex(),
		To:       &to,
		Gas:      uint64(*args.Gas),
		GasPrice: args.GasPrice.ToInt(),
		Value:    args.Value.ToInt(),
		Data:     args.Data.String(),
	}
}
