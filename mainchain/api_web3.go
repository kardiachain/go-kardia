/*
 *  Copyright 2018 KardiaChain
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
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	networkVersion uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{networkVersion}
}

// Version returns the current ethereum protocol version.
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}

// PublicWeb3API provides web3-compatible APIs to access the KardiaChain blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicWeb3API struct {
	kaiService *KardiaService
}

// NewPublicWeb3API creates a new KardiaChain blockchain web3 APIs.
func NewPublicWeb3API(k *KardiaService) *PublicWeb3API {
	return &PublicWeb3API{k}
}

func (s *PublicWeb3API) Dummy() {

}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicKaiAPI) GetBalance(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Big, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*common.Big)(state.GetBalance(address)), state.Error()
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
}

// Call executes the given transaction on the state for the given block height.
// Note, this function doesn't make and changes in the state/blockchain and is
// useful to execute and retrieve values.
func (s *PublicKaiAPI) Call(ctx context.Context, args CallArgs, blockHeightOrHash rpc.BlockHeightOrHash) (hexutil.Bytes, error) {
	result, err := s.doCall(ctx, toCallArg(args), blockHeightOrHash, kvm.Config{}, configs.DefaultTimeOutForStaticCall)
	if err != nil {
		return nil, err
	}
	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, newRevertError(result)
	}
	return result.Return(), result.Err
}

func toCallArg(msg CallArgs) (result types.CallArgsJSON) {
	if msg.From != nil {
		result.From = msg.From.Hex()
	}
	if msg.To != nil {
		to := msg.To.Hex()
		result.To = &to
	}
	result.Data = msg.Data.String()
	// add default params to avoid eth_call crashing
	result.Value = msg.Value.ToInt()
	if msg.Gas != nil {
		result.Gas = uint64(*msg.Gas)
	}
	result.GasPrice = msg.GasPrice.ToInt()
	return result
}
