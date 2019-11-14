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
	"fmt"
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// DualBlockJSON represents Block in JSON format
type DualBlockJSON struct {
	Hash           string             `json:"hash"`
	Height         uint64             `json:"height"`
	LastBlock      string             `json:"lastBlock"`
	CommitHash     string             `json:"commitHash"`
	Time           *big.Int           `json:"time"`
	NumDualEvents  uint64             `json:"num_events"`
	DualEvents     []*PublicDualEvent `json:"dual_events"`
	DualEventsHash string             `json:"dual_events_hash"`
	GasLimit       uint64             `json:"gasLimit"`
	GasUsed        uint64             `json:"gasUsed"`
	Validator      string             `json:"validator"`
	Root           string             `json:"stateRoot"`    // state root
	ReceiptHash    string             `json:"receiptsRoot"` // receipt root
	Bloom          int64              `json:"logsBloom"`
	ValidatorsHash string             `json:"validators_hash"` // validators for the current block
	ConsensusHash  string             `json:"consensus_hash"`
}

// PublicDualAPI provides APIs to access Dual full node-related information.
type PublicDualAPI struct {
	dualService *DualService
}

// NewPublicDualAPI creates a new Dual protocol API for full nodes.
func NewPublicDualAPI(dualService *DualService) *PublicDualAPI {
	return &PublicDualAPI{dualService}
}

// NewDualBlockJSON creates a new Block JSON data from Block.
func NewDualBlockJSON(block *types.Block) *DualBlockJSON {
	dualEvents := make([]*PublicDualEvent, 0)
	for index, dualEvent := range block.DualEvents() {
		json := NewPublicDualEvent(dualEvent, block.Hash(), block.Height(), uint64(index))
		dualEvents = append(dualEvents, json)
	}

	return &DualBlockJSON{
		Hash:           block.Hash().Hex(),
		Height:         block.Height(),
		LastBlock:      block.Header().LastBlockID.String(),
		CommitHash:     block.LastCommitHash().Hex(),
		Time:           block.Header().Time,
		NumDualEvents:  block.Header().NumDualEvents,
		DualEvents:     dualEvents,
		DualEventsHash: block.Header().DualEventsHash.Hex(),
		GasLimit:       block.Header().GasLimit,
		GasUsed:        block.Header().GasUsed,
		Validator:      block.Header().Validator.Hex(),
		Root:           block.Header().Root.Hex(),
		ReceiptHash:    block.Header().ReceiptHash.Hex(),
		Bloom:          block.Header().Bloom.Big().Int64(),
		ValidatorsHash: block.Header().ValidatorsHash.Hex(),
		ConsensusHash:  block.Header().ConsensusHash.Hex(),
	}
}

// BlockNumber returns current block number.
func (s *PublicDualAPI) BlockNumber() uint64 {
	return s.dualService.blockchain.CurrentBlock().Height()
}

// GetBlockByHash returns block by block hash.
func (s *PublicDualAPI) GetBlockByHash(blockHash string) *DualBlockJSON {
	if blockHash[0:2] == "0x" {
		blockHash = blockHash[2:]
	}
	block := s.dualService.blockchain.GetBlockByHash(common.HexToHash(blockHash))

	return NewDualBlockJSON(block)
}

// GetBlockByNumber returns block by block number.
func (s *PublicDualAPI) GetBlockByNumber(blockNumber uint64) *DualBlockJSON {
	block := s.dualService.blockchain.GetBlockByHeight(blockNumber)
	if block == nil {
		return nil
	}

	return NewDualBlockJSON(block)
}

// Validator returns node's validator, nil if current node is not a validator.
func (s *PublicDualAPI) Validator() map[string]interface{} {
	if val := s.dualService.csManager.Validator(); val != nil {
		return map[string]interface{}{
			"address":     val.Address.Hex(),
			"votingPower": val.VotingPower,
		}
	}

	return nil
}

// Validators returns a list of validator.
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

// PublicDualEvent represents dual event in JSON format
type PublicDualEvent struct {
	BlockHash         string        `json:"blockHash"`
	BlockNumber       common.Uint64 `json:"blockNumber"`
	Nonce             uint64        `json:"nonce"`
	TriggeredEvent    string        `json:"triggeredEvent"`
	PendingTxMetadata string        `json:"pendingTxMetadata"`
	Hash              string        `json:"hash"`
	EventIndex        uint          `json:"eventIndex"`
}

// NewPublicDualEvent returns a dual event that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewPublicDualEvent(dualEvent *types.DualEvent, blockHash common.Hash, blockNumber uint64, eventIndex uint64) *PublicDualEvent {
	result := &PublicDualEvent{
		TriggeredEvent:    dualEvent.TriggeredEvent.String(),
		PendingTxMetadata: dualEvent.PendingTxMetadata.String(),
		Hash:              dualEvent.Hash().Hex(),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash.Hex()
		result.BlockNumber = common.Uint64(blockNumber)
		result.EventIndex = uint(eventIndex)
	}

	return result
}

// TODO(#215): Since dual event isn't saved to StateDB. This function doesn't work.
// TypeDualEvent returns type of dual event by event hash
func (s *PublicDualAPI) TypeDualEvent(hash string) map[string]interface{} {
	if hash[0:2] == "0x" {
		hash = hash[2:]
	}
	dualEventHash := common.HexToHash(hash)
	if dualEvent, _, _, _ :=
		s.dualService.groupDb.ReadDualEvent(dualEventHash); dualEvent != nil {
		return map[string]interface{}{
			"TxSource": dualEvent.TriggeredEvent.TxSource,
			"Target":   dualEvent.PendingTxMetadata.Target,
		}
	}

	return nil
}

// TODO(#215): Since dual event isn't saved to StateDB. This function doesn't work.
// GetDualEvent gets dual event by event hash.
func (s *PublicDualAPI) GetDualEvent(hash string) *PublicDualEvent {
	if hash[0:2] == "0x" {
		hash = hash[2:]
	}
	dualEventHash := common.HexToHash(hash)
	if dualEvent, blockHash, blockNumber, eventIndex :=
		s.dualService.groupDb.ReadDualEvent(dualEventHash); dualEvent != nil {
		return NewPublicDualEvent(dualEvent, blockHash, blockNumber, eventIndex)
	}

	return nil
}

// PendingDualEvents returns information of pending dual events.
func (s *PublicDualAPI) PendingDualEvents() ([]*PublicDualEvent, error) {
	pending := s.dualService.EventPool().GetPendingData()
	if pending.Len() == 0 {
		return nil, fmt.Errorf("event pool is empty")
	}
	dualEvents := make([]*PublicDualEvent, pending.Len())
	for _, dualEvent := range *pending {
		jsonData := NewPublicDualEvent(dualEvent, common.Hash{}, 0, 0)
		dualEvents = append(dualEvents, jsonData)
	}
	return dualEvents, nil
}
