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
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	blockTable           = "Block"
	txTable              = "Transaction"
	dualEvtTable         = "DualEvent"
	receiptTable         = "Receipt"
	commitTable          = "Commit"
	headHeaderTable      = "HeadHeader"
	headBlockTable       = "HeadBlock"
	chainConfigTable     = "ChainConfig"
	trieTable            = "Trie"
	txLookupEntryTable   = "TxLookupEntry"
	watcherActionTable   = "watcherAction"
	dualActionTable      = "dualEvent"
	contractAddressTable = "ContractAddress"
	emptyAddress         = "0x0000000000000000000000000000000000000000"
)

type (
	PartSetHeader struct {
		Total int
		Hash  string
	}

	Header struct {
		Height        uint64 `json:"height"        bson:"height"`
		Time          uint64 `json:"time"          bson:"time"`
		NumTxs        uint64 `json:"numTxs"        bson:"numTxs"`
		NumDualEvents uint64 `json:"numDualEvents" bson:"numDualEvents"`

		GasLimit uint64 `json:"gasLimit"         bson:"gasLimit"`
		GasUsed  uint64 `json:"gasUsed"          bson:"gasUsed"`

		// prev block info
		LastBlockID string        `json:"lastBlockID"      bson:"lastBlockID"`
		PartsHeader PartSetHeader `json:"partsHeader" 		  bson:"partsHeader"`
		Coinbase    string        `json:"miner"            bson:"miner"` // address

		// hashes of block data
		LastCommitHash string `json:"lastCommitHash"      bson:"lastCommitHash"` // commit from validators from the last block
		TxHash         string `json:"txHash"              bson:"txHash"`         // transactions
		DualEventsHash string `json:"dualEventsHash"      bson:"dualEventsHash"` // dual's events
		AppHash        string `json:"app_hash"           bson:"app_hash"`        // state root
		ReceiptHash    string `json:"receiptsRoot"        bson:"receiptsRoot"`   // receipt root
		Bloom          string `json:"logsBloom"           bson:"logsBloom"`

		Validator      string `json:"validator"           bson:"validator"`
		ValidatorsHash string `json:"validators_hash"` // validators for the current block
		ConsensusHash  string `json:"consensus_hash"`  // consensus params for current block
	}
	Block struct {
		Header Header `json:"header"    bson:"header"`
		Hash   string `json:"hash"      bson:"hash"`
		Height uint64 `json:"height"    bson:"height"`
	}
	Receipt struct {
		BlockHash         string `json:"blockHash"         bson:"blockHash"`
		Height            uint64 `json:"height"            bson:"height"`
		PostState         string `json:"root"              bson:"root"`
		Status            uint64 `json:"status"            bson:"status"`
		CumulativeGasUsed uint64 `json:"cumulativeGasUsed" bson:"cumulativeGasUsed"`
		Bloom             string `json:"logsBloom"         bson:"logsBloom"`
		Logs              []*Log `json:"logs"              bson:"logs"`
		TxHash            string `json:"transactionHash"   bson:"transactionHash"`
		ContractAddress   string `json:"contractAddress"   bson:"contractAddress"`
		GasUsed           uint64 `json:"gasUsed"           bson:"gasUsed"`
	}
	Log struct {
		Address     string   `json:"address"          bson:"address"`
		Topics      []string `json:"topics"           bson:"topics"`
		Data        string   `json:"data"             bson:"data"`
		BlockHeight uint64   `json:"blockHeight"      bson:"blockHeight"`
		TxHash      string   `json:"transactionHash"  bson:"transactionHash"`
		TxIndex     uint     `json:"transactionIndex" bson:"transactionIndex"`
		BlockHash   string   `json:"blockHash"        bson:"blockHash"`
		Index       uint     `json:"logIndex"         bson:"logIndex"`
		Removed     bool     `json:"removed"          bson:"removed"`
	}
	Transaction struct {
		ID           primitive.ObjectID `json:"id"           bson:"_id,omitempty"`
		From         string             `json:"from"         bson:"from"`
		AccountNonce uint64             `json:"nonce"        bson:"nonce"`
		Price        string             `json:"gasPrice"     bson:"gasPrice"`
		GasLimit     uint64             `json:"gas"          bson:"gas"`
		Recipient    string             `json:"to"           bson:"to"` // nil means contract creation
		Amount       string             `json:"value"        bson:"value"`
		Payload      string             `json:"input"        bson:"input"`

		// Signature values
		V string `json:"v"            bson:"v"`
		R string `json:"r"            bson:"r"`
		S string `json:"s"            bson:"s"`

		// This is only used when marshaling to JSON.
		Hash      string `json:"hash"         bson:"hash"`
		BlockHash string `json:"blockHash"    bson:"blockHash"`
		Height    uint64 `json:"height"       bson:"height"`
		Index     int    `json:"index"        bson:"index"`
	}
	DualEvent struct {
		Nonce             uint64                       `json:"nonce"                bson:"nonce"`
		TriggeredEvent    *types.EventData             `json:"triggeredEvent"       bson:"triggeredEvent"`
		PendingTxMetadata *types.TxMetadata            `json:"pendingTxMetadata"    bson:"pendingTxMetadata"`
		KardiaSmcs        []*types.KardiaSmartcontract `json:"kardiaSmcs"           bson:"kardiaSmcs"`
		Hash              string                       `json:"hash"                 bson:"hash"`
		BlockHash         string                       `json:"blockHash"            bson:"blockHash"`
		Height            uint64                       `json:"height"               bson:"height"`
	}
	EventData struct {
		TxHash       string              `json:"txHash"           bson:"txHash"`
		TxSource     string              `json:"txSource"         bson:"txSource"`
		FromExternal bool                `json:"fromExternal"     bson:"fromExternal"`
		Data         *types.EventSummary `json:"data"             bson:"data"`
		Actions      *types.DualActions  `json:"actions"          bson:"actions"`
	}
	EventSummary struct {
		TxMethod string   `json:"txMethod"           bson:"txMethod"` // Smc's method
		TxValue  *big.Int `json:"txValue"            bson:"txValue"`  // Amount of the tx
		ExtData  []string `json:"extData"            bson:"extData"`  // Additional data along with this event
	}
	Commit struct {
		Height      uint64        `json:"height"           bson:"height"`
		BlockID     string        `json:"blockID"          bson:"blockID"`
		Precommits  []*Vote       `json:"precommits"       bson:"precommits"`
		PartsHeader PartSetHeader `json:"partsHeader" bson:"partsHeader"`
	}
	Vote struct {
		ValidatorAddress string              `json:"validatorAddress"           bson:"validatorAddress"`
		ValidatorIndex   int64               `json:"validatorIndex"             bson:"validatorIndex"`
		Height           int64               `json:"height"                     bson:"height"`
		Round            int64               `json:"round"                      bson:"round"`
		Timestamp        uint64              `json:"timestamp"                  bson:"timestamp"`
		Type             types.SignedMsgType `json:"type"                       bson:"type"`
		BlockID          string              `json:"blockID"                    bson:"blockID"`
		Signature        string              `json:"signature"                  bson:"signature"`
		PartsHeader      PartSetHeader       `json:"partsHeader" bson:"partsHeader"`
	}
	HeadHeaderHash struct {
		ID   int    `json:"ID"      bson:"ID"`
		Hash string `json:"hash"    bson:"hash"`
	}
	HeadBlockHash struct {
		ID   int    `json:"ID"      bson:"ID"`
		Hash string `json:"hash"    bson:"hash"`
	}
	ChainConfig struct {
		Hash        string `json:"hash"     bson:"hash"`
		Period      uint64 `json:"period"   bson:"period"`
		Epoch       uint64 `json:"epoch"    bson:"epoch"`
		BaseAccount `json:"baseAccount,omitempty"`
	}
	Caching struct {
		Key   string `json:"key"       bson:"key"`
		Value string `json:"value"     bson:"value"`
	}
	TxLookupEntry struct {
		TxHash     string `json:"txHash"     bson:"txHash"`
		BlockHash  string `json:"blockHash"  bson:"blockHash"`
		BlockIndex uint64 `json:"blockIndex" bson:"blockIndex"`
		Index      uint64 `json:"index"      bson:"index"`
	}
	Watcher struct {
		MasterContractAddress string   `json:"masterContractAddress" bson:"masterContractAddress"`
		MasterABI             string   `json:"masterABI"             bson:"masterABI"`
		ContractAddress       string   `json:"contractAddress"       bson:"contractAddress"`
		ABI                   string   `json:"ABI"                   bson:"ABI"`
		Method                string   `json:"method"                bson:"method"`
		DualActions           []string `json:"dualActions"           bson:"dualActions"`
		WatcherActions        []string `json:"watcherActions"        bson:"watcherActions"`
	}
	BaseAccount struct {
		Address    string `json:"address"`
		PrivateKey string `json:"PrivateKey"`
	}
)

func NewBlock(block *types.Block) *Block {
	header := Header{
		Height:         block.Header().Height,
		Time:           0,
		LastBlockID:    block.Header().LastBlockID.String(),
		NumTxs:         block.NumTxs(),
		TxHash:         block.Header().TxHash.Hex(),
		Coinbase:       block.Header().Coinbase.Hex(),
		ConsensusHash:  block.Header().ConsensusHash.Hex(),
		DualEventsHash: block.Header().DualEventsHash.Hex(),
		GasLimit:       block.Header().GasLimit,
		LastCommitHash: block.Header().LastCommitHash.Hex(),
		NumDualEvents:  block.Header().NumDualEvents,
		Validator:      block.Header().Validator.Hex(),
		ValidatorsHash: block.Header().ValidatorsHash.Hex(),
	}
	if block.Header().Time != nil {
		header.Time = block.Header().Time.Uint64()
	}
	return &Block{
		Header: header,
		Height: block.Height(),
		Hash:   block.Hash().Hex(),
	}
}

func (block *Block) ToHeader() *types.Header {
	header := types.Header{
		Height:         block.Header.Height,
		LastBlockID:    toBlockID(block.Header.LastBlockID, block.Header.PartsHeader),
		Time:           big.NewInt(int64(block.Header.Time)),
		NumTxs:         block.Header.NumTxs,
		TxHash:         common.HexToHash(block.Header.TxHash),
		Coinbase:       common.HexToAddress(block.Header.Coinbase),
		ConsensusHash:  common.HexToHash(block.Header.ConsensusHash),
		DualEventsHash: common.HexToHash(block.Header.DualEventsHash),
		GasLimit:       block.Header.GasLimit,
		LastCommitHash: common.HexToHash(block.Header.LastCommitHash),
		NumDualEvents:  block.Header.NumDualEvents,
		AppHash:        common.HexToHash(block.Header.AppHash),
		Validator:      common.HexToAddress(block.Header.Validator),
		ValidatorsHash: common.HexToHash(block.Header.ValidatorsHash),
	}
	return &header
}

func (block *Block) ToBlock() *types.Block {
	return types.NewBlockWithHeader(block.ToHeader())
}

func NewTransaction(tx *types.Transaction, height uint64, blockHash string, index int) (*Transaction, error) {
	sender, err := types.Sender(types.HomesteadSigner{}, tx)
	if err != nil {
		return nil, err
	}
	v, r, s := tx.RawSignatureValues()
	newTx := &Transaction{
		Hash:         tx.Hash().Hex(),
		Height:       height,
		BlockHash:    blockHash,
		GasLimit:     tx.Gas(),
		Amount:       tx.Value().String(),
		From:         sender.Hex(),
		AccountNonce: tx.Nonce(),
		Payload:      common.Bytes2Hex(tx.Data()),
		Price:        tx.GasPrice().String(),
		R:            r.String(),
		S:            s.String(),
		V:            v.String(),
		Index:        index,
	}

	if tx.To() == nil {
		newTx.Recipient = "0x"
	} else {
		newTx.Recipient = tx.To().Hex()
	}

	return newTx, nil
}

func (tx *Transaction) ToTransaction(signer types.Signer) *types.Transaction {
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

	var newTx *types.Transaction
	if tx.Recipient == "0x" {
		newTx = types.NewContractCreation(
			tx.AccountNonce,
			amount,
			tx.GasLimit,
			price,
			common.Hex2Bytes(tx.Payload),
		)
	} else {
		newTx = types.NewTransaction(
			tx.AccountNonce,
			common.HexToAddress(tx.Recipient),
			amount,
			tx.GasLimit,
			price,
			common.Hex2Bytes(tx.Payload),
		)
	}

	sig := make([]byte, 65)
	copy(sig[:32], r.Bytes())
	copy(sig[32:64], s.Bytes())
	sig[64] = byte(v.Uint64() - 27)

	signedTx, err := newTx.WithSignature(signer, sig)
	if err != nil {
		log.Error("error while signing tx based on stored r,s,v", "err", err)
		return nil
	}
	return signedTx
}

func NewLog(log *types.Log) *Log {

	topics := make([]string, 0)
	if len(log.Topics) > 0 {
		for _, topic := range log.Topics {
			topics = append(topics, topic.Hex())
		}
	}

	return &Log{
		Data:        common.Bytes2Hex(log.Data),
		BlockHash:   log.BlockHash.Hex(),
		TxHash:      log.TxHash.Hex(),
		Address:     log.Address.Hex(),
		Index:       log.Index,
		BlockHeight: log.BlockHeight,
		Removed:     log.Removed,
		Topics:      topics,
		TxIndex:     log.TxIndex,
	}
}

func (log *Log) ToLog() *types.Log {
	topics := make([]common.Hash, 0)
	for _, topic := range log.Topics {
		topics = append(topics, common.HexToHash(topic))
	}
	return &types.Log{
		Data:        common.Hex2Bytes(log.Data),
		BlockHash:   common.HexToHash(log.BlockHash),
		TxHash:      common.HexToHash(log.TxHash),
		Address:     common.HexToAddress(log.Address),
		Index:       log.Index,
		BlockHeight: log.BlockHeight,
		Removed:     log.Removed,
		Topics:      topics,
		TxIndex:     log.TxIndex,
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
		BlockHash:         blockHash,
		Height:            height,
		Logs:              logs,
		TxHash:            receipt.TxHash.Hex(),
		Bloom:             common.Bytes2Hex(receipt.Bloom.Bytes()),
		GasUsed:           receipt.GasUsed,
		ContractAddress:   receipt.ContractAddress.Hex(),
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Status:            receipt.Status,
		PostState:         common.Bytes2Hex(receipt.PostState),
	}
}

func (receipt *Receipt) ToReceipt() *types.Receipt {
	logs := make([]*types.Log, 0)
	for _, l := range receipt.Logs {
		logs = append(logs, l.ToLog())
	}

	return &types.Receipt{
		Logs:              logs,
		PostState:         common.Hex2Bytes(receipt.PostState),
		Status:            receipt.Status,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		ContractAddress:   common.HexToAddress(receipt.ContractAddress),
		GasUsed:           receipt.GasUsed,
		Bloom:             types.BytesToBloom(common.Hex2Bytes(receipt.Bloom)),
		TxHash:            common.HexToHash(receipt.TxHash),
	}
}

func NewVote(vote *types.Vote) *Vote {
	return &Vote{
		Height:           vote.Height.Int64(),
		Type:             vote.Type,
		Timestamp:        vote.Timestamp.Uint64(),
		Round:            vote.Round.Int64(),
		BlockID:          vote.BlockID.StringLong(),
		Signature:        common.Bytes2Hex(vote.Signature),
		ValidatorAddress: vote.ValidatorAddress.Hex(),
		ValidatorIndex:   vote.ValidatorIndex.Int64(),
	}
}

func (vote *Vote) ToVote() *types.Vote {
	return &types.Vote{
		Height:           common.NewBigInt64(vote.Height),
		Type:             vote.Type,
		Timestamp:        big.NewInt(int64(vote.Timestamp)),
		Round:            common.NewBigInt64(vote.Round),
		BlockID:          toBlockID(vote.BlockID, vote.PartsHeader),
		Signature:        common.Hex2Bytes(vote.Signature),
		ValidatorAddress: common.HexToAddress(vote.ValidatorAddress),
		ValidatorIndex:   common.NewBigInt64(vote.ValidatorIndex),
	}
}

func NewCommit(commit *types.Commit, height uint64) *Commit {
	votes := make([]*Vote, 0)
	for _, vote := range commit.Precommits {
		if vote != nil {
			votes = append(votes, NewVote(vote))
		} else {
			votes = append(votes, nil)
		}
	}

	return &Commit{
		Precommits: votes,
		BlockID:    commit.BlockID.StringLong(),
		Height:     height,
	}
}

func (commit *Commit) ToCommit() *types.Commit {
	votes := make([]*types.Vote, 0)
	for _, vote := range commit.Precommits {
		if vote != nil {
			votes = append(votes, vote.ToVote())
		} else {
			votes = append(votes, nil)
		}
	}
	return &types.Commit{
		Precommits: votes,
		BlockID:    toBlockID(commit.BlockID, commit.PartsHeader),
	}
}

func NewChainConfig(config *types.ChainConfig, hash common.Hash) *ChainConfig {
	return &ChainConfig{
		Hash:   hash.Hex(),
		Epoch:  config.Kaicon.Epoch,
		Period: config.Kaicon.Period,
		BaseAccount: BaseAccount{
			Address:    config.BaseAccount.Address.Hex(),
			PrivateKey: common.Bytes2Hex(config.PrivateKey.D.Bytes()),
		},
	}
}

func (config *ChainConfig) ToChainConfig() *types.ChainConfig {
	pk, err := crypto.HexToECDSA(config.BaseAccount.PrivateKey)
	if err != nil {
		return nil
	}
	kaiCon := types.KaiconConfig{
		Epoch:  config.Epoch,
		Period: config.Period,
	}
	return &types.ChainConfig{Kaicon: &kaiCon, BaseAccount: &types.BaseAccount{PrivateKey: *pk, Address: common.HexToAddress(config.BaseAccount.Address)}}
}

func toBlockID(blockID string, partSetHeader PartSetHeader) types.BlockID {
	return types.BlockID{
		Hash: common.BytesToHash([]byte(blockID)),
		PartsHeader: types.PartSetHeader{
			Total: *common.NewBigInt32(partSetHeader.Total),
			Hash:  common.BytesToHash([]byte(partSetHeader.Hash)),
		},
	}
}
