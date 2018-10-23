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
	"math/big"
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
)

// PeerRoundState contains the known state of a peer.
// NOTE: Read-only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height              *cmn.BigInt   `json:"height"`                // Height peer is at
	Round               *cmn.BigInt   `json:"round"`                 // Round peer is at, -1 if unknown.
	Step                RoundStepType `json:"step"`                  // Step peer is at
	StartTime           *big.Int      `json:"start_time"`            // Estimated start of round 0 at this height
	Proposal            bool          `json:"proposal"`              // True if peer has proposal for this round
	ProposalBlockHeader cmn.Hash      `json:"proposal_block_header"` //
	ProposalPOLRound    *cmn.BigInt   `json:"proposal_pol_round"`    // Proposal's POL round. -1 if none.
	ProposalPOL         *cmn.BitArray `json:"proposal_pol"`          // nil until ProposalPOLMessage received.
	Prevotes            *cmn.BitArray `json:"prevotes"`              // All votes peer has for this round
	Precommits          *cmn.BitArray `json:"precommits"`            // All precommits peer has for this round
	LastCommitRound     *cmn.BigInt   `json:"last_commit_round"`     // Round of commit for last height. -1 if none.
	LastCommit          *cmn.BitArray `json:"last_commit"`           // All commit precommits of commit for last height.
	CatchupCommitRound  *cmn.BigInt   `json:"catchup_commit_round"`  // Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommit       *cmn.BitArray `json:"catchup_commit"`        // All commit precommits peer has for this height & CatchupCommitRound
}

// StringLong returns a string representation of the PeerRoundState
func (prs PeerRoundState) StringLong() string {
	return fmt.Sprintf("PeerRoundState{%v/%v/%v @%v  Proposal:%v  POL:%v (round %v)  Prevotes:%v  Precommits:%v  LastCommit:%v (round %v)  Catchup:%v (round %v)}",
		prs.Height, prs.Round, prs.Step, time.Unix(prs.StartTime.Int64(), 0),
		prs.ProposalBlockHeader,
		prs.ProposalPOL, prs.ProposalPOLRound,
		prs.Prevotes,
		prs.Precommits,
		prs.LastCommit, prs.LastCommitRound,
		prs.CatchupCommit, prs.CatchupCommitRound)
}

// String returns a short string representing PeerRoundState
func (prs PeerRoundState) String() string {
	return fmt.Sprintf("PeerRoundState{%v/%v/%v @%v  Proposal:%v  POL:%v (round %v)  Prevotes:%v  Precommits:%v  LastCommit:%v (round %v)  Catchup:%v (round %v)}",
		prs.Height, prs.Round, prs.Step, time.Unix(prs.StartTime.Int64(), 0),
		prs.ProposalBlockHeader.Fingerprint(),
		prs.ProposalPOL,
		prs.ProposalPOLRound,
		prs.Prevotes,
		prs.Precommits,
		prs.LastCommit, prs.LastCommitRound,
		prs.CatchupCommit, prs.CatchupCommitRound)
}
