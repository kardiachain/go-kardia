package types

import (
	"time"
)

// Proposal defines a block proposal for the consensus.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height     int64     `json:"height"`
	Round      int       `json:"round"`
	Timestamp  time.Time `json:"timestamp"`
	Block      *Block    `json:"block"`        // TODO(huny@): Should we use hash instead?
	POLRound   int       `json:"pol_round"`    // -1 if null.
	POLBlockID BlockID   `json:"pol_block_id"` // zero if null.
	Signature  []byte    `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height int64, round int, block *Block, polRound int, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:     height,
		Round:      round,
		Timestamp:  time.Now().UTC(),
		Block:      block,
		POLRound:   polRound,
		POLBlockID: polBlockID,
	}
}

// TODO(huny@): Implement signature
