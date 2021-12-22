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
	"errors"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/types"
)

// BlockHeaderJSON represents BlockHeader in JSON format
type BlockHeaderJSON struct {
	Hash              common.Hash    `json:"hash"`
	Height            uint64         `json:"height"`
	LastBlock         common.Hash    `json:"lastBlock"`
	LastBlockID       *types.BlockID `json:"lastBlockID"`
	CommitHash        common.Hash    `json:"commitHash"`
	Time              time.Time      `json:"time"`
	NumTxs            uint64         `json:"numTxs"`
	GasUsed           uint64         `json:"gasUsed"`
	GasLimit          uint64         `json:"gasLimit"`
	Rewards           string         `json:"Rewards"`
	ProposerAddress   string         `json:"proposerAddress"`
	TxHash            common.Hash    `json:"dataHash"` // transactions
	Bloom             types.Bloom    `json:"logsBloom"`
	ValidatorsHash    common.Hash    `json:"validatorHash"`     // current block validators hash
	NextValidatorHash common.Hash    `json:"nextValidatorHash"` // next block validators hash
	ConsensusHash     common.Hash    `json:"consensusHash"`     // current consensus hash
	AppHash           common.Hash    `json:"appHash"`           // state of transactions
	EvidenceHash      common.Hash    `json:"evidenceHash"`      // hash of evidence
}

// BlockJSON represents Block in JSON format
type BlockJSON struct {
	Hash              common.Hash          `json:"hash"`
	Height            uint64               `json:"height"`
	LastBlock         common.Hash          `json:"lastBlock"`
	LastBlockID       *types.BlockID       `json:"lastBlockID"`
	CommitHash        common.Hash          `json:"commitHash"`
	Time              time.Time            `json:"time"`
	NumTxs            uint64               `json:"numTxs"`
	GasLimit          uint64               `json:"gasLimit"`
	GasUsed           uint64               `json:"gasUsed"`
	Rewards           string               `json:"rewards"`
	ProposerAddress   common.Address       `json:"proposerAddress"`
	TxHash            common.Hash          `json:"dataHash"` // hash of txs
	Bloom             types.Bloom          `json:"logsBloom"`
	ValidatorsHash    common.Hash          `json:"validatorHash"`     // validators for the current block
	NextValidatorHash common.Hash          `json:"nextValidatorHash"` // validators for the current block
	ConsensusHash     common.Hash          `json:"consensusHash"`     // hash of current consensus
	AppHash           common.Hash          `json:"appHash"`           // txs state
	EvidenceHash      common.Hash          `json:"evidenceHash"`      // hash of evidence
	Txs               []*PublicTransaction `json:"txs"`
	Receipts          []*BasicReceipt      `json:"receipts"`
}

// PublicKaiAPI provides APIs to access Kai full node-related
// information.
type PublicKaiAPI struct {
	kaiService *KardiaService
}

// NewPublicKaiAPI creates a new Kai protocol API for full nodes.
func NewPublicKaiAPI(kaiService *KardiaService) *PublicKaiAPI {
	return &PublicKaiAPI{kaiService}
}

// getBasicReceipt is used to get simplified receipt. This function is used when loading block info
func getBasicReceipt(receipt types.Receipt) *BasicReceipt {
	logs := getReceiptLogs(receipt)
	basicReceipt := BasicReceipt{
		TransactionHash:   receipt.TxHash.Hex(),
		GasUsed:           receipt.GasUsed,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Logs:              logs,
	}
	basicReceipt.Status = uint(receipt.Status)
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		basicReceipt.ContractAddress = receipt.ContractAddress.Hex()
	}

	return &basicReceipt
}

// NewBlockHeaderJSON creates a new BlockHeader JSON data from Block
func NewBlockHeaderJSON(header *types.Header, blockInfo *types.BlockInfo) *BlockHeaderJSON {
	if header == nil {
		return nil
	}
	return &BlockHeaderJSON{
		Hash:              header.Hash(),
		Height:            header.Height,
		LastBlock:         header.LastBlockID.Hash,
		LastBlockID:       &header.LastBlockID,
		CommitHash:        header.LastCommitHash,
		Time:              header.Time,
		NumTxs:            header.NumTxs,
		Rewards:           blockInfo.Rewards.String(),
		GasUsed:           blockInfo.GasUsed,
		GasLimit:          header.GasLimit,
		ProposerAddress:   header.ProposerAddress.Hex(),
		TxHash:            header.TxHash,
		Bloom:             blockInfo.Bloom,
		ValidatorsHash:    header.ValidatorsHash,
		NextValidatorHash: header.NextValidatorsHash,
		ConsensusHash:     header.ConsensusHash,
		AppHash:           header.AppHash,
		EvidenceHash:      header.EvidenceHash,
	}
}

// NewBlockJSON creates a new Block JSON data from Block
func NewBlockJSON(config *configs.ChainConfig, block *types.Block, blockInfo *types.BlockInfo) *BlockJSON {
	if block == nil {
		return nil
	}
	txs := block.Transactions()
	transactions := make([]*PublicTransaction, 0, len(txs))
	basicReceipts := make([]*BasicReceipt, 0)

	if blockInfo != nil {
		for _, receipt := range blockInfo.Receipts {
			basicReceipts = append(basicReceipts, getBasicReceipt(*receipt))
		}
	} else {
		blockInfo = &types.BlockInfo{}
	}

	for index, transaction := range txs {
		idx := uint64(index)
		tx := NewPublicTransaction(config, transaction, block.Hash(), block.Height(), idx)
		// add time for tx
		tx.Time = block.Header().Time
		transactions = append(transactions, tx)
	}

	return &BlockJSON{
		Hash:              block.Hash(),
		Height:            block.Height(),
		LastBlock:         block.Header().LastBlockID.Hash,
		LastBlockID:       &block.Header().LastBlockID,
		Txs:               transactions,
		CommitHash:        block.LastCommitHash(),
		Time:              block.Header().Time,
		NumTxs:            block.Header().NumTxs,
		GasLimit:          block.Header().GasLimit,
		GasUsed:           blockInfo.GasUsed,
		Rewards:           blockInfo.Rewards.String(),
		ProposerAddress:   block.Header().ProposerAddress,
		Bloom:             blockInfo.Bloom,
		TxHash:            block.Header().TxHash,
		ValidatorsHash:    block.Header().ValidatorsHash,
		NextValidatorHash: block.Header().NextValidatorsHash,
		ConsensusHash:     block.Header().ConsensusHash,
		AppHash:           block.Header().AppHash,
		EvidenceHash:      block.Header().EvidenceHash,
		Receipts:          basicReceipts,
	}
}

// BlockNumber returns current block height
func (s *PublicKaiAPI) BlockNumber() uint64 {
	return s.kaiService.blockchain.CurrentBlock().Height()
}

// GetBlockHeaderByNumber returns blockHeader by block height
func (s *PublicKaiAPI) GetBlockHeaderByNumber(ctx context.Context, blockHeight rpc.BlockHeight) *BlockHeaderJSON {
	header := s.kaiService.HeaderByHeight(ctx, blockHeight)
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, header.Hash())
	return NewBlockHeaderJSON(header, blockInfo)
}

// GetBlockHeaderByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockHeaderByHash(ctx context.Context, blockHash rpc.BlockHeightOrHash) *BlockHeaderJSON {
	header, _ := s.kaiService.HeaderByHeightOrHash(ctx, blockHash)
	if header == nil {
		return nil
	}
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, header.Hash())
	return NewBlockHeaderJSON(header, blockInfo)
}

// GetBlockByNumber returns block by block height
func (s *PublicKaiAPI) GetBlockByNumber(ctx context.Context, blockHeight rpc.BlockHeight) *BlockJSON {
	block := s.kaiService.BlockByHeight(ctx, blockHeight)
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, block.Hash())
	return NewBlockJSON(s.kaiService.chainConfig, block, blockInfo)
}

// GetBlockByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockByHash(ctx context.Context, blockHash rpc.BlockHeightOrHash) *BlockJSON {
	block, _ := s.kaiService.BlockByHeightOrHash(ctx, blockHash)
	if block == nil {
		return nil
	}
	blockInfo := s.kaiService.BlockInfoByBlockHash(ctx, block.Hash())
	return NewBlockJSON(s.kaiService.chainConfig, block, blockInfo)
}

type Validator struct {
	Name                  string       `json:"name"`
	Address               string       `json:"address"`
	SmcAddress            string       `json:"smcAddress"`
	StakedAmount          string       `json:"stakedAmount"`
	CommissionRate        string       `json:"commissionRate"`
	TotalDelegators       int          `json:"totalDelegators"`
	MaxRate               string       `json:"maxRate"`
	MaxChangeRate         string       `json:"maxChangeRate"`
	Jailed                bool         `json:"jailed"`
	DelegationShares      string       `json:"delegationShares"`
	AccumulatedCommission string       `json:"accumulatedCommission"`
	UbdEntryCount         string       `json:"ubdEntryCount"`
	UpdateTime            uint64       `json:"updateTime"`
	Status                uint8        `json:"status"`
	UnbondingTime         string       `json:"unbondingTime"`
	UnbondingHeight       string       `json:"unbondingHeight"`
	SigningInfo           *SigningInfo `json:"signingInfo"`
	Delegators            []*Delegator `json:"delegators,omitempty"`
}

type SigningInfo struct {
	StartHeight        uint64 `json:"startHeight"`
	IndexOffset        uint64 `json:"indexOffset"`
	Tombstoned         bool   `json:"tombstoned"`
	MissedBlockCounter uint64 `json:"missedBlockCounter"`
	JailedUntil        uint64 `json:"jailedUntil"`
}

type Delegator struct {
	Address      string `json:"address"`
	StakedAmount string `json:"stakedAmount"`
	Reward       string `json:"reward"`
}

// Validator returns node's validator, nil if current node is not a validator
func (s *PublicKaiAPI) Validator(ctx context.Context, valAddr common.Address, isGetDelegators bool) (*Validator, error) {
	val, err := s.kaiService.GetValidator(valAddr)
	if err != nil {
		return nil, err
	}
	var delegatorsList []*Delegator
	if isGetDelegators {
		for _, del := range val.Delegators {
			delegatorsList = append(delegatorsList, &Delegator{
				Address:      del.Address.Hex(),
				StakedAmount: del.StakedAmount.String(),
				Reward:       del.Reward.String(),
			})
		}
	} else {
		delegatorsList = nil
	}

	var name []byte
	for _, b := range val.Name {
		if b != 0 {
			name = append(name, byte(b))
		}
	}

	return &Validator{
		Name:                  string(name),
		Address:               val.ValAddr.String(),
		SmcAddress:            val.ValStakingSmc.String(),
		StakedAmount:          val.Tokens.String(),
		CommissionRate:        val.CommissionRate.String(),
		TotalDelegators:       len(val.Delegators),
		MaxRate:               val.MaxRate.String(),
		MaxChangeRate:         val.MaxChangeRate.String(),
		Jailed:                val.Jailed,
		DelegationShares:      val.DelegationShares.String(),
		AccumulatedCommission: val.AccumulatedCommission.String(),
		UbdEntryCount:         val.UbdEntryCount.String(),
		UpdateTime:            val.UpdateTime.Uint64(),
		Status:                val.Status,
		UnbondingTime:         val.UnbondingTime.String(),
		UnbondingHeight:       val.UnbondingHeight.String(),
		SigningInfo: &SigningInfo{
			StartHeight:        val.SigningInfo.StartHeight.Uint64(),
			IndexOffset:        val.SigningInfo.IndexOffset.Uint64(),
			Tombstoned:         val.SigningInfo.Tombstoned,
			MissedBlockCounter: val.SigningInfo.MissedBlockCounter.Uint64(),
			JailedUntil:        val.SigningInfo.JailedUntil.Uint64(),
		},
		Delegators: delegatorsList,
	}, nil
}

// Validators returns a list of validator
func (s *PublicKaiAPI) Validators(ctx context.Context, isGetDelegators bool) ([]*Validator, error) {
	var validators []*Validator
	valList, err := s.kaiService.GetValidators()
	if err != nil {
		return nil, err
	}
	for _, val := range valList {
		validator, err := s.Validator(ctx, val.ValAddr, isGetDelegators)
		if err != nil {
			return nil, err
		}
		validators = append(validators, validator)
	}
	return validators, nil
}

type PublicTransaction struct {
	BlockHash        string       `json:"blockHash"`
	BlockHeight      uint64       `json:"blockNumber"`
	Time             time.Time    `json:"time"`
	From             string       `json:"from"`
	Gas              uint64       `json:"gas"`
	GasPrice         uint64       `json:"gasPrice"`
	GasUsed          uint64       `json:"gasUsed,omitempty"`
	ContractAddress  string       `json:"contractAddress,omitempty"`
	Hash             string       `json:"hash"`
	Input            string       `json:"input"`
	Nonce            uint64       `json:"nonce"`
	To               string       `json:"to"`
	TransactionIndex uint         `json:"transactionIndex"`
	Value            string       `json:"value"`
	Logs             []Log        `json:"logs,omitempty"`
	LogsBloom        types.Bloom  `json:"logsBloom,omitempty"`
	Root             common.Bytes `json:"root,omitempty"`
	V                *common.Big  `json:"v"`
	R                *common.Big  `json:"r"`
	S                *common.Big  `json:"s"`
}

type Log struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockHeight uint64   `json:"blockHeight"`
	TxHash      string   `json:"transactionHash"`
	TxIndex     uint     `json:"transactionIndex"`
	BlockHash   string   `json:"blockHash"`
	Index       uint     `json:"logIndex"`
	Removed     bool     `json:"removed"`
}

type PublicReceipt struct {
	BlockHash         string      `json:"blockHash"`
	BlockHeight       uint64      `json:"blockHeight"`
	TransactionHash   string      `json:"transactionHash"`
	TransactionIndex  uint64      `json:"transactionIndex"`
	From              string      `json:"from"`
	To                string      `json:"to"`
	GasUsed           uint64      `json:"gasUsed"`
	CumulativeGasUsed uint64      `json:"cumulativeGasUsed"`
	ContractAddress   string      `json:"contractAddress"`
	Logs              []Log       `json:"logs"`
	LogsBloom         types.Bloom `json:"logsBloom"`
	Status            uint        `json:"status"`
}

type BasicReceipt struct {
	TransactionHash   string `json:"transactionHash"`
	GasUsed           uint64 `json:"gasUsed"`
	CumulativeGasUsed uint64 `json:"cumulativeGasUsed"`
	ContractAddress   string `json:"contractAddress"`
	Logs              []Log  `json:"logs"`
	Status            uint   `json:"status"`
}

// NewPublicTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewPublicTransaction(config *configs.ChainConfig, tx *types.Transaction, blockHash common.Hash, blockHeight uint64, index uint64) *PublicTransaction {
	from, _ := types.Sender(types.LatestSigner(config), tx)
	v, r, s := tx.RawSignatureValues()
	result := &PublicTransaction{
		From:     from.Hex(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice().Uint64(),
		Hash:     tx.Hash().Hex(),
		Input:    common.Encode(tx.Data()),
		Nonce:    tx.Nonce(),
		Value:    tx.Value().String(),
		V:        (*common.Big)(v),
		R:        (*common.Big)(r),
		S:        (*common.Big)(s),
	}
	if tx.To() != nil {
		result.To = tx.To().Hex()
	} else {
		result.To = "0x"
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash.Hex()
		result.BlockHeight = blockHeight
		result.TransactionIndex = uint(index)
	}
	return result
}

// PublicTransactionAPI provides public apis relate to transactions
type PublicTransactionAPI struct {
	s *KardiaService
}

// NewPublicTransactionAPI is a constructor of PublicTransactionAPI
func NewPublicTransactionAPI(service *KardiaService) *PublicTransactionAPI {
	return &PublicTransactionAPI{service}
}

// SendRawTransaction decode encoded data into tx and then add tx into pool
func (a *PublicTransactionAPI) SendRawTransaction(ctx context.Context, txs string) (string, error) {
	log.Info("SendRawTransaction", "data", txs)
	tx := new(types.Transaction)
	encodedTx := common.FromHex(txs)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}.Hex(), err
	}
	// Drop tx exceeds gas requirements (DDoS protection)
	if err := checkGas(tx.GasPrice(), tx.Gas()); err != nil {
		return common.Hash{}.Hex(), err
	}
	// If the transaction fee cap is already specified, ensure the
	// fee of the given transaction is reasonable.
	if err := checkTxFee(tx.GasPrice(), tx.Gas(), configs.TxFeeCap); err != nil {
		return common.Hash{}.Hex(), err
	}

	return tx.Hash().Hex(), a.s.TxPool().AddLocal(tx)
}

// KardiaCall execute a contract method call only against
// state on the local node. No tx is generated and submitted
// onto the blockchain
func (s *PublicKaiAPI) KardiaCall(ctx context.Context, args kaiapi.TransactionArgs, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
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

// GetValidatorSet get the validators set at block height
func (s *PublicKaiAPI) GetValidatorSet(blockHeight rpc.BlockHeight) (*types.ValidatorSet, error) {
	return s.kaiService.stateDB.LoadValidators(blockHeight.Uint64())
}

// GetCommit get validators' commits for the block by height
func (s *PublicKaiAPI) GetCommit(blockHeight rpc.BlockHeight) *types.Commit {
	return s.kaiService.kaiDb.ReadCommit(blockHeight.Uint64())
}

// AccountResult is the result structs for GetProof
type AccountResult struct {
	Address      common.Address  `json:"address"`
	AccountProof []string        `json:"accountProof"`
	Balance      *common.Big     `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        common.Uint64   `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	StorageProof []StorageResult `json:"storageProof"`
}
type StorageResult struct {
	Key   string      `json:"key"`
	Value *common.Big `json:"value"`
	Proof []string    `json:"proof"`
}

// GetProof returns the Merkle-proof for a given account and optionally some storage keys.
func (s *PublicKaiAPI) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockHeightOrHash rpc.BlockHeightOrHash) (*AccountResult, error) {
	state, _, err := s.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}

	storageTrie := state.StorageTrie(address)
	storageHash := types.EmptyRootHash
	codeHash := state.GetCodeHash(address)
	storageProof := make([]StorageResult, len(storageKeys))

	// if we have a storageTrie, (which means the account exists), we can update the storageHash
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

// PendingTransactions returns pending transactions
func (a *PublicTransactionAPI) PendingTransactions() ([]*PublicTransaction, error) {
	pendingTxs := a.s.TxPool().GetPendingData()
	transactions := make([]*PublicTransaction, 0, len(pendingTxs))

	for _, tx := range pendingTxs {
		jsonData := NewPublicTransaction(a.s.Config(), tx, common.Hash{}, 0, 0)
		transactions = append(transactions, jsonData)
	}
	return transactions, nil
}

// GetTransaction gets transaction by transaction hash
func (a *PublicTransactionAPI) GetTransaction(hash string) (*PublicTransaction, error) {
	txHash := common.HexToHash(hash)
	tx, blockHash, height, index := a.s.kaiDb.ReadTransaction(txHash)

	if tx == nil {
		return nil, errors.New("tx for hash not found")
	}

	publicTx := NewPublicTransaction(a.s.Config(), tx, blockHash, height, index)
	// get block by block height
	block := a.s.blockchain.GetBlockByHeight(height)
	// get block time from block
	publicTx.Time = block.Header().Time
	return publicTx, nil
}

// getReceiptLogs gets logs from receipt
func getReceiptLogs(receipt types.Receipt) []Log {
	if receipt.Logs != nil {
		logs := make([]Log, 0, len(receipt.Logs))
		for _, l := range receipt.Logs {
			topics := make([]string, 0, len(l.Topics))
			for _, topic := range l.Topics {
				topics = append(topics, topic.Hex())
			}
			logs = append(logs, Log{
				Address:     l.Address.Hex(),
				Topics:      topics,
				Data:        l.Data.String(),
				BlockHeight: l.BlockHeight,
				TxHash:      l.TxHash.Hex(),
				TxIndex:     l.TxIndex,
				BlockHash:   l.BlockHash.Hex(),
				Index:       l.Index,
				Removed:     l.Removed,
			})
		}
		return logs
	}
	return nil
}

// getTransactionReceipt gets transaction receipt from transaction, blockHash, blockHeight and index.
func getPublicReceipt(config *configs.ChainConfig, receipt types.Receipt, tx *types.Transaction, blockHash common.Hash, blockHeight, index uint64) *PublicReceipt {
	from, _ := types.Sender(types.LatestSigner(config), tx)
	logs := getReceiptLogs(receipt)

	publicReceipt := &PublicReceipt{
		BlockHash:         blockHash.Hex(),
		BlockHeight:       blockHeight,
		TransactionHash:   tx.Hash().Hex(),
		TransactionIndex:  index,
		From:              from.Hex(),
		To:                "0x",
		GasUsed:           uint64(receipt.GasUsed),
		CumulativeGasUsed: uint64(receipt.CumulativeGasUsed),
		ContractAddress:   "0x",
		Logs:              logs,
		LogsBloom:         receipt.Bloom,
	}

	// To field is nil for contract creation tx.
	if tx.To() != nil {
		publicReceipt.To = tx.To().Hex()
	}
	publicReceipt.Status = uint(receipt.Status)
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		publicReceipt.ContractAddress = receipt.ContractAddress.Hex()
	}

	return publicReceipt
}

// GetTransactionReceipt gets transaction receipt from transaction, blockHash, blockHeight and index.
func (a *PublicTransactionAPI) GetTransactionReceipt(ctx context.Context, hash string) (*PublicReceipt, error) {
	txHash := common.HexToHash(hash)
	tx, blockHash, height, index := a.s.kaiDb.ReadTransaction(txHash)
	if tx == nil || height == 0 {
		return nil, nil
	}
	// get receipts from db
	blockInfo := a.s.BlockInfoByBlockHash(ctx, blockHash)
	if blockInfo == nil {
		return nil, ErrBlockInfoNotFound
	}
	// return the receipt if tx and receipt hashes at index are the same
	if len(blockInfo.Receipts) > int(index) && blockInfo.Receipts[index].TxHash.Equal(txHash) {
		receipt := blockInfo.Receipts[index]
		return getPublicReceipt(a.s.chainConfig, *receipt, tx, blockHash, height, index), nil
	}
	// else traverse receipts list to find the corresponding receipt of txHash
	for _, r := range blockInfo.Receipts {
		if !r.TxHash.Equal(txHash) {
			continue
		} else {
			receipt := r
			return getPublicReceipt(a.s.chainConfig, *receipt, tx, blockHash, height, index), nil
		}
	}

	// dirty hack searching receipt in the few previous block
	for i := uint64(1); i <= 2; i++ {
		block := a.s.BlockByHeight(ctx, rpc.BlockHeight(height-i))
		// get receipts from db
		blockInfo := a.s.BlockInfoByBlockHash(ctx, block.Hash())
		if blockInfo == nil {
			return nil, ErrBlockInfoNotFound
		}
		for _, r := range blockInfo.Receipts {
			if !r.TxHash.Equal(txHash) {
				continue
			} else {
				// update the correct lookup entry and try again
				a.s.kaiDb.WriteTxLookupEntries(block, block.Transactions())
				return a.GetTransactionReceipt(ctx, hash)
			}
		}
	}
	// return nil if not found
	return nil, nil
}

// GetRawTransactionByHash returns the bytes of the transaction for the given hash.
func (a *PublicTransactionAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (common.Bytes, error) {
	// Retrieve a finalized transaction, or a pooled otherwise
	tx, _, _, _ := a.s.GetTransaction(ctx, hash)
	if tx == nil {
		if tx = a.s.TxPool().Get(hash); tx == nil {
			// Transaction not found anywhere, abort
			return nil, nil
		}
	}
	// Serialize to RLP and return
	return tx.MarshalBinary()
}

// PublicAccountAPI provides APIs support getting account's info
type PublicAccountAPI struct {
	kaiService *KardiaService
}

// NewPublicAccountAPI is a constructor that init new PublicAccountAPI
func NewPublicAccountAPI(kaiService *KardiaService) *PublicAccountAPI {
	return &PublicAccountAPI{kaiService}
}

// Balance returns address's balance
func (a *PublicAccountAPI) Balance(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (string, error) {
	state, _, err := a.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return "", err
	}
	return state.GetBalance(address).String(), nil
}

// Nonce return address's nonce
func (a *PublicAccountAPI) Nonce(address string) (uint64, error) {
	addr := common.HexToAddress(address)
	nonce := a.kaiService.txPool.Nonce(addr)
	return nonce, nil
}

// NonceAtHeight the nonce of an address at the given height
func (a *PublicAccountAPI) NonceAtHeight(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (uint64, error) {
	// Ask transaction pool for the nonce, which includes pending transactions
	if blockHeight, ok := blockHeightOrHash.Height(); ok && blockHeight == rpc.PendingBlockHeight {
		return a.kaiService.txPool.Nonce(address), nil
	}
	// Resolve block height and use its state to ask for the nonce
	state, _, err := a.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return 0, err
	}
	nonce := state.GetNonce(address)
	return nonce, state.Error()
}

// GetCode returns the code stored at the given address in the state for the given block height.
func (a *PublicAccountAPI) GetCode(ctx context.Context, address common.Address, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	state, _, err := a.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block height. The rpc.LatestBlockHeight and rpc.PendingBlockHeight meta block
// heights are also allowed.
func (a *PublicAccountAPI) GetStorageAt(ctx context.Context, address common.Address, key string, blockHeightOrHash rpc.BlockHeightOrHash) (common.Bytes, error) {
	state, _, err := a.kaiService.StateAndHeaderByHeightOrHash(ctx, blockHeightOrHash)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicKaiAPI) GasPrice(ctx context.Context) (string, error) {
	price, err := s.kaiService.SuggestPrice(ctx)
	return price.String(), err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicKaiAPI) EstimateGas(ctx context.Context, args kaiapi.TransactionArgs, blockHeightOrHash *rpc.BlockHeightOrHash) (uint64, error) {
	bHeightOrHash := rpc.BlockHeightOrHashWithHeight(rpc.PendingBlockHeight)
	if blockHeightOrHash != nil {
		bHeightOrHash = *blockHeightOrHash
	}
	estimatedGas, err := kaiapi.DoEstimateGas(ctx, s.kaiService, args, bHeightOrHash, configs.GasLimitCap)
	return uint64(estimatedGas), err
}

// checkGas is a function used to check whether the fee of
// a transaction meets the requirements.
func checkGas(gasPrice *big.Int, gas uint64) error {
	if gasPrice == nil {
		return ErrNilGasPrice
	}
	if gasPrice.Cmp(configs.GasPriceCap) < 0 {
		return ErrNotEnoughGasPrice
	}
	if gas > configs.GasLimitCap {
		return ErrExceedGasLimit
	}
	return nil
}

// checkTxFee is an internal function used to check whether the fee of
// the given transaction is reasonable.
func checkTxFee(gasPrice *big.Int, gas uint64, cap float64) error {
	feeKAI := new(big.Float).Quo(new(big.Float).SetInt(new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gas))), new(big.Float).SetInt(big.NewInt(configs.KAI)))
	feeFloat, _ := feeKAI.Float64()
	if feeFloat > cap {
		return ErrTxFeeCap
	}
	return nil
}

// toHexSlice creates a slice of hex-strings based on []byte.
func toHexSlice(b [][]byte) []string {
	r := make([]string, len(b))
	for i := range b {
		r[i] = common.Encode(b[i])
	}
	return r
}
