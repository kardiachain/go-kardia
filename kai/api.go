package kai

import (
	"math/big"
	"context"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/blockchain/rawdb"
	"encoding/hex"

)


// BlockJSON represents Block in JSON format
type BlockJSON struct {
	Hash           string                      `json:"hash"`
	Height         uint64                      `json:"height"`
	LastBlock      string                      `json:"lastBlock"`
	CommitHash     string                      `json:"commitHash"`
	Time           *big.Int                    `json:"time"`
	NumTxs         uint64                      `json:"num_txs"`
	GasLimit       uint64                      `json:"gasLimit"`
	GasUsed        uint64                      `json:"gasUsed"`
	Validator      string                      `json:"validator"`
	TxHash         string                      `json:"data_hash"` // transactions
	Root           string                      `json:"stateRoot"` // state root
	ReceiptHash    string                      `json:"receiptsRoot"` // receipt root
	Bloom          int64                       `json:"logsBloom"`
	ValidatorsHash string                      `json:"validators_hash"` // validators for the current block
	ConsensusHash  string                      `json:"consensus_hash"`
	Txs            []*PublicTransactionJSON    `json:"txs"`
}


// PublicKaiAPI provides APIs to access Kai full node-related
// information.
type PublicKaiAPI struct {
	kaiService *Kardia
}

// NewPublicKaiAPI creates a new Kai protocol API for full nodes.
func NewPublicKaiAPI(kaiService *Kardia) *PublicKaiAPI {
	return &PublicKaiAPI{kaiService}
}


// NewBlockJSON creates a new Block JSON data from Block
func NewBlockJSON(block types.Block) *BlockJSON {
	txs := block.Transactions()
	transactions := make([]*PublicTransactionJSON, 0, len(txs))
	for index, tx := range txs {
		json := NewPublicTransactionJSON(newPublicTransaction(tx, block.Hash(), block.Height(), uint64(index)))
		transactions = append(transactions, json)
	}

	return &BlockJSON{
		Hash: block.Hash().Hex(),
		Height: block.Height(),
		LastBlock: block.Header().LastBlockID.String(),
		Txs: transactions,
		CommitHash: block.LastCommitHash().Hex(),
		Time: block.Header().Time,
		NumTxs: block.Header().NumTxs,
		GasLimit: block.Header().GasLimit,
		GasUsed: block.Header().GasUsed,
		Validator: block.Header().Coinbase.Hex(),
		TxHash: block.Header().TxHash.Hex(),
		Root: block.Header().Root.Hex(),
		ReceiptHash: block.Header().ReceiptHash.Hex(),
		Bloom: block.Header().Bloom.Big().Int64(),
		ValidatorsHash: block.Header().ValidatorsHash.Hex(),
		ConsensusHash: block.Header().ConsensusHash.Hex(),
	}
}


// BlockNumber returns current block number
func (s *PublicKaiAPI) BlockNumber() uint64 {
	return s.kaiService.blockchain.CurrentBlock().Height()
}


// GetBlockByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockByHash(blockHash string) *BlockJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.kaiService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	return NewBlockJSON(*block)
}


// GetBlockByNumber returns block by block number
func (s *PublicKaiAPI) GetBlockByNumber(blockNumber uint64) *BlockJSON {
	block := s.kaiService.blockchain.GetBlockByHeight(blockNumber)
	return NewBlockJSON(*block)
}


// PublicTransaction represents a transaction that will serialize to the RPC representation of a transaction
type PublicTransaction struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *common.Big     `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              common.Uint64   `json:"gas"`
	GasPrice         *common.Big     `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            common.Bytes    `json:"input"`
	Nonce            common.Uint64   `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex uint            `json:"transactionIndex"`
	Value            *common.Big     `json:"value"`
}

type PublicTransactionJSON struct {
	BlockHash        string     	`json:"blockHash"`
	BlockNumber      *common.Big    `json:"blockNumber"`
	From             string     	`json:"from"`
	Gas              common.Uint64  `json:"gas"`
	GasPrice         *common.Big    `json:"gasPrice"`
	Hash             string     	`json:"hash"`
	Input            string     	`json:"input"`
	Nonce            common.Uint64  `json:"nonce"`
	To               string     	`json:"to"`
    TransactionIndex uint       	`json:"transactionIndex"`
	Value            *common.Big    `json:"value"`
}

type Log struct {
	Address      string           `json:"address"`
	Topics       []string         `json:"topics"`
	Data         string           `json:"data"`
	BlockHeight  uint64           `json:"blockHeight"`
	TxHash       string           `json:"transactionHash"`
	TxIndex      uint             `json:"transactionIndex"`
	BlockHash    string           `json:"blockHash"`
	Index        uint             `json:"logIndex"`
	Removed      bool             `json:"removed"`
}


// newPublicTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newPublicTransaction(tx *types.Transaction, blockHash common.Hash, blockNumber uint64, index uint64) *PublicTransaction {
	from, _ := types.Sender(tx)

	result := &PublicTransaction{
		From:     from,
		Gas:      common.Uint64(tx.Gas()),
		GasPrice: (*common.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    common.Bytes(tx.Data()),
		Nonce:    common.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*common.Big)(tx.Value()),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*common.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = uint(index)
	}
	return result
}


// PublicTransactionAPI provides public apis relate to transactions
type PublicTransactionAPI struct {
	s *Kardia
}


// NewPublicTransactionAPI is a constructor of PublicTransactionAPI
func NewPublicTransactionAPI(service *Kardia) *PublicTransactionAPI {
	return &PublicTransactionAPI{service}
}


// NewPublicTransactionJSON is a constructor of PublicTransactionJSON
func NewPublicTransactionJSON(tx *PublicTransaction) *PublicTransactionJSON {
	result := &PublicTransactionJSON{
		From: tx.From.Hex(),
		Gas: tx.Gas,
		GasPrice: tx.GasPrice,
		Hash: tx.Hash.Hex(),
		Input: common.Encode(tx.Input),
		Nonce: tx.Nonce,
		To: tx.To.Hex(),
		Value: tx.Value,
	}

	if tx.BlockHash != (common.Hash{}) {
		result.BlockHash = tx.BlockHash.Hex()
		result.BlockNumber = tx.BlockNumber
		result.TransactionIndex = tx.TransactionIndex
	}

	return result
}


// SendRawTransaction decode encoded data into tx and then add tx into pool
func (a *PublicTransactionAPI) SendRawTransaction(ctx context.Context, txs string) (string, error) {
	tx := new(types.Transaction)
	encodedTx := common.FromHex(txs)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}.Hex(), err
	}
	if err := a.s.TxPool().AddRemote(tx); err != nil {
		return common.Hash{}.Hex(), err
	}
	return tx.Hash().Hex(), nil
}


// PendingTransactions returns pending transactions
func (a *PublicTransactionAPI) PendingTransactions() ([]*PublicTransactionJSON, error) {
	pending, err := a.s.TxPool().Pending()
	if err != nil {
		return nil, err
	}

	transactions := make([]*PublicTransactionJSON, 0, len(pending))

	// loop through pending txs
	for _, txs := range pending {
		for _, tx := range txs {
			jsonData := NewPublicTransactionJSON(newPublicTransaction(tx, common.Hash{}, 0, 0))
			transactions = append(transactions, jsonData)
		}
	}

	return transactions, nil
}


// GetTransaction gets transaction by transaction hash
func (a *PublicTransactionAPI) GetTransaction(hash string) *PublicTransactionJSON {
	txHash := common.HexToHash(hash)
	tx, blockHash, height, index := rawdb.ReadTransaction(a.s.chainDb, txHash)
	return NewPublicTransactionJSON(newPublicTransaction(tx, blockHash, height, index))
}


func (a *PublicTransactionAPI) getReceipts(hash common.Hash) (types.Receipts, error) {
	height := rawdb.ReadHeaderNumber(a.s.chainDb, hash)
	if height == nil {
		return nil, nil
	}
	return rawdb.ReadReceipts(a.s.chainDb, hash, *height), nil
}


// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (a *PublicTransactionAPI) GetTransactionReceipt(ctx context.Context, hash string) (map[string]interface{}, error) {
	txHash := common.HexToHash(hash)
	tx, blockHash, height, index := rawdb.ReadTransaction(a.s.chainDb, txHash)
	if tx == nil {
		return nil, nil
	}
	receipts, err := a.getReceipts(blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]
	from, _ := types.Sender(tx)
	logs := make([]Log, 0)

	if receipt.Logs != nil {
		logs := make([]Log, 0, len(receipt.Logs))
		for _, log := range receipt.Logs {
			topics := make([]string, 0, len(log.Topics))
			for _, topic := range log.Topics {
				topics = append(topics, topic.Hex())
			}
			logs = append(logs, Log{
				Address:     log.Address.Hex(),
				Topics:      topics,
				Data:        hex.EncodeToString(log.Data),
				BlockHeight: log.BlockHeight,
				TxHash:      log.TxHash.Hex(),
				TxIndex:     log.TxIndex,
				BlockHash:   log.BlockHash.Hex(),
				Index:       log.Index,
				Removed:     log.Removed,
			})
		}
	}


	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockHeight":       uint64(height),
		"transactionHash":   hash,
		"transactionIndex":  uint64(index),
		"from":              from.Hex(),
		"to":                tx.To().Hex(),
		"gasUsed":           uint64(receipt.GasUsed),
		"cumulativeGasUsed": uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              logs,
		"logsBloom":         receipt.Bloom.Big().Int64(),
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = common.Bytes(receipt.PostState)
	} else {
		fields["status"] = uint(receipt.Status)
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}


// PublicAccountAPI provides APIs support getting account's info
type PublicAccountAPI struct {
	kaiService *Kardia
}


// NewPublicAccountAPI is a constructor that init new PublicAccountAPI
func NewPublicAccountAPI(kaiService *Kardia) *PublicAccountAPI {
	return &PublicAccountAPI{kaiService}
}


// Balance returns address's balance
func (a *PublicAccountAPI)Balance(address string, hash string, height int64) int64 {
	addr := common.HexToAddress(address)
	block := new(types.Block)
	if len(hash) > 0 && height >= 0 {
		block = a.kaiService.blockchain.GetBlock(common.HexToHash(hash), uint64(height))
	} else if len(hash) > 0 {
		block = a.kaiService.blockchain.GetBlockByHash(common.HexToHash(hash))
	} else if height >= 0 {
		block = a.kaiService.blockchain.GetBlockByHeight(uint64(height))
	} else {
		block = a.kaiService.blockchain.CurrentBlock()
	}
	ba := block.Accounts().GetAccount(&addr)
	return ba.Balance.Int64()
}
