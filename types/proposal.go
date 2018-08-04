package types

import (
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
)

// Proposal defines a block proposal for the consensus.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height     *cmn.BigInt `json:"height"`
	Round      *cmn.BigInt `json:"round"`
	Timestamp  time.Time   `json:"timestamp"`
	Block      *Block      `json:"block"`        // TODO(huny@): Should we use hash instead?
	POLRound   *cmn.BigInt `json:"pol_round"`    // -1 if null.
	POLBlockID BlockID     `json:"pol_block_id"` // zero if null.
	Signature  []byte      `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height *cmn.BigInt, round *cmn.BigInt, block *Block, polRound *cmn.BigInt, polBlockID BlockID) *Proposal {
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
