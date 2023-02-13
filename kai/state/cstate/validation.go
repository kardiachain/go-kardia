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

package cstate

import (
	"fmt"

	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
)

func validateBlock(evidencePool EvidencePool, store Store, state LatestBlockState, block *types.Block) error {
	// Validate internal consistency
	if err := block.ValidateBasic(trie.NewStackTrie(nil)); err != nil {
		return err
	}

	// Validate basic info
	if block.Height() != state.LastBlockHeight+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v", state.LastBlockHeight+1, block.Height())
	}
	if state.LastBlockHeight == 0 && block.Height() != state.InitialHeight {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v for initial block, got %v",
			block.Height(), state.InitialHeight)
	}
	if state.LastBlockHeight > 0 && block.Height() != state.LastBlockHeight+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v",
			state.LastBlockHeight+1,
			block.Height(),
		)
	}

	// Validate prev block info
	if !block.Header().LastBlockID.Equal(state.LastBlockID) {
		return fmt.Errorf("wrong Block.Header.LastBlockID. Expected %v, got %v", state.LastBlockID, block.Header().LastBlockID)
	}
	// Validate app info
	if !block.AppHash().Equal(state.AppHash) {
		return fmt.Errorf("wrong Block.Header.AppHash. Expected %X, got %X",
			state.AppHash,
			block.AppHash(),
		)
	}

	if !block.Header().ValidatorsHash.Equal(state.Validators.Hash()) {
		return fmt.Errorf("wrong Block.Header.ValidatorsHash. Expected %X, got %X",
			state.Validators.Hash(),
			block.Header().ValidatorsHash,
		)
	}

	if !block.Header().NextValidatorsHash.Equal(state.NextValidators.Hash()) {
		return fmt.Errorf("wrong Block.Header.NextValidatorHash. Expected %X, got %X",
			state.NextValidators.Hash(),
			block.Header().NextValidatorsHash,
		)
	}

	// Validate block LastCommit
	if block.Height() == state.InitialHeight {
		if len(block.LastCommit().Signatures) != 0 {
			return ErrLastCommitSig
		}
	} else {
		// LastCommit.Signatures length is checked in VerifyCommit.
		if err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, block.Height()-1, block.LastCommit()); err != nil {
			return err
		}
	}

	// Validate block Time
	switch {
	case block.Height() > state.InitialHeight:
		if !block.Time().After(state.LastBlockTime) {
			return fmt.Errorf("block time %v not greater than last block time %v",
				block.Time(),
				state.LastBlockTime,
			)
		}
		medianTime := MedianTime(block.LastCommit(), state.LastValidators)
		if !block.Time().Equal(medianTime) {
			return fmt.Errorf("invalid block time. Expected %v, got %v",
				medianTime,
				block.Time(),
			)
		}

	case block.Height() == state.InitialHeight:
		genesisTime := state.LastBlockTime
		if !block.Time().Equal(genesisTime) {
			return fmt.Errorf("block time %v is not equal to genesis time %v",
				block.Time(),
				genesisTime,
			)
		}
	default:
		return fmt.Errorf("block height %v lower than initial height %v",
			block.Height(), state.InitialHeight)
	}

	// Limit the amount of evidence
	maxNumEvidence, _ := types.MaxEvidencePerBlock(int64(state.ConsensusParams.Block.MaxBytes))
	numEvidence := int64(len(block.Evidence().Evidence))
	if numEvidence > maxNumEvidence {
		return types.NewErrEvidenceOverflow(maxNumEvidence, numEvidence)

	}

	// Validate proposer is a known validator
	if !state.Validators.HasAddress(block.ProposerAddress()) {
		return fmt.Errorf("block proposer is not a validator %X", block.ValidatorHash())
	}

	// Validate all evidence.
	return evidencePool.CheckEvidence(block.Evidence().Evidence)
}
