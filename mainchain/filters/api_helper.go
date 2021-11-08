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
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// UnmarshalLogsBloom encodes bloom as a hex string with 0x prefix.
func UnmarshalLogsBloom(b *types.Bloom) (string, error) {
	return "0x" + common.Bytes2Hex(b[:]), nil
}

// rpcMarshalHeader uses the generalized output filler, then adds additional fields, which requires
// a `blockInfo`.
func (api *PublicFilterAPI) rpcMarshalHeader(ctx context.Context, header *types.Header) map[string]interface{} {
	fields := RPCMarshalHeader(header)
	fields["gasUsed"] = "0x0"
	blockInfo := api.backend.BlockInfoByBlockHash(ctx, header.Hash())
	if blockInfo != nil {
		bloom, err := UnmarshalLogsBloom(blockInfo.Bloom)
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
		"sha3Uncles":       common.NewZeroHash(),
		"stateRoot":        head.AppHash,
		"miner":            head.ProposerAddress,
		"difficulty":       "0x000000",
		"extraData":        common.NewZeroHash(),
		"size":             common.Uint64(head.Size()),
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
