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
	"github.com/kardiachain/go-kardia/lib/rlp"
)

// Proposal defines a block proposal for the consensus.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height     *cmn.BigInt `json:"height"`
	Round      *cmn.BigInt `json:"round"`
	Timestamp  *big.Int    `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	Block      *Block      `json:"block"`
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

// This function is used to address RLP's diosyncrasies (issues#73), enabling
// RLP encoding/decoding to pass.
// Note: Use this "before" sending the object to other peers.
func (p *Proposal) MakeNilEmpty() {
	p.Block.MakeNilEmpty()
}

// This function is used to address RLP's diosyncrasies (issues#73), enabling
// RLP encoding/decoding to pass.
// Note: Use this "after" receiving the object to other peers.
func (p *Proposal) MakeEmptyNil() {
	p.Block.MakeEmptyNil()
}

// SignBytes returns the Proposal bytes for signing
func (p *Proposal) SignBytes(chainID string) []byte {
	bz, err := rlp.EncodeToBytes(CreateCanonicalProposal(chainID, p))
	if err != nil {
		panic(err)
	}
	return bz
}

// String returns a short string representing the Proposal
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %X @%v}",
		p.Height, p.Round, p.Block, p.POLRound,
		p.POLBlockID,
		cmn.Fingerprint(p.Signature[:]),
		time.Unix(p.Timestamp.Int64(), 0))
}
