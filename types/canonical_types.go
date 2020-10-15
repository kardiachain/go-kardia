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
	tmproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(bid tmproto.BlockID) *tmproto.CanonicalBlockID {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		panic(err)
	}
	var cbid *tmproto.CanonicalBlockID
	if rbid == nil || rbid.IsZero() {
		cbid = nil
	} else {
		cbid = &tmproto.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
}

// CanonicalizeVote transforms the given PartSetHeader to a CanonicalPartSetHeader.
func CanonicalizePartSetHeader(psh tmproto.PartSetHeader) tmproto.CanonicalPartSetHeader {
	return tmproto.CanonicalPartSetHeader(psh)
}

// ------- Helper functions to create canonical types --------------
func CreateCanonicalProposal(chainID string, proposal *tmproto.Proposal) tmproto.CanonicalProposal {
	return tmproto.CanonicalProposal{
		Type:      tmproto.ProposalType,
		Height:    proposal.Height, // encoded as sfixed64
		Round:     proposal.Round,  // encoded as sfixed64
		POLRound:  proposal.PolRound,
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

func CreateCanonicalVote(chainID string, vote *tmproto.Vote) tmproto.CanonicalVote {
	return tmproto.CanonicalVote{
		ChainID:   chainID,
		Type:      tmproto.PrevoteType,
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: vote.Timestamp,
	}
}
