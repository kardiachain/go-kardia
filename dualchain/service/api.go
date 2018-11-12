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

package service

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

const (
	defaultGasPrice             = 50 * params.Shannon
	defaultTimeOutForStaticCall = 5
)

// BlockJSON represents Block in JSON format
type BlockJSON struct {
	Hash           string               `json:"hash"`
	Height         uint64               `json:"height"`
	LastBlock      string               `json:"lastBlock"`
	CommitHash     string               `json:"commitHash"`
	Time           *big.Int             `json:"time"`
	NumTxs         uint64               `json:"num_txs"`
	GasLimit       uint64               `json:"gasLimit"`
	GasUsed        uint64               `json:"gasUsed"`
	Validator      string               `json:"validator"`
	TxHash         string               `json:"data_hash"`    // transactions
	Root           string               `json:"stateRoot"`    // state root
	ReceiptHash    string               `json:"receiptsRoot"` // receipt root
	Bloom          int64                `json:"logsBloom"`
	ValidatorsHash string               `json:"validators_hash"` // validators for the current block
	ConsensusHash  string               `json:"consensus_hash"`
}

// PublicKaiAPI provides APIs to access Kai full node-related
// information.
type PublicKaiAPI struct {
	dualService *DualService
}

// NewPublicKaiAPI creates a new Kai protocol API for full nodes.
func NewPublicKaiAPI(dualService *DualService) *PublicKaiAPI {
	return &PublicKaiAPI{dualService}
}

// NewBlockJSON creates a new Block JSON data from Block
func NewBlockJSON(block types.Block) *BlockJSON {

	return &BlockJSON{
		Hash:           block.Hash().Hex(),
		Height:         block.Height(),
		LastBlock:      block.Header().LastBlockID.String(),
		CommitHash:     block.LastCommitHash().Hex(),
		Time:           block.Header().Time,
		NumTxs:         block.Header().NumTxs,
		GasLimit:       block.Header().GasLimit,
		GasUsed:        block.Header().GasUsed,
		Validator:      block.Header().Coinbase.Hex(),
		TxHash:         block.Header().TxHash.Hex(),
		Root:           block.Header().Root.Hex(),
		ReceiptHash:    block.Header().ReceiptHash.Hex(),
		Bloom:          block.Header().Bloom.Big().Int64(),
		ValidatorsHash: block.Header().ValidatorsHash.Hex(),
		ConsensusHash:  block.Header().ConsensusHash.Hex(),
	}
}

// BlockNumber returns current block number
func (s *PublicKaiAPI) BlockNumber() uint64 {
	return s.dualService.blockchain.CurrentBlock().Height()
}

// GetBlockByHash returns block by block hash
func (s *PublicKaiAPI) GetBlockByHash(blockHash string) *BlockJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.dualService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	return NewBlockJSON(*block)
}

// GetBlockByNumber returns block by block number
func (s *PublicKaiAPI) GetBlockByNumber(blockNumber uint64) *BlockJSON {
	block := s.dualService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil
	}
	return NewBlockJSON(*block)
}

// Validator returns node's validator, nil if current node is not a validator
func (s *PublicKaiAPI) Validator() map[string]interface{} {
	if val := s.dualService.csManager.Validator(); val != nil {
		return map[string]interface{}{
			"address":     val.Address.Hex(),
			"votingPower": val.VotingPower,
		}
	}
	return nil
}

// Validators returns a list of validator
func (s *PublicKaiAPI) Validators() []map[string]interface{} {
	if vals := s.dualService.csManager.Validators(); vals != nil && len(vals) > 0 {
		results := make([]map[string]interface{}, len(vals))
		for i, val := range vals {
			results[i] = map[string]interface{}{
				"address":     val.Address.Hex(),
				"votingPower": val.VotingPower,
			}
		}
		return results
	}
	return nil
}


