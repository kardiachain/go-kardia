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
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	defaultTimeOutForStaticCall = 5
)

// BlockHeaderJSON represents BlockHeader in JSON format
type BlockHeaderJSON struct {
	Hash              string    `json:"hash"`
	Height            uint64    `json:"height"`
	LastBlock         string    `json:"lastBlock"`
	CommitHash        string    `json:"commitHash"`
	Time              time.Time `json:"time"`
	NumTxs            uint64    `json:"numTxs"`
	GasLimit          uint64    `json:"gasLimit"`
	GasUsed           uint64    `json:"gasUsed"`
	ProposerAddress   string    `json:"proposerAddress"`
	TxHash            string    `json:"dataHash"`     // transactions
	ReceiptHash       string    `json:"receiptsRoot"` // receipt root
	Bloom             string    `json:"logsBloom"`
	ValidatorsHash    string    `json:"validatorHash"`     // current block validators hash
	NextValidatorHash string    `json:"nextValidatorHash"` // next block validators hash
	ConsensusHash     string    `json:"consensusHash"`     // current consensus hash
	AppHash           string    `json:"appHash"`           // state of transactions
	EvidenceHash      string    `json:"evidenceHash"`      // hash of evidence
}

// BlockJSON represents Block in JSON format
type BlockJSON struct {
	Hash              string               `json:"hash"`
	Height            uint64               `json:"height"`
	LastBlock         string               `json:"lastBlock"`
	CommitHash        string               `json:"commitHash"`
	Time              time.Time            `json:"time"`
	NumTxs            uint64               `json:"numTxs"`
	GasLimit          uint64               `json:"gasLimit"`
	GasUsed           uint64               `json:"gasUsed"`
	Rewards           string               `json:"rewards"`
	ProposerAddress   string               `json:"proposerAddress"`
	TxHash            string               `json:"dataHash"`     // hash of txs
	ReceiptHash       string               `json:"receiptsRoot"` // receipt root
	Bloom             string               `json:"logsBloom"`
	ValidatorsHash    string               `json:"validatorHash"`     // validators for the current block
	NextValidatorHash string               `json:"nextValidatorHash"` // validators for the current block
	ConsensusHash     string               `json:"consensusHash"`     // hash of current consensus
	AppHash           string               `json:"appHash"`           // txs state
	EvidenceHash      string               `json:"evidenceHash"`      // hash of evidence
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
		ContractAddress:   "0x",
		Logs:              logs,
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		basicReceipt.Root = common.Bytes(receipt.PostState)
	} else {
		basicReceipt.Status = uint(receipt.Status)
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		basicReceipt.ContractAddress = receipt.ContractAddress.Hex()
	}

	return &basicReceipt
}

// NewBlockHeaderJSON creates a new BlockHeader JSON data from Block
func NewBlockHeaderJSON(block types.Block) *BlockHeaderJSON {
	return &BlockHeaderJSON{
		Hash:              block.Hash().Hex(),
		Height:            block.Height(),
		LastBlock:         block.Header().LastBlockID.Hash.Hex(),
		CommitHash:        block.LastCommitHash().Hex(),
		Time:              block.Header().Time,
		NumTxs:            block.Header().NumTxs,
		GasLimit:          block.Header().GasLimit,
		GasUsed:           block.Header().GasUsed,
		ProposerAddress:   block.Header().ProposerAddress.Hex(),
		TxHash:            block.Header().TxHash.Hex(),
		ValidatorsHash:    block.Header().ValidatorsHash.Hex(),
		NextValidatorHash: block.Header().NextValidatorsHash.Hex(),
		ConsensusHash:     block.Header().ConsensusHash.Hex(),
		AppHash:           block.Header().AppHash.Hex(),
		EvidenceHash:      block.Header().EvidenceHash.Hex(),
	}
}

// NewBasicBlockJSON creates a new Block JSON data from Block
func NewBasicBlockJSON(block types.Block) *BlockJSON {
	txs := block.Transactions()
	transactions := make([]*PublicTransaction, 0, len(txs))

	for index, transaction := range txs {
		idx := uint64(index)
		tx := NewPublicTransaction(transaction, block.Hash(), block.Height(), idx)
		// add time for tx
		tx.Time = block.Header().Time
		transactions = append(transactions, tx)
	}

	return &BlockJSON{
		Hash:              block.Hash().Hex(),
		Height:            block.Height(),
		LastBlock:         block.Header().LastBlockID.Hash.Hex(),
		Txs:               transactions,
		CommitHash:        block.LastCommitHash().Hex(),
		Time:              block.Header().Time,
		NumTxs:            block.Header().NumTxs,
		GasLimit:          block.Header().GasLimit,
		GasUsed:           block.Header().GasUsed,
		ProposerAddress:   block.Header().ProposerAddress.Hex(),
		TxHash:            block.Header().TxHash.Hex(),
		ValidatorsHash:    block.Header().ValidatorsHash.Hex(),
		NextValidatorHash: block.Header().NextValidatorsHash.Hex(),
		ConsensusHash:     block.Header().ConsensusHash.Hex(),
		AppHash:           block.Header().AppHash.Hex(),
		EvidenceHash:      block.Header().EvidenceHash.Hex(),
	}
}

// NewBlockJSON creates a new Block JSON data from Block
func NewBlockJSON(block types.Block, blockInfo types.BlockInfo) *BlockJSON {
	txs := block.Transactions()
	transactions := make([]*PublicTransaction, 0, len(txs))
	basicReceipts := make([]*BasicReceipt, 0)

	for _, receipt := range blockInfo.Receipts {
		basicReceipts = append(basicReceipts, getBasicReceipt(*receipt))
	}

	for index, transaction := range txs {
		idx := uint64(index)
		tx := NewPublicTransaction(transaction, block.Hash(), block.Height(), idx)
		// add time for tx
		tx.Time = block.Header().Time
		transactions = append(transactions, tx)
	}

	return &BlockJSON{
		Hash:              block.Hash().Hex(),
		Height:            block.Height(),
		LastBlock:         block.Header().LastBlockID.Hash.Hex(),
		Txs:               transactions,
		CommitHash:        block.LastCommitHash().Hex(),
		Time:              block.Header().Time,
		NumTxs:            block.Header().NumTxs,
		GasLimit:          block.Header().GasLimit,
		GasUsed:           blockInfo.GasUsed,
		Rewards:           blockInfo.Rewards.String(),
		ProposerAddress:   block.Header().ProposerAddress.Hex(),
		TxHash:            block.Header().TxHash.Hex(),
		ValidatorsHash:    block.Header().ValidatorsHash.Hex(),
		NextValidatorHash: block.Header().NextValidatorsHash.Hex(),
		ConsensusHash:     block.Header().ConsensusHash.Hex(),
		AppHash:           block.Header().AppHash.Hex(),
		EvidenceHash:      block.Header().EvidenceHash.Hex(),
		Receipts:          basicReceipts,
	}
}

// BlockNumber returns current block number
func (s *PublicKaiAPI) BlockNumber() uint64 {
	return s.kaiService.blockchain.CurrentBlock().Height()
}

// GetHeaderBlockByNumber returns blockHeader by block number
func (s *PublicKaiAPI) GetBlockHeaderByNumber(blockNumber uint64) *BlockHeaderJSON {
	block := s.kaiService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil
	}
	return NewBlockHeaderJSON(*block)
}

// GetBlockHeaderByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockHeaderByHash(blockHash string) *BlockHeaderJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.kaiService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	return NewBlockHeaderJSON(*block)
}

// GetBasicBlockByHash returns block by block hash
func (s *PublicKaiAPI) GetBasicBlockByHash(blockHash string) *BlockJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.kaiService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	return NewBasicBlockJSON(*block)
}

// GetBasicBlockByNumber returns block by block number
func (s *PublicKaiAPI) GetBasicBlockByNumber(blockNumber uint64) *BlockJSON {
	block := s.kaiService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil
	}
	return NewBasicBlockJSON(*block)
}

// GetBlockByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockByHash(blockHash string) (*BlockJSON, error) {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.kaiService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	if block == nil {
		return nil, errors.New("block for hash not found")
	}

	blockInfo, err := getBlockInfo(s.kaiService.kaiDb, block.Hash())
	if err != nil {
		return nil, err
	}
	return NewBlockJSON(*block, *blockInfo), nil
}

// GetBlockByNumber returns block by block number
func (s *PublicKaiAPI) GetBlockByNumber(blockNumber uint64) (*BlockJSON, error) {
	block := s.kaiService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil, errors.New("block for height not found")
	}
	blockInfo, err := getBlockInfo(s.kaiService.kaiDb, block.Hash())
	if err != nil {
		return nil, err
	}
	return NewBlockJSON(*block, *blockInfo), nil
}

// Validator returns node's validator, nil if current node is not a validator
// TODO @trinhdn: get validators' info from staking smart contract
func (s *PublicKaiAPI) Validator(ctx context.Context, valAddr common.Address) (*types.Validator, error) {
	val, err := s.kaiService.csManager.GetValidator(valAddr)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Validators returns a list of validator
// TODO @trinhdn: get validators' info from staking smart contract
func (s *PublicKaiAPI) Validators(ctx context.Context) ([]*types.Validator, error) {
	val, err := s.kaiService.csManager.GetValidators()
	if err != nil {
		return nil, err
	}
	return val, nil
}

type PublicTransaction struct {
	BlockHash        string    `json:"blockHash"`
	BlockNumber      uint64    `json:"blockNumber"`
	Time             time.Time `json:"time"`
	From             string    `json:"from"`
	Gas              uint64    `json:"gas"`
	GasPrice         uint64    `json:"gasPrice"`
	Hash             string    `json:"hash"`
	Input            string    `json:"input"`
	Nonce            uint64    `json:"nonce"`
	To               string    `json:"to"`
	TransactionIndex uint      `json:"transactionIndex"`
	Value            string    `json:"value"`
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
	BlockHash         string       `json:"blockHash"`
	BlockHeight       uint64       `json:"blockHeight"`
	TransactionHash   string       `json:"transactionHash"`
	TransactionIndex  uint64       `json:"transactionIndex"`
	From              string       `json:"from"`
	To                string       `json:"to"`
	GasUsed           uint64       `json:"gasUsed"`
	CumulativeGasUsed uint64       `json:"cumulativeGasUsed"`
	ContractAddress   string       `json:"contractAddress"`
	Logs              []Log        `json:"logs"`
	LogsBloom         types.Bloom  `json:"logsBloom"`
	Root              common.Bytes `json:"root"`
	Status            uint         `json:"status"`
}

type BasicReceipt struct {
	TransactionHash   string       `json:"transactionHash"`
	GasUsed           uint64       `json:"gasUsed"`
	CumulativeGasUsed uint64       `json:"cumulativeGasUsed"`
	ContractAddress   string       `json:"contractAddress"`
	Logs              []Log        `json:"logs"`
	Root              common.Bytes `json:"root"`
	Status            uint         `json:"status"`
}

// NewPublicTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewPublicTransaction(tx *types.Transaction, blockHash common.Hash, blockNumber uint64, index uint64) *PublicTransaction {
	from, _ := types.Sender(types.FrontierSigner{}, tx)

	result := &PublicTransaction{
		From:     from.Hex(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice().Uint64(),
		Hash:     tx.Hash().Hex(),
		Input:    common.Encode(tx.Data()),
		Nonce:    tx.Nonce(),
		Value:    tx.Value().String(),
	}
	if tx.To() != nil {
		result.To = tx.To().Hex()
	} else {
		result.To = "0x"
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash.Hex()
		result.BlockNumber = blockNumber
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
	return tx.Hash().Hex(), a.s.TxPool().AddLocal(tx)
}

// KardiaCall execute a contract method call only against
// state on the local node. No tx is generated and submitted
// onto the blockchain
func (s *PublicKaiAPI) KardiaCall(ctx context.Context, call types.CallArgsJSON, blockNumber uint64) (string, error) {
	args := types.NewArgs(call)
	log.Info("KardiaCall callargs:", "args", args)
	result, err := s.doCall(ctx, args, blockNrOrHash, kvm.Config{}, configs.DefaultTimeOutForStaticCall*time.Second)
	return common.Encode(result.ReturnData), err
}

// PendingTransactions returns pending transactions
func (a *PublicTransactionAPI) PendingTransactions() ([]*PublicTransaction, error) {
	pendingTxs := a.s.TxPool().GetPendingData()
	transactions := make([]*PublicTransaction, 0, len(pendingTxs))

	for _, tx := range pendingTxs {
		jsonData := NewPublicTransaction(tx, common.Hash{}, 0, 0)
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

	publicTx := NewPublicTransaction(tx, blockHash, height, index)
	// get block by block height
	block := a.s.blockchain.GetBlockByHeight(height)
	// get block time from block
	publicTx.Time = block.Header().Time
	return publicTx, nil
}

func getBlockInfo(kaiDb types.StoreDB, hash common.Hash) (*types.BlockInfo, error) {
	height := kaiDb.ReadHeaderNumber(hash)
	if height == nil {
		return nil, nil
	}

	return kaiDb.ReadBlockInfo(hash, *height), nil

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
				Data:        hex.EncodeToString(l.Data),
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

// getTransactionReceipt gets transaction receipt from transaction, blockHash, blockNumber and index.
func getPublicReceipt(receipt types.Receipt, tx *types.Transaction, blockHash common.Hash, blockNumber, index uint64) *PublicReceipt {
	from, _ := types.Sender(types.HomesteadSigner{}, tx)
	logs := getReceiptLogs(receipt)

	publicReceipt := &PublicReceipt{
		BlockHash:         blockHash.Hex(),
		BlockHeight:       uint64(blockNumber),
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
	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		publicReceipt.Root = common.Bytes(receipt.PostState)
	} else {
		publicReceipt.Status = uint(receipt.Status)
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		publicReceipt.ContractAddress = receipt.ContractAddress.Hex()
	}

	return publicReceipt
}

// GetPublicReceipt returns the public receipt for the given transaction hash.
func (a *PublicTransactionAPI) GetTransactionReceipt(ctx context.Context, hash string) (*PublicReceipt, error) {
	txHash := common.HexToHash(hash)
	tx, blockHash, height, index := a.s.kaiDb.ReadTransaction(txHash)
	if tx == nil {
		return nil, nil
	}
	// get receipts from db
	blockInfo, err := getBlockInfo(a.s.kaiDb, blockHash)
	if err != nil {
		return nil, err
	}
	if len(blockInfo.Receipts) <= int(index) {
		return nil, nil
	}
	receipt := blockInfo.Receipts[index]
	return getPublicReceipt(*receipt, tx, blockHash, height, index), nil
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
func (a *PublicAccountAPI) Balance(address string, hash string, height uint64) (string, error) {
	addr := common.HexToAddress(address)
	log.Info("Addr", "addr", addr.Hex(), "hash", hash, "height", height)
	block := new(types.Block)
	if len(hash) > 0 && height > 0 {
		block = a.kaiService.blockchain.GetBlock(common.HexToHash(hash), height)
	} else if len(hash) > 0 {
		block = a.kaiService.blockchain.GetBlockByHash(common.HexToHash(hash))
	} else if height > 0 {
		block = a.kaiService.blockchain.GetBlockByHeight(height)
	} else {
		block = a.kaiService.blockchain.CurrentBlock()
	}

	if block == nil {
		return "-1", errors.New("block for hash or height not found")
	}

	state, err := a.kaiService.blockchain.StateAt(block.Height())
	if err != nil {
		return "-1", err
	}
	return state.GetBalance(addr).String(), nil
}

// Nonce return address's nonce
func (a *PublicAccountAPI) Nonce(address string) (uint64, error) {
	addr := common.HexToAddress(address)
	nonce := a.kaiService.txPool.Nonce(addr)
	return nonce, nil
}

// doCall is an interface to make smart contract call against the state of local node
// No tx is generated or submitted to the blockchain
func (s *PublicKaiAPI) doCall(ctx context.Context, args *types.CallArgs, blockNr uint64, vmCfg kvm.Config, timeout time.Duration) ([]byte, uint64, bool, error) {
	defer func(start time.Time) { log.Debug("Executing KVM call finished", "runtime", time.Since(start)) }(time.Now())
	log.Info("doCall callargs:", "args", args)
	state, header, err := s.kaiService.APIBackend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}

	if statedb == nil || err != nil {
		return nil, 0, false, err
	}
	// Set sender address or use a default if none specified
	addr := args.From

	if addr == (common.Address{}) {
		addr = configs.GenesisDeployerAddr
	}

	// Set default gas & gas price if none were set
	gas, gasPrice := uint64(args.Gas), args.GasPrice

	if gas == 0 {
		gas = common.MaxInt64 / 2
	}
	if gasPrice.Sign() == 0 {
		gasPrice = new(big.Int).SetUint64(configs.TxGas)
	}

	// Create new call message
	msg := types.NewMessage(addr, args.To, 0, args.Value, gas, gasPrice, args.Data, false)
	log.Info("doCall msg:", "msg", msg)
	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Create a new context to be used in the KVM environment
	context := vm.NewKVMContext(msg, header, s.kaiService.BlockChain())
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	kvm := kvm.NewKVM(context, statedb, vmCfg)
	// Wait for the context to be done and cancel the KVM. Even if the
	// KVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		kvm.Cancel()
	}()
	// Apply the transaction to the current state (included in the env)
	gp := new(types.GasPool).AddGas(common.MaxUint64)
	res, gas, failed, err := blockchain.ApplyMessage(kvm, msg, gp)
	if err != nil {
		log.Info("doCall err:", "err", err)
		return nil, err
	}

	// If the timer caused an abort, return an appropriate error message
	if kvm.Cancelled() {
		return nil, 0, false, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}

	return res, gas, failed, err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicKaiAPI) EstimateGas(ctx context.Context, call types.CallArgsJSON) (uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  = configs.TxGas - 1
		hi  uint64
		cap uint64
	)
	args := types.NewArgs(call)
	if args.Gas >= configs.TxGas {
		hi = args.Gas
	} else {
		// Retrieve the current pending block to act as the gas ceiling
		block := s.kaiService.BlockChain().CurrentBlock()
		hi = block.GasLimit()
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = gas

		_, _, failed, err := s.doCall(ctx, args, s.BlockNumber(), kvm.Config{}, 0)
		if err != nil || failed {
			return false
		}
		return true
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance or always failing transaction")
		}
	}
	return hi, nil
}
