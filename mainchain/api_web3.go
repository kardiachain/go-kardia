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
	"fmt"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/accounts"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/node"
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

// Version returns the current KardiaChain protocol version.
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

// GasPrice returns a suggestion for a gas price.
func (s *PublicWeb3API) GasPrice(ctx context.Context) (*common.Big, error) {
	price, err := s.kaiService.SuggestPrice(ctx)
	return (*common.Big)(price), err
}

// ChainId returns chain ID for the current KardiaChain config.
func (s *PublicWeb3API) ChainId() *common.Big {
	return (*common.Big)(s.kaiService.chainConfig.ChainID)
}

// BlockNumber returns the block height of the chain head.
func (s *PublicWeb3API) BlockNumber() common.Uint64 {
	header := s.kaiService.HeaderByHeight(context.Background(), rpc.LatestBlockHeight) // latest header should always be available
	return common.Uint64(header.Height)
}

// GetHeaderByNumber returns the requested canonical block header.
// * When blockNr is math.MaxUint64 - 1 the chain head is returned.
// * When blockNr is math.MaxUint64 - 2 the pending chain head is returned.
func (s *PublicWeb3API) GetHeaderByNumber(ctx context.Context, height rpc.BlockHeight) (map[string]interface{}, error) {
	header := s.kaiService.HeaderByHeight(ctx, height)
	if header != nil {
		response := s.rpcMarshalHeader(ctx, header)
		if height == rpc.PendingBlockHeight {
			// Pending header need to nil out a few fields
			for _, field := range []string{"hash", "miner"} {
				response[field] = nil
			}
		}
		return response, nil
	}
	return nil, ErrHeaderNotFound
}

// GetHeaderByHash returns the requested header by hash.
func (s *PublicWeb3API) GetHeaderByHash(ctx context.Context, hash common.Hash) map[string]interface{} {
	header := s.kaiService.HeaderByHash(ctx, hash)
	if header != nil {
		return s.rpcMarshalHeader(ctx, header)
	}
	return nil
}

// GetBlockByNumber returns the requested canonical block.
// * When blockNr is -1 the chain head is returned.
// * When blockNr is -2 the pending chain head is returned.
// * When fullTx is true all transactions in the block are returned, otherwise
//   only the transaction hash is returned.
func (s *PublicWeb3API) GetBlockByNumber(ctx context.Context, height rpc.BlockHeight, fullTx bool) (map[string]interface{}, error) {
	block := s.kaiService.BlockByHeight(ctx, height)
	if block != nil {
		response, err := s.rpcMarshalBlock(ctx, block, true, fullTx)
		if err == nil && height == rpc.PendingBlockHeight {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, ErrBlockNotFound
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicWeb3API) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block := s.kaiService.BlockByHash(ctx, hash)
	if block != nil {
		return s.rpcMarshalBlock(ctx, block, true, fullTx)
	}
	return nil, ErrBlockNotFound
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block height. The rpc.LatestBlockHeight and rpc.PendingBlockHeight meta
// block heights are also allowed.
func (s *PublicWeb3API) GetBalance(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Big, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*common.Big)(state.GetBalance(address)), state.Error()
}

// GetCode returns the code stored at the given address in the state for the given block height.
func (s *PublicWeb3API) GetCode(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockHeight and rpc.PendingBlockHeight meta block
// heights are also allowed.
func (s *PublicWeb3API) GetStorageAt(ctx context.Context, address common.Address, key string, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

// GetProof returns the Merkle-proof for a given account and optionally some storage keys.
func (s *PublicWeb3API) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockHeightOrHash rpc.BlockHeightOrHash) (*AccountResult, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}

	storageTrie := state.StorageTrie(address)
	storageHash := types.EmptyRootHash
	codeHash := state.GetCodeHash(address)
	storageProof := make([]StorageResult, len(storageKeys))

	// if we have a storageTrie, (which means the account exists), we can update the storagehash
	if storageTrie != nil {
		storageHash = storageTrie.Hash()
	} else {
		// no storageTrie means the account does not exist, so the codeHash is the hash of an empty bytearray.
		codeHash = crypto.Keccak256Hash(nil)
	}

	// create the proof for the storageKeys
	for i, key := range storageKeys {
		if storageTrie != nil {
			proof, storageError := state.GetStorageProof(address, common.HexToHash(key))
			if storageError != nil {
				return nil, storageError
			}
			storageProof[i] = StorageResult{key, (*common.Big)(state.GetState(address, common.HexToHash(key)).Big()), toHexSlice(proof)}
		} else {
			storageProof[i] = StorageResult{key, &common.Big{}, []string{}}
		}
	}

	// create the accountProof
	accountProof, proofErr := state.GetProof(address)
	if proofErr != nil {
		return nil, proofErr
	}

	return &AccountResult{
		Address:      address,
		AccountProof: toHexSlice(accountProof),
		Balance:      (*common.Big)(state.GetBalance(address)),
		CodeHash:     codeHash,
		Nonce:        common.Uint64(state.GetNonce(address)),
		StorageHash:  storageHash,
		StorageProof: storageProof,
	}, state.Error()
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
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

	ChainID *common.Big `json:"chainId,omitempty"`
}

// Call executes the given transaction on the state for the given block height.
// Note, this function doesn't make and changes in the state/blockchain and is
// useful to execute and retrieve values.
func (s *PublicWeb3API) Call(ctx context.Context, args kaiapi.TransactionArgs, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	result, err := kaiapi.DoCall(ctx, s.kaiService, args, blockHeightOrHash, kvm.Config{}, time.Duration(configs.TimeOutForStaticCall)*time.Millisecond)
	if err != nil {
		return nil, err
	}
	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, kaiapi.NewRevertError(result)
	}
	return result.Return(), result.Err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicWeb3API) EstimateGas(ctx context.Context, args kaiapi.TransactionArgs, blockHeightOrHash *rpc.BlockHeightOrHash) (common.Uint64, error) {
	bHeightOrHash := rpc.BlockHeightOrHashWithHeight(rpc.PendingBlockHeight)
	if blockHeightOrHash != nil {
		bHeightOrHash = *blockHeightOrHash
	}
	return kaiapi.DoEstimateGas(ctx, s.kaiService, args, bHeightOrHash, configs.GasLimitCap)
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockHeight      *common.Big     `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              common.Uint64   `json:"gas"`
	GasPrice         *common.Big     `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            common.Bytes    `json:"input"`
	Nonce            common.Uint64   `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *common.Uint64  `json:"transactionIndex"`
	Value            *common.Big     `json:"value"`
	Type             common.Uint64   `json:"type"`
	ChainID          *common.Big     `json:"chainId,omitempty"`
	V                *common.Big     `json:"v"`
	R                *common.Big     `json:"r"`
	S                *common.Big     `json:"s"`
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	kaiService *KardiaService
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(k *KardiaService) *PublicTransactionPoolAPI {
	// The signer used by the API should always be the 'latest' known one because we expect
	// signers to be backwards-compatible with old transactions.
	return &PublicTransactionPoolAPI{k}
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*RPCTransaction, error) {
	// Try to return an already finalized transaction
	tx, blockHash, blockHeight, index := s.kaiService.GetTransaction(ctx, hash)
	if tx != nil {
		return newRPCTransaction(tx, blockHash, blockHeight, index), nil
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.kaiService.TxPool().Get(hash); tx != nil {
		return newRPCPendingTransaction(tx), nil
	}

	// Transaction unknown, return as such
	return nil, nil
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (s *PublicTransactionPoolAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (common.Bytes, error) {
	// Retrieve a finalized transaction, or a pooled otherwise
	tx, _, _, _ := s.kaiService.GetTransaction(ctx, hash)
	if tx == nil {
		if tx = s.kaiService.TxPool().Get(hash); tx == nil {
			// Transaction not found anywhere, abort
			return nil, ErrTransactionHashNotFound
		}
	}
	// Serialize to RLP and return
	return rlp.EncodeToBytes(tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	tx, blockHash, blockHeight, index := s.kaiService.GetTransaction(ctx, hash)
	if tx == nil || blockHeight == 0 {
		return nil, nil
	}
	// get receipts from db
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, blockHash)
	if blockInfo == nil {
		return nil, ErrBlockInfoNotFound
	}
	// return the receipt if tx and receipt hashes at index are the same
	if len(blockInfo.Receipts) > int(index) && blockInfo.Receipts[index].TxHash.Equal(hash) {
		receipt := blockInfo.Receipts[index]
		return getWeb3Receipt(s.kaiService.chainConfig, receipt, tx, blockHash, blockHeight, index, blockInfo), nil
	}
	// else traverse receipts list to find the corresponding receipt of txHash
	for _, r := range blockInfo.Receipts {
		if !r.TxHash.Equal(hash) {
			continue
		} else {
			receipt := r
			return getWeb3Receipt(s.kaiService.chainConfig, receipt, tx, blockHash, blockHeight, index, blockInfo), nil
		}
	}

	// dirty hack searching receipt in the few previous block
	for i := uint64(1); i <= 2; i++ {
		block := s.kaiService.BlockByHeight(ctx, rpc.BlockHeight(blockHeight-i))
		// get receipts from db
		blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, block.Hash())
		if blockInfo == nil {
			return nil, ErrBlockInfoNotFound
		}
		for _, r := range blockInfo.Receipts {
			if !r.TxHash.Equal(hash) {
				continue
			} else {
				// update the correct lookup entry and try again
				s.kaiService.kaiDb.WriteTxLookupEntries(block)
				return s.GetTransactionReceipt(ctx, hash)
			}
		}
	}
	// return nil if not found
	return nil, nil
}

func getWeb3Receipt(config *configs.ChainConfig, receipt *types.Receipt, tx *types.Transaction, blockHash common.Hash, blockHeight, index uint64, blockInfo *types.BlockInfo) map[string]interface{} {
	// Derive the sender
	from, _ := types.Sender(types.LatestSigner(config), tx)
	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       common.Uint64(blockHeight),
		"transactionHash":   tx.Hash(),
		"transactionIndex":  common.Uint64(index),
		"from":              from,
		"to":                tx.To(),
		"gasUsed":           common.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": common.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
	}
	// convert bloom and logs
	bloom, err := UnmarshalLogsBloom(&blockInfo.Bloom)
	if err == nil {
		fields["logsBloom"] = bloom
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	} else {
		web3Logs := make([]*types.LogForWeb3, len(receipt.Logs))
		for i := range receipt.Logs {
			web3Logs[i] = &types.LogForWeb3{
				Log: *receipt.Logs[i],
			}
		}
		fields["logs"] = web3Logs
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = common.Bytes(receipt.PostState)
	} else {
		fields["status"] = common.Uint(receipt.Status)
	}

	// If the ContractAddress field is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (*common.Uint64, error) {
	// Ask transaction pool for the nonce which includes pending transactions
	if blockHeight, ok := blockHeightOrHash.Height(); ok && blockHeight == rpc.PendingBlockHeight {
		nonce := s.kaiService.txPool.Nonce(address)
		return (*common.Uint64)(&nonce), nil
	}
	// Resolve block number and use its state to ask for the nonce
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	nonce := state.GetNonce(address)
	return (*common.Uint64)(&nonce), state.Error()
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, input common.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(input, &tx); err != nil {
		return common.Hash{}, err
	}
	// Drop tx exceeds gas requirements (DDoS protection)
	if err := checkGas(tx.GasPrice(), tx.Gas()); err != nil {
		return common.Hash{}, err
	}
	// If the transaction fee cap is already specified, ensure the
	// fee of the given transaction is reasonable.
	if err := checkTxFee(tx.GasPrice(), tx.Gas(), configs.TxFeeCap); err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), s.kaiService.TxPool().AddLocal(tx)
}

// publicWeb3API offers helper utils
type publicWeb3API struct {
	nodeConfig *node.Config
}

// ClientVersion returns the node name
func (s *publicWeb3API) ClientVersion() string {
	return s.nodeConfig.NodeName()
}

// Sha3 applies the sha3 implementation on the input.
// It assumes the input is hex encoded.
func (s *publicWeb3API) Sha3(input common.Bytes) common.Bytes {
	return crypto.Keccak256(input)
}

// PublicNodeAccountAPI provides an API to access accounts managed by this node.
// It offers only methods that can retrieve accounts.
type PublicNodeAccountAPI struct {
	am *accounts.Manager
}

// NewPublicNodeAccountAPI creates a new PublicNodeAccountAPI.
func NewPublicNodeAccountAPI(am *accounts.Manager) *PublicNodeAccountAPI {
	return &PublicNodeAccountAPI{am: am}
}

// Accounts returns the collection of accounts this node manages
func (s *PublicNodeAccountAPI) Accounts() []common.Address {
	return s.am.Accounts()
}
