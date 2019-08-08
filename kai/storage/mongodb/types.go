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

package mongodb

import (
"github.com/kardiachain/go-kardia/lib/common"
"github.com/kardiachain/go-kardia/lib/log"
"github.com/kardiachain/go-kardia/types"
"math/big"
)

const (
	blockTable = "Block"
	txTable = "Transaction"
	dualEvtTable = "DualEvent"
	receiptTable = "Receipt"
	commitTable = "Commit"
	headHeaderTable = "HeadHeader"
	headBlockTable = "HeadBlock"
	chainConfigTable = "ChainConfig"
	trieTable = "Trie"
	txLookupEntryTable = "TxLookupEntry"
)

type Header struct {
	Height uint64                         `json:"height"        bson:"height"`
	Time   uint64                       `json:"time"          bson:"time"`
	NumTxs uint64                         `json:"numTxs"        bson:"numTxs"`
	NumDualEvents uint64                  `json:"numDualEvents" bson:"numDualEvents"`

	GasLimit uint64                       `json:"gasLimit"         bson:"gasLimit"`
	GasUsed  uint64                       `json:"gasUsed"          bson:"gasUsed"`

	// prev block info
	LastBlockID string                    `json:"lastBlockID"      bson:"lastBlockID"`
	Coinbase string                       `json:"miner"            bson:"miner"` // address

	// hashes of block data
	LastCommitHash string                 `json:"lastCommitHash"      bson:"lastCommitHash"` // commit from validators from the last block
	TxHash         string                 `json:"txHash"              bson:"txHash"` // transactions
	DualEventsHash string                 `json:"dualEventsHash"      bson:"dualEventsHash"` // dual's events
	Root           string                 `json:"stateRoot"           bson:"stateRoot"` // state root
	ReceiptHash    string                 `json:"receiptsRoot"        bson:"receiptsRoot"` // receipt root
	Bloom          string                 `json:"logsBloom"           bson:"logsBloom"`

	ValidatorsHash string                 `json:"validators_hash"` // validators for the current block
	ConsensusHash  string                 `json:"consensus_hash"`  // consensus params for current block
}

type Block struct {
	Header Header `json:"header"    bson:"header"`
	Hash   string `json:"hash"      bson:"hash"`
	Height uint64 `json:"height"    bson:"height"`
}

type Receipt struct {
	BlockHash         string  `json:"blockHash"         bson:"blockHash"`
	Height            uint64  `json:"height"            bson:"height"`
	PostState         string  `json:"root"              bson:"root"`
	Status            uint64  `json:"status"            bson:"status"`
	CumulativeGasUsed uint64  `json:"cumulativeGasUsed" bson:"cumulativeGasUsed"`
	Bloom             string  `json:"logsBloom"         bson:"logsBloom"`
	Logs              []*Log  `json:"logs"              bson:"logs"`
	TxHash          string    `json:"transactionHash"   bson:"transactionHash"`
	ContractAddress string    `json:"contractAddress"   bson:"contractAddress"`
	GasUsed         uint64    `json:"gasUsed"           bson:"gasUsed"`
}

type Log struct {
	Address      string      `json:"address"          bson:"address"`
	Topics       []string    `json:"topics"           bson:"topics"`
	Data         string      `json:"data"             bson:"data"`
	BlockHeight  uint64      `json:"blockHeight"      bson:"blockHeight"`
	TxHash       string      `json:"transactionHash"  bson:"transactionHash"`
	TxIndex      uint        `json:"transactionIndex" bson:"transactionIndex"`
	BlockHash    string      `json:"blockHash"        bson:"blockHash"`
	Index        uint        `json:"logIndex"         bson:"logIndex"`
	Removed      bool        `json:"removed"          bson:"removed"`
}

type Transaction struct {
	From         string          `json:"from"         bson:"from"`
	AccountNonce uint64          `json:"nonce"        bson:"nonce"`
	Price        string          `json:"gasPrice"     bson:"gasPrice"`
	GasLimit     uint64          `json:"gas"          bson:"gas"`
	Recipient    string          `json:"to"           bson:"to"` // nil means contract creation
	Amount       string          `json:"value"        bson:"value"`
	Payload      string          `json:"input"        bson:"input"`

	// Signature values
	V string                     `json:"v"            bson:"v"`
	R string                     `json:"r"            bson:"r"`
	S string                     `json:"s"            bson:"s"`

	// This is only used when marshaling to JSON.
	Hash string                  `json:"hash"         bson:"hash"`
	BlockHash string             `json:"blockHash"    bson:"blockHash"`
	Height    uint64             `json:"height"       bson:"height"`
	Index     int                `json:"index"        bson:"index"`
}

type DualEvent struct {
	Nonce             uint64                `json:"nonce"                bson:"nonce"`
	TriggeredEvent    *types.EventData      `json:"triggeredEvent"       bson:"triggeredEvent"`
	PendingTxMetadata *types.TxMetadata     `json:"pendingTxMetadata"    bson:"pendingTxMetadata"`
	KardiaSmcs []*types.KardiaSmartcontract `json:"kardiaSmcs"           bson:"kardiaSmcs"`
	Hash              string                `json:"hash"                 bson:"hash"`
	BlockHash         string                `json:"blockHash"            bson:"blockHash"`
	Height            uint64                `json:"height"               bson:"height"`
}

type EventData struct {
	TxHash       string                  `json:"txHash"           bson:"txHash"`
	TxSource     string                  `json:"txSource"         bson:"txSource"`
	FromExternal bool                    `json:"fromExternal"     bson:"fromExternal"`
	Data         *types.EventSummary     `json:"data"             bson:"data"`
	Actions      *types.DualActions      `json:"actions"          bson:"actions"`
}

type EventSummary struct {
	TxMethod string   `json:"txMethod"           bson:"txMethod"`// Smc's method
	TxValue  *big.Int `json:"txValue"            bson:"txValue"`// Amount of the tx
	ExtData  []string `json:"extData"            bson:"extData"`// Additional data along with this event
}

type DualActions struct {
	Actions []*DualAction
}

type DualAction struct {
	Name string           `json:"name"           bson:"name"`
}

type KardiaSmartcontract struct {
	EventWatcher *types.Watcher   `json:"eventWatcher"           bson:"eventWatcher"`
	Actions      *DualActions     `json:"actions"                bson:"actions"`
}

type Watcher struct {
	SmcAddress string       `json:"smcAddress"           bson:"smcAddress"`
	WatcherAction string    `json:"watcherAction"        bson:"watcherAction"`
}

type Commit struct {
	Height     uint64  `json:"height"           bson:"height"`
	BlockID    string  `json:"blockID"          bson:"blockID"`
	Precommits []*Vote `json:"precommits"       bson:"precommits"`
}

type Vote struct {
	ValidatorAddress string      `json:"validatorAddress"           bson:"validatorAddress"`
	ValidatorIndex   int64       `json:"validatorIndex"             bson:"validatorIndex"`
	Height           int64       `json:"height"                     bson:"height"`
	Round            int64       `json:"round"                      bson:"round"`
	Timestamp        *big.Int    `json:"timestamp"                  bson:"timestamp"`
	Type             byte        `json:"type"                       bson:"type"`
	BlockID          string      `json:"blockID"                    bson:"blockID"`
	Signature        string      `json:"signature"                  bson:"signature"`
}

type HeadHeaderHash struct {
	ID           int         `json:"ID"      bson:"ID"`
	Hash         string      `json:"hash"    bson:"hash"`
}

type HeadBlockHash struct {
	ID           int         `json:"ID"      bson:"ID"`
	Hash         string      `json:"hash"    bson:"hash"`
}

type ChainConfig struct {
	Hash   string `json:"hash"     bson:"hash"`
	Period uint64 `json:"period"   bson:"period"`
	Epoch  uint64 `json:"epoch"    bson:"epoch"`
}

type Caching struct {
	Key   string `json:"key"       bson:"key"`
	Value string `json:"value"     bson:"value"`
}

type TxLookupEntry struct {
	TxHash     string   `json:"txHash"     bson:"txHash"`
	BlockHash  string   `json:"blockHash"  bson:"blockHash"`
	BlockIndex uint64   `json:"blockIndex" bson:"blockIndex"`
	Index      uint64   `json:"index"      bson:"index"`
}

func NewBlock(block *types.Block) *Block {
	header := Header{
		Height: block.Header().Height,
		Time: 0,
		LastBlockID: block.Header().LastBlockID.String(),
		NumTxs: block.NumTxs(),
		TxHash: block.Header().TxHash.Hex(),
		GasUsed: block.Header().GasUsed,
		Bloom: common.Bytes2Hex(block.Header().Bloom.Bytes()),
		Coinbase: block.Header().Coinbase.Hex(),
		ConsensusHash: block.Header().ConsensusHash.Hex(),
		DualEventsHash: block.Header().DualEventsHash.Hex(),
		GasLimit: block.Header().GasLimit,
		LastCommitHash: block.Header().LastCommitHash.Hex(),
		NumDualEvents: block.Header().NumDualEvents,
		ReceiptHash: block.Header().ReceiptHash.Hex(),
		Root: block.Header().Root.Hex(),
		ValidatorsHash: block.Header().ValidatorsHash.Hex(),
	}
	if block.Header().Time != nil {
		header.Time = block.Header().Time.Uint64()
	}
	return &Block{
		Header: header,
		Height: block.Height(),
		Hash: block.Hash().Hex(),
	}
}

func (block *Block) ToHeader() *types.Header {
	header := types.Header{
		Height:         block.Header.Height,
		LastBlockID:    (types.BlockID)(common.HexToHash(block.Header.LastBlockID)),
		Time:           big.NewInt(int64(block.Header.Time)),
		NumTxs:         block.Header.NumTxs,
		TxHash:         common.HexToHash(block.Header.TxHash),
		GasUsed:        block.Header.GasUsed,
		Bloom:          types.BytesToBloom(common.FromHex(block.Header.Bloom)),
		Coinbase:       common.HexToAddress(block.Header.Coinbase),
		ConsensusHash:  common.HexToHash(block.Header.ConsensusHash),
		DualEventsHash: common.HexToHash(block.Header.DualEventsHash),
		GasLimit:       block.Header.GasLimit,
		LastCommitHash: common.HexToHash(block.Header.LastCommitHash),
		NumDualEvents:  block.Header.NumDualEvents,
		ReceiptHash:    common.HexToHash(block.Header.ReceiptHash),
		Root:           common.HexToHash(block.Header.Root),
		ValidatorsHash: common.HexToHash(block.Header.ValidatorsHash),
	}
	return &header
}

func (block *Block) ToBlock(logger log.Logger) *types.Block {
	return types.NewBlockWithHeader(logger, block.ToHeader())
}

func NewTransaction(tx *types.Transaction, height uint64, blockHash string, index int) (*Transaction, error) {
	sender, err := types.Sender(tx)
	if err != nil {
		return nil, err
	}
	r, s, v := tx.RawSignatureValues()
	return &Transaction{
		Hash: tx.Hash().Hex(),
		Height: height,
		BlockHash: blockHash,
		GasLimit: tx.Gas(),
		Amount: tx.Value().String(),
		From: sender.Hex(),
		AccountNonce: tx.Nonce(),
		Payload: common.Bytes2Hex(tx.Data()),
		Price: tx.GasPrice().String(),
		Recipient: tx.To().Hex(),
		R: r.String(),
		S: s.String(),
		V: v.String(),
		Index: index,
	}, nil
}

func (tx *Transaction) ToTransaction() *types.Transaction {
	amount, ok := big.NewInt(1).SetString(tx.Amount, 10)
	if !ok {
		log.Error("cannot cast amount to big.Int", "txHash", tx.Hash)
		return nil
	}

	price, ok := big.NewInt(1).SetString(tx.Price, 10)
	if !ok {
		log.Error("cannot cast price to big.Int", "txHash", tx.Hash)
		return nil
	}

	s, ok := big.NewInt(1).SetString(tx.S, 10)
	if !ok {
		log.Error("cannot cast tx.S to big.Int", "txHash", tx.Hash)
		return nil
	}

	r, ok := big.NewInt(1).SetString(tx.R, 10)
	if !ok {
		log.Error("cannot cast tx.R to big.Int", "txHash", tx.Hash)
		return nil
	}

	v, ok := big.NewInt(1).SetString(tx.V, 10)
	if !ok {
		log.Error("cannot cast tx.V to big.Int", "txHash", tx.Hash)
		return nil
	}

	return types.NewTransactionWithSignedData(
		tx.AccountNonce,
		common.HexToAddress(tx.Recipient),
		amount,
		tx.GasLimit,
		price,
		common.Hex2Bytes(tx.Payload),
		s,
		r,
		v,
	)
}

func NewLog(log *types.Log) *Log {

	topics := make([]string, 0)
	if len(log.Topics) > 0 {
		for _, topic := range log.Topics {
			topics = append(topics, topic.Hex())
		}
	}

	return &Log{
		Data: common.Bytes2Hex(log.Data),
		BlockHash: log.BlockHash.Hex(),
		TxHash: log.TxHash.Hex(),
		Address: log.Address.Hex(),
		Index: log.Index,
		BlockHeight: log.BlockHeight,
		Removed: log.Removed,
		Topics: topics,
		TxIndex: log.TxIndex,
	}
}

func (log *Log) ToLog() *types.Log {
	topics := make([]common.Hash, 0)
	for _, topic := range log.Topics {
		topics = append(topics, common.HexToHash(topic))
	}
	return &types.Log{
		Data: common.Hex2Bytes(log.Data),
		BlockHash: common.HexToHash(log.BlockHash),
		TxHash: common.HexToHash(log.TxHash),
		Address: common.HexToAddress(log.Address),
		Index: log.Index,
		BlockHeight: log.BlockHeight,
		Removed: log.Removed,
		Topics: topics,
		TxIndex: log.TxIndex,
	}
}

func NewReceipt(receipt *types.Receipt, height uint64, blockHash string) *Receipt {
	logs := make([]*Log, 0)
	if len(receipt.Logs) > 0 {
		for _, l := range receipt.Logs {
			logs = append(logs, NewLog(l))
		}
	}
	return &Receipt{
		BlockHash: blockHash,
		Height: height,
		Logs: logs,
		TxHash: receipt.TxHash.Hex(),
		Bloom: common.Bytes2Hex(receipt.Bloom.Bytes()),
		GasUsed: receipt.GasUsed,
		ContractAddress: receipt.ContractAddress.Hex(),
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Status: receipt.Status,
		PostState: common.Bytes2Hex(receipt.PostState),
	}
}

func (receipt *Receipt) ToReceipt() *types.Receipt {
	logs := make([]*types.Log, 0)
	for _, l := range receipt.Logs {
		logs = append(logs, l.ToLog())
	}

	return &types.Receipt{
		Logs: logs,
		PostState: common.Hex2Bytes(receipt.PostState),
		Status: receipt.Status,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		ContractAddress: common.HexToAddress(receipt.ContractAddress),
		GasUsed: receipt.GasUsed,
		Bloom: types.BytesToBloom(common.Hex2Bytes(receipt.Bloom)),
		TxHash: common.HexToHash(receipt.TxHash),
	}
}

func NewVote(vote *types.Vote) *Vote {
	return &Vote{
		Height: vote.Height.Int64(),
		Type: vote.Type,
		Timestamp: vote.Timestamp,
		Round: vote.Round.Int64(),
		BlockID: vote.BlockID.String(),
		Signature: common.Bytes2Hex(vote.Signature),
		ValidatorAddress: vote.ValidatorAddress.Hex(),
		ValidatorIndex: vote.ValidatorIndex.Int64(),
	}
}

func (vote *Vote) ToVote() *types.Vote {
	return &types.Vote{
		Height: common.NewBigInt64(vote.Height),
		Type: vote.Type,
		Timestamp: vote.Timestamp,
		Round: common.NewBigInt64(vote.Round),
		BlockID: (types.BlockID)(common.HexToHash(vote.BlockID)),
		Signature: common.Hex2Bytes(vote.Signature),
		ValidatorAddress: common.HexToAddress(vote.ValidatorAddress),
		ValidatorIndex: common.NewBigInt64(vote.ValidatorIndex),
	}
}

func NewCommit(commit *types.Commit, height uint64) *Commit {
	votes := make([]*Vote, 0)
	for _, vote := range commit.Precommits {
		votes = append(votes, NewVote(vote))
	}
	return &Commit{
		Precommits: votes,
		BlockID: commit.BlockID.String(),
		Height: height,
	}
}

func (commit *Commit) ToCommit() *types.Commit {
	votes := make([]*types.Vote, 0)
	for _, vote := range commit.Precommits {
		votes = append(votes, vote.ToVote())
	}
	return &types.Commit{
		Precommits: votes,
		BlockID: (types.BlockID)(common.HexToHash(commit.BlockID)),
	}
}

func NewChainConfig(config *types.ChainConfig, hash common.Hash) *ChainConfig {
	return &ChainConfig{
		Hash: hash.Hex(),
		Epoch: config.Kaicon.Epoch,
		Period: config.Kaicon.Period,
	}
}

func (config *ChainConfig) ToChainConfig() *types.ChainConfig {
	kaiCon := types.KaiconConfig{
		Epoch: config.Epoch,
		Period: config.Period,
	}
	return &types.ChainConfig{Kaicon: &kaiCon}
}

