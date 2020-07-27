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

package types

import (
	"testing"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
)

func TestProposalCreation(t *testing.T) {
	block := CreateNewBlock(1)
	proposal := NewProposal(cmn.NewBigInt64(1), cmn.NewBigInt64(2), block, cmn.NewBigInt64(3), CreateBlockIDRandom())

	if !proposal.Height.Equals(cmn.NewBigInt64(1)) ||
		!proposal.Round.Equals(cmn.NewBigInt64(2)) ||
		proposal.Block.Hash() != block.Hash() ||
		!proposal.POLRound.Equals(cmn.NewBigInt64(3)) {
		t.Error("Proposal Creation Error")
	}

}

func TestProposalSignBytes(t *testing.T) {
	block := CreateNewBlock(1)
	proposal := NewProposal(cmn.NewBigInt64(1), cmn.NewBigInt64(2), block, cmn.NewBigInt64(3), CreateBlockIDRandom())
	signedByte := proposal.SignBytes("KAI")
	if signedByte == nil {
		t.Error("Proposal's SignBytes returned nil")
	}
}
