/*
 *  Copyright 2021 KardiaChain
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

package blockchain

import (
	"fmt"

	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/types"
)

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func ValidateState(block *types.Block, blockInfo *types.BlockInfo, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	if blockInfo.GasUsed != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", blockInfo.GasUsed, usedGas)
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != blockInfo.Bloom {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", blockInfo.Bloom, rbloom)
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(false); block.Header().AppHash != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", block.Header().AppHash, root)
	}
	return nil
}
