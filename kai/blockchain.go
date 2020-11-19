/*
 *  Copyright 2020 KardiaChain
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
// Package kai
package kai

import (
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
// Get/Set/Commit
type EvidencePool interface {
	PendingEvidence(int64) ([]types.Evidence, int64)
}

type BlockOperations interface {
	Height() uint64
	CreateProposalBlock(
		height uint64, lastState cstate.LastestBlockState,
		proposerAddr common.Address, commit *types.Commit) (block *types.Block, blockParts *types.PartSet)
	SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
	LoadBlock(height uint64) *types.Block
	LoadBlockCommit(height uint64) *types.Commit
	LoadSeenCommit(height uint64) *types.Commit
	LoadBlockPart(height uint64, index int) *types.Part
	LoadBlockMeta(height uint64) *types.BlockMeta
}
