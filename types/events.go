package types

import (
	cmn "github.com/kardiachain/go-kardia/lib/common"
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

// ------- EventDataNewBlock ---------
type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Header *Header `json:"header"`
}

// BlockEventPublisher publishes all block related events
type BlockEventPublisher interface {
	PublishEventNewBlock(block EventDataNewBlock) error
	PublishEventNewBlockHeader(header EventDataNewBlockHeader) error
	//namdoh@ PublishEventTx(EventDataTx) error
}
