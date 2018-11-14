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
	
	"github.com/kardiachain/go-kardia/kai/chaindb"
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

// PublicDualAPI provides APIs to access Dual full node-related
// information.
type PublicDualAPI struct {
	dualService *DualService
}

// NewPublicDualAPI creates a new Dual protocol API for full nodes.
func NewPublicDualAPI(dualService *DualService) *PublicDualAPI {
	return &PublicDualAPI{dualService}
}

// NewBlockJSON creates a new Block JSON data from Block
func NewBlockJSON(block *types.Block) *BlockJSON {

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
func (s *PublicDualAPI) BlockNumber() uint64 {
	return s.dualService.blockchain.CurrentBlock().Height()
}

// GetBlockByHash returns block by block hash
func (s *PublicDualAPI) GetBlockByHash(blockHash string) *BlockJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.dualService.blockchain.GetBlockByHash(common.HexToHash(blockHash))
	return NewBlockJSON(block)
}

// GetBlockByNumber returns block by block number
func (s *PublicDualAPI) GetBlockByNumber(blockNumber uint64) *BlockJSON {
	block := s.dualService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil
	}
	return NewBlockJSON(block)
}

// Validator returns node's validator, nil if current node is not a validator
func (s *PublicDualAPI) Validator() map[string]interface{} {
	if val := s.dualService.csManager.Validator(); val != nil {
		return map[string]interface{}{
			"address":     val.Address.Hex(),
			"votingPower": val.VotingPower,
		}
	}
	return nil
}

// Validators returns a list of validator
func (s *PublicDualAPI) Validators() []map[string]interface{} {
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

type PublicDualEvent struct {
	BlockHash				string				`json:"blockHash"`
	BlockNumber				common.Uint64		`json:"blockNumber"`
	Nonce           		uint64      		`json:"nonce"`
	TriggeredEvent  		string				`json:"triggeredEvent"`
	PendingTxMetadata		string				`json:"pendingTxMetadata"`
	Hash					string				`json:"hash"`
	EventIndex				uint				`json:"eventIndex"`
}

// NewPublicDualEvent returns a dual event that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewPublicDualEvent(dualEvent *types.DualEvent, blockHash common.Hash, blockNumber uint64, eventIndex uint64) *PublicDualEvent {
	result := &PublicDualEvent{
		Nonce:				dualEvent.Nonce,    	
		TriggeredEvent:		dualEvent.TriggeredEvent.String(),
		PendingTxMetadata:	dualEvent.PendingTxMetadata.String(),
		Hash:				dualEvent.Hash().Hex(),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash.Hex()
		result.BlockNumber = common.Uint64(blockNumber)
		result.EventIndex = uint(eventIndex)
	}
	return result
}

type PublicDualEventAPI struct {
	dualService *DualService
}

func NewPublicDualEventAPI(dualService *DualService) *PublicDualEventAPI {
	return &PublicDualEventAPI{dualService}
}

// GetDualEvent gets dual event by event hash
func (s *PublicDualEventAPI) GetDualEvent(hash string) *PublicDualEvent {
	dualEventHash := common.HexToHash(hash)
	dualEvent, blockHash, blockNumber, eventIndex := chaindb.ReadDualEvent(s.dualService.groupDb, dualEventHash)
	return NewPublicDualEvent(dualEvent, blockHash, blockNumber, eventIndex)
}

// PendingDualEvents returns pending dual events
func (s *PublicDualEventAPI) PendingDualEvents() ([]*PublicDualEvent, error) {
	pending, err := s.dualService.EventPool().Pending()
	if err != nil {
		return nil, err
	}
	
	dualEvents := make([]*PublicDualEvent, len(pending))
	
	for _, dualEvent := range pending {
		jsonData := NewPublicDualEvent(dualEvent, common.Hash{}, 0, 0)
		dualEvents = append(dualEvents, jsonData)
	} 
	
	return dualEvents, nil
}

// GetContentDualEvents retrieves the data content of the dual's event pool, 
// returning all the pending as well as queued dual's events, sorted by nonce.
func (s *PublicDualEventAPI) GetContentDualEvents() ([]*PublicDualEvent, []*PublicDualEvent) {
	pending, queued := s.dualService.EventPool().Content()
	
	pendingEvents := make([]*PublicDualEvent, len(pending))
	queuedEvents := make([]*PublicDualEvent, len(queued))
	
	for _, dualEvent := range pending {
		jsonData := NewPublicDualEvent(dualEvent, common.Hash{}, 0, 0)
		pendingEvents = append(pendingEvents, jsonData)
	}
	
	for _, dualEvent := range queued {
		jsonData := NewPublicDualEvent(dualEvent, common.Hash{}, 0, 0)
		queuedEvents = append(queuedEvents, jsonData)
	}
	
	return pendingEvents, queuedEvents
}

// GetStatusDualEvents returns the status (unknown/pending/queued) of a batch 
// of dual's events identified by their hashes.
func (s *PublicDualEventAPI) GetStatusDualEvent(hash string) map[string]string {
	
}

