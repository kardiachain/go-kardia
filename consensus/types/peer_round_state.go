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
	"fmt"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

// PeerRoundState contains the known state of a peer.
// NOTE: Read-only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                   uint64              `json:"height"`                      // Height peer is at
	Round                    uint32              `json:"round"`                       // Round peer is at, -1 if unknown.
	Step                     RoundStepType       `json:"step"`                        // Step peer is at
	StartTime                uint64              `json:"start_time"`                  // Estimated start of round 0 at this height
	Proposal                 bool                `json:"proposal"`                    // True if peer has proposal for this round
	ProposalBlockPartsHeader types.PartSetHeader `json:"proposal_block_parts_header"` //
	ProposalBlockParts       *cmn.BitArray       `json:"proposal_block_parts"`        //
	ProposalPOLRound         uint32              `json:"proposal_pol_round"`          // Proposal's POL round. -1 if none.
	ProposalPOL              *cmn.BitArray       `json:"proposal_pol"`                // nil until ProposalPOLMessage received.
	Prevotes                 *cmn.BitArray       `json:"prevotes"`                    // All votes peer has for this round
	Precommits               *cmn.BitArray       `json:"precommits"`                  // All precommits peer has for this round
	LastCommitRound          uint32              `json:"last_commit_round"`           // Round of commit for last height. -1 if none.
	LastCommit               *cmn.BitArray       `json:"last_commit"`                 // All commit precommits of commit for last height.
	CatchupCommitRound       uint32              `json:"catchup_commit_round"`        // Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommit            *cmn.BitArray       `json:"catchup_commit"`              // All commit precommits peer has for this height & CatchupCommitRound
}

// StringIndented returns an indented string representation of the PeerRoundState
func (prs PeerRoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`PeerRoundState{
						%s  %v/%v/%v @%v
						%s  Proposal %v -> %v
						%s  POL      %v (round %v)
						%s  Prevotes   %v
						%s  Precommits %v
						%s  LastCommit %v (round %v)
						%s  Catchup    %v (round %v)
						%s}`,
		indent, prs.Height, prs.Round, prs.Step, prs.StartTime,
		indent, prs.ProposalBlockPartsHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent, prs.LastCommit, prs.LastCommitRound,
		indent, prs.CatchupCommit, prs.CatchupCommitRound,
		indent)
}
