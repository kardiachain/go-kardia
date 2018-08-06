package types

import (
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
)

type CanonicalProposal struct {
	ChainID    string      `json:"@chain_id"`
	Type       string      `json:"@type"`
	Block      *Block      `json:"block"`
	Height     *cmn.BigInt `json:"height"`
	POLBlockID BlockID     `json:"pol_block_id"`
	POLRound   *cmn.BigInt `json:"pol_round"`
	Round      *cmn.BigInt `json:"round"`
	Timestamp  time.Time   `json:"timestamp"`
}

// ------- Helper functions to create canonical types --------------
func CreateCanonicalProposal(chainID string, proposal *Proposal) CanonicalProposal {
	return CanonicalProposal{
		ChainID:    chainID,
		Type:       "proposal",
		Block:      proposal.Block,
		Height:     proposal.Height,
		Timestamp:  proposal.Timestamp,
		POLBlockID: proposal.POLBlockID,
		POLRound:   proposal.POLRound,
		Round:      proposal.Round,
	}
}
