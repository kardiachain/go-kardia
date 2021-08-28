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

package kai

import (
	"context"
	"math"
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// UnmarshalLogsBloom encodes bloom as a hex string with 0x prefix.
func UnmarshalLogsBloom(b *types.Bloom) (string, error) {
	return "0x" + common.Bytes2Hex(b[:]), nil
}

// rpcMarshalHeader uses the generalized output filler, then adds additional fields, which requires
// a `blockInfo`.
func (s *PublicWeb3API) rpcMarshalHeader(ctx context.Context, header *types.Header) map[string]interface{} {
	fields := RPCMarshalHeader(header)
	fields["gasUsed"] = "0x0"
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, header.Hash())
	if blockInfo != nil {
		bloom, err := UnmarshalLogsBloom(&blockInfo.Bloom)
		if err == nil {
			fields["logsBloom"] = bloom
		}
		fields["gasUsed"] = common.Uint64(blockInfo.GasUsed)
		fields["rewards"] = (*common.Big)(blockInfo.Rewards)
	}
	return fields
}

// RPCMarshalHeader converts the given header to the RPC output.
func RPCMarshalHeader(head *types.Header) map[string]interface{} {
	// TODO(trinhdn97): remove hardcode
	return map[string]interface{}{
		"number":           (*common.Big)(new(big.Int).SetUint64(head.Height)),
		"hash":             head.Hash(),
		"parentHash":       head.LastBlockID.Hash,
		"nonce":            "0x0000000000000000",
		"mixHash":          common.NewZeroHash(),
		"sha3Uncles":       common.NewZeroHash(),
		"stateRoot":        head.AppHash,
		"miner":            head.ProposerAddress.Hex(),
		"difficulty":       "0x000000",
		"extraData":        common.NewZeroHash(),
		"size":             "0x000000",
		"gasLimit":         common.Uint64(head.GasLimit),
		"timestamp":        common.Uint64(head.Time.Unix()),
		"transactionsRoot": head.TxHash,
		"receiptsRoot":     common.NewZeroHash(),
		// additional KardiaChain network fields
		"numTxs":            common.Uint64(head.NumTxs),
		"commitHash":        head.LastCommitHash,
		"validatorHash":     head.ValidatorsHash,
		"nextValidatorHash": head.NextValidatorsHash,
		"consensusHash":     head.ConsensusHash,
		"evidenceHash":      head.EvidenceHash,
	}
}

// rpcMarshalBlock uses the generalized output filler, then adds adds additional fields, which requires
// a `blockInfo`.
func (s *PublicWeb3API) rpcMarshalBlock(ctx context.Context, b *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	fields, err := RPCMarshalBlock(b, inclTx, fullTx)
	if err != nil {
		return nil, err
	}
	fields["gasUsed"] = "0x0"
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, b.Hash())
	if blockInfo != nil {
		bloom, err := UnmarshalLogsBloom(&blockInfo.Bloom)
		if err == nil {
			fields["logsBloom"] = bloom
		}
		fields["gasUsed"] = common.Uint64(blockInfo.GasUsed)
		fields["rewards"] = (*common.Big)(blockInfo.Rewards)
	}
	return fields, nil
}

// RPCMarshalBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func RPCMarshalBlock(block *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	fields := RPCMarshalHeader(block.Header())

	if inclTx {
		formatTx := func(tx *types.Transaction) (interface{}, error) {
			return tx.Hash(), nil
		}
		if fullTx {
			formatTx = func(tx *types.Transaction) (interface{}, error) {
				return newRPCTransactionFromBlockHash(block, tx.Hash()), nil
			}
		}
		txs := block.Transactions()
		transactions := make([]interface{}, len(txs))
		var err error
		for i, tx := range txs {
			if transactions[i], err = formatTx(tx); err != nil {
				return nil, err
			}
		}
		fields["transactions"] = transactions
	}

	return fields, nil
}

// newRPCTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockHash(b *types.Block, hash common.Hash) *RPCTransaction {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64) *RPCTransaction {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.Hash(), b.Height(), index)
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *types.Transaction) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, 0, 0)
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx *types.Transaction, blockHash common.Hash, blockHeight uint64, index uint64) *RPCTransaction {
	// Determine the signer. For replay-protected transactions, use the most permissive
	// signer, because we assume that signers are backwards-compatible with old
	// transactions. For non-protected transactions, the homestead signer signer is used
	// because the return value of ChainId is zero for those transactions.
	var signer types.Signer
	if tx.Protected() {
		signer = types.LatestSignerForChainID(tx.ChainId())
	} else {
		signer = types.HomesteadSigner{}
	}
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()
	result := &RPCTransaction{
		From:     from,
		Gas:      common.Uint64(tx.Gas()),
		GasPrice: (*common.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    common.Bytes(tx.Data()),
		Nonce:    common.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*common.Big)(tx.Value()),
		V:        (*common.Big)(v),
		R:        (*common.Big)(r),
		S:        (*common.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = &blockHash
		result.BlockHeight = (*common.Big)(new(big.Int).SetUint64(blockHeight))
		result.TransactionIndex = (*common.Uint64)(&index)
	}
	return result
}

// ToMessage converts CallArgs to the Message type used by the core KVM
func (args *CallArgs) ToMessage(globalGasCap uint64) types.Message {
	// Set sender address or use zero address if none specified.
	var addr common.Address
	if args.From != nil {
		addr = *args.From
	}

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
	gasPrice := new(big.Int)
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	}
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	var data []byte
	if args.Data != nil {
		data = *args.Data
	}

	msg := types.NewMessage(addr, args.To, 0, value, gas, gasPrice, data, false)
	return msg
}
