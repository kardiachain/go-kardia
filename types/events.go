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
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
)

// Reserved event types
const (
	EventBond              = "Bond"
	EventCompleteProposal  = "CompleteProposal"
	EventDupeout           = "Dupeout"
	EventFork              = "Fork"
	EventLock              = "Lock"
	EventNewBlock          = "NewBlock"
	EventNewBlockHeader    = "NewBlockHeader"
	EventNewRound          = "NewRound"
	EventNewRoundStep      = "NewRoundStep"
	EventPolka             = "Polka"
	EventRebond            = "Rebond"
	EventRelock            = "Relock"
	EventTimeoutPropose    = "TimeoutPropose"
	EventTimeoutWait       = "TimeoutWait"
	EventTx                = "Tx"
	EventUnbond            = "Unbond"
	EventUnlock            = "Unlock"
	EventVote              = "Vote"
	EventProposalHeartbeat = "ProposalHeartbeat"
	EventValidBlock        = "EventValidBlock"
)

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height *cmn.BigInt `json:"height"`
	Round  *cmn.BigInt `json:"round"`
	Step   string      `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-"`
}

// implements events.EventData
type KaiEventData interface {
	AssertIsKaiEventData()
	// empty interface
}

func (_ EventDataRoundState) AssertIsKaiEventData() {}
func (_ EventDataVote) AssertIsKaiEventData()       {}

// ------- EventDataNewBlock ---------
type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Header *Header `json:"header"`
}

type EventDataVote struct {
	Vote *Vote
}

// BlockEventPublisher publishes all block related events
type BlockEventPublisher interface {
	PublishEventNewBlock(block EventDataNewBlock) error
	PublishEventNewBlockHeader(header EventDataNewBlockHeader) error
	//namdoh@ PublishEventTx(EventDataTx) error
}

type EventDataCompleteProposal struct {
	Height int64  `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	BlockID BlockID `json:"block_id"`
}

func (_ EventDataCompleteProposal) AssertIsKaiEventData() {}
