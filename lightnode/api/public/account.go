// Package public
package public

import (
	"context"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lightnode"
	"github.com/kardiachain/go-kardiamain/rpc"
)

// PublicAccountAPI provides APIs support getting account's info
type AccountAPI struct {
	service lightnode.NodeService
}

// NewPublicAccountAPI is a constructor that init new PublicAccountAPI
func NewPublicAccountAPI(kaiService lightnode.NodeService) *AccountAPI {
	return &AccountAPI{kaiService}
}

// Balance returns address's balance
func (a *AccountAPI) Balance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (string, error) {
	state, _, err := a.service.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return "", err
	}
	return state.GetBalance(address).String(), nil
}

// Nonce return address's nonce
func (a *AccountAPI) Nonce(address string) (uint64, error) {
	addr := common.HexToAddress(address)
	nonce := a.service.TxPool().Nonce(addr)
	return nonce, nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (a *AccountAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (common.Bytes, error) {
	state, _, err := a.service.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (a *AccountAPI) GetStorageAt(ctx context.Context, address common.Address, key string, blockNrOrHash rpc.BlockNumberOrHash) (common.Bytes, error) {
	state, _, err := a.service.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}
