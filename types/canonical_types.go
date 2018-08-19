package types

import (
	"math/big"

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
	Timestamp  *big.Int    `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
}

type CanonicalVote struct {
	ChainID   string      `json:"@chain_id"`
	Type      string      `json:"@type"`
	BlockID   BlockID     `json:"block_id"`
	Height    *cmn.BigInt `json:"height"`
	Round     *cmn.BigInt `json:"round"`
	Timestamp *big.Int    `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	VoteType  byte        `json:"type"`
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

func CreateCanonicalVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		ChainID:   chainID,
		Type:      "vote",
		BlockID:   vote.BlockID,
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: vote.Timestamp,
		VoteType:  vote.Type,
	}
}
