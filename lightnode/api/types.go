// Package api
package api

import (
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

// BlockHeaderJSON represents BlockHeader in JSON format
type BlockHeaderJSON struct {
	Hash              string    `json:"hash"`
	Height            uint64    `json:"height"`
	LastBlock         string    `json:"lastBlock"`
	CommitHash        string    `json:"commitHash"`
	Time              time.Time `json:"time"`
	NumTxs            uint64    `json:"numTxs"`
	GasUsed           uint64    `json:"gasUsed"`
	GasLimit          uint64    `json:"gasLimit"`
	Rewards           string    `json:"Rewards"`
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
}

type PublicTransaction struct {
	BlockHash        string       `json:"blockHash"`
	BlockNumber      uint64       `json:"blockNumber"`
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
	Status           uint         `json:"status"`
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
