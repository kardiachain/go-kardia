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

package consensus

import (
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardiamain/types"
)

func validateBlock(state LastestBlockState, block *types.Block) error {
	// validate internal consistency
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	// validate basic info
	if int64(block.Header().Height) != state.LastBlockHeight.Int64()+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v", state.LastBlockHeight.Int64()+1, block.Height())
	}
	/*	TODO: Determine bounds for Time
		See blockchain/manager "stopSyncingDurationMinutes"
		if !block.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/

	// validate prev block info
	if !block.Header().LastBlockID.Equal(state.LastBlockID) {
		return fmt.Errorf("Wrong Block.Header.LastBlockID.  Expected %v, got %v", state.LastBlockID, block.Header().LastBlockID)
	}
	// TODO(namdoh): Re-enable validating txs
	//newTxs := int64(len(block.Data.Txs))
	//if block.TotalTxs != state.LastBlockTotalTx+newTxs {
	//	return fmt.Errorf("Wrong Block.Header.TotalTxs. Expected %v, got %v", state.LastBlockTotalTx+newTxs, block.TotalTxs)
	//}

	// TODO(namdoh): Re-enable validating other info
	// validate app info
	//if !bytes.Equal(block.AppHash, state.AppHash) {
	//	return fmt.Errorf("Wrong Block.Header.AppHash.  Expected %X, got %v", state.AppHash, block.AppHash)
	//}
	//if !bytes.Equal(block.ConsensusHash, state.ConsensusParams.Hash()) {
	//	return fmt.Errorf("Wrong Block.Header.ConsensusHash.  Expected %X, got %v", state.ConsensusParams.Hash(), block.ConsensusHash)
	//}
	//if !bytes.Equal(block.LastResultsHash, state.LastResultsHash) {
	//	return fmt.Errorf("Wrong Block.Header.LastResultsHash.  Expected %X, got %v", state.LastResultsHash, block.LastResultsHash)
	//}
	//if !bytes.Equal(block.ValidatorsHash, state.Validators.Hash()) {
	//	return fmt.Errorf("Wrong Block.Header.ValidatorsHash.  Expected %X, got %v", state.Validators.Hash(), block.ValidatorsHash)
	//}

	// Validate block LastCommit.
	if block.Header().Height == 1 {
		if len(block.LastCommit().Precommits) != 0 {
			return errors.New("block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		if len(block.LastCommit().Precommits) != state.LastValidators.Size() {
			return fmt.Errorf("invalid block commit size. Expected %v, got %v",
				state.LastValidators.Size(), len(block.LastCommit().Precommits))
		}
		err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, int64(block.Height()-1), block.LastCommit())
		if err != nil {
			return err
		}
	}

	if !block.AppHash().Equal(state.AppHash) {
		return fmt.Errorf("Wrong Block.header.AppHash.  Expected %s, got %s",
			state.AppHash.String(),
			block.AppHash().String(),
		)
	}

	// TODO: Each check requires loading an old validator set.
	// We should cap the amount of evidence per block
	// to prevent potential proposer DoS.
	// TODO(namdoh): Validates evidences.
	//for _, ev := range block.Evidence.Evidence {
	//	if err := VerifyEvidence(stateDB, state, ev); err != nil {
	//		return types.NewEvidenceInvalidErr(ev, err)
	//	}
	//}

	return nil
}
