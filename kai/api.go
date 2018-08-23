package kai

import (
	"fmt"
	"math/big"
	"context"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
)

// PublicKaiAPI provides an API to access Kai full node-related
// information.
type PublicKaiAPI struct {
	kaiService *Kardia
}

// NewPublicKaiAPI creates a new Kai protocol API for full nodes.
func NewPublicKaiAPI(kaiService *Kardia) *PublicKaiAPI {
	return &PublicKaiAPI{kaiService}
}

func (s *PublicKaiAPI) SendSignedTransaction() common.Uint64 {
	return common.Uint64(100)
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
	log.Info(fmt.Sprintf("RawTransaction: %v", txs))
	tx := new(types.Transaction)
	encodedTx := common.FromHex(txs)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}.Hex(), err
	}
	return tx.Hash().Hex(), a.s.TxPool().AddRemote(tx)
}


// PendingTransactions returns pending transactions
func (a *PublicTransactionAPI) PendingTransactions() ([]*PublicTransactionJSON, error) {
	pending, err := a.s.TxPool().Pending()
	if err != nil {
		return nil, err
	}

	transactions := make([]*PublicTransactionJSON, 0, len(pending))

	// loop through pending txs
	// stores tx hash into map to track duplicated txs
	for _, txs := range pending {
		for _, tx := range txs {
			jsonData := NewPublicTransactionJSON(newPublicTransaction(tx, common.Hash{}, 0, 0))
			transactions = append(transactions, jsonData)
		}
	}

	return transactions, nil
}
