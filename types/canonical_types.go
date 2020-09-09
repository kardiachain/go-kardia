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

type CanonicalProposal struct {
	ChainID    string  `json:"@chain_id"`
	Type       string  `json:"@type"`
	Height     uint64  `json:"height"`
	POLBlockID BlockID `json:"pol_block_id"`
	POLRound   uint32  `json:"pol_round"`
	Round      uint32  `json:"round"`
	Timestamp  uint64  `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
}

type CanonicalVote struct {
	ChainID   string  `json:"@chain_id"`
	Type      string  `json:"@type"`
	BlockID   BlockID `json:"block_id"`
	Height    uint64  `json:"height"`
	Round     uint32  `json:"round"`
	Timestamp uint64  `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	VoteType  byte    `json:"type"`
}

// ------- Helper functions to create canonical types --------------
func CreateCanonicalProposal(chainID string, proposal *Proposal) CanonicalProposal {
	return CanonicalProposal{
		ChainID:    chainID,
		Type:       "proposal",
		Height:     proposal.Height,
		Timestamp:  proposal.Timestamp,
		POLBlockID: proposal.POLBlockID,
		POLRound:   proposal.POLRound,
		Round:      proposal.Round,
	}
}

func CreateCanonicalVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		ChainID:   chainID,
		Type:      "vote",
		BlockID:   vote.BlockID,
		Height:    uint64(vote.Height),
		Round:     vote.Round,
		Timestamp: vote.Timestamp,
		VoteType:  vote.Type,
	}
}
