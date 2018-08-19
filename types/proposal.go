package types

import (
	"fmt"
	"math/big"
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

// Proposal defines a block proposal for the consensus.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height     *cmn.BigInt `json:"height"`
	Round      *cmn.BigInt `json:"round"`
	Timestamp  *big.Int    `json:"timestamp"`    // TODO(thientn/namdoh): epoch seconds, change to milis.
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
		Timestamp:  big.NewInt(time.Now().Unix()),
		Block:      block,
		POLRound:   polRound,
		POLBlockID: polBlockID,
	}
}

// SignBytes returns the Proposal bytes for signing
func (p *Proposal) SignBytes(chainID string) []byte {
	bz, err := rlp.EncodeToBytes(CreateCanonicalProposal(chainID, p))
	if err != nil {
		panic(err)
	}
	return bz
}

// String returns a string representation of the Proposal.
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %X @ %s}",
		p.Height, p.Round, p.Block, p.POLRound,
		p.POLBlockID,
		cmn.Fingerprint(p.Signature), p.Timestamp)
}
