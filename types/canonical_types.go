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
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(bid kproto.BlockID) *kproto.CanonicalBlockID {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		panic(err)
	}
	var cbid *kproto.CanonicalBlockID
	if rbid == nil || rbid.IsZero() {
		cbid = nil
	} else {
		cbid = &kproto.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
}

// CanonicalizeVote transforms the given PartSetHeader to a CanonicalPartSetHeader.
func CanonicalizePartSetHeader(psh kproto.PartSetHeader) kproto.CanonicalPartSetHeader {
	return kproto.CanonicalPartSetHeader(psh)
}

// ------- Helper functions to create canonical types --------------
func CreateCanonicalProposal(chainID string, proposal *kproto.Proposal) kproto.CanonicalProposal {
	return kproto.CanonicalProposal{
		Type:      kproto.ProposalType,
		Height:    proposal.Height, // encoded as sfixed64
		Round:     proposal.Round,  // encoded as sfixed64
		POLRound:  proposal.PolRound,
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

func CreateCanonicalVote(chainID string, vote *kproto.Vote) kproto.CanonicalVote {
	return kproto.CanonicalVote{
		ChainID:   chainID,
		Type:      kproto.PrevoteType,
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: vote.Timestamp,
	}
}
