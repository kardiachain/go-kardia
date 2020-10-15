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
	"errors"
	"fmt"
	"time"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/protoio"
	tmproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

// Proposal defines a block proposal for the consensus.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height     uint64    `json:"height"`
	Round      uint32    `json:"round"`
	POLRound   uint32    `json:"pol_round"`
	Timestamp  time.Time `json:"timestamp"`    // -1 if null.
	POLBlockID BlockID   `json:"pol_block_id"` // zero if null.
	Signature  []byte    `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height uint64, round uint32, polRound uint32, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:     height,
		Round:      round,
		Timestamp:  time.Now(),
		POLRound:   polRound,
		POLBlockID: polBlockID,
	}
}

// ProposalSignBytes returns the proto-encoding of the canonicalized Proposal,
// for signing. Panics if the marshaling fails.
//
// The encoded Protobuf message is varint length-prefixed (using MarshalDelimited)
// for backwards-compatibility with the Amino encoding, due to e.g. hardware
// devices that rely on this encoding.
//
// See CanonicalizeProposal
func ProposalSignBytes(chainID string, p *tmproto.Proposal) []byte {
	pb := CreateCanonicalProposal(chainID, p)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

// String returns a short string representing the Proposal
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v) %X @%v}",
		p.Height, p.Round, p.POLRound,
		p.POLBlockID,
		cmn.Fingerprint(p.Signature[:]),
		p.Timestamp.Unix())
}

// ValidateBasic performs basic validation.
func (p *Proposal) ValidateBasic() error {
	if p.Height < 0 {
		return errors.New("negative Height")
	}
	if p.Round < 0 {
		return errors.New("negative Round")
	}

	if err := p.POLBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// ValidateBasic above would pass even if the BlockID was empty:
	if !p.POLBlockID.IsComplete() {
		return fmt.Errorf("expected a complete, non-empty BlockID, got: %v", p.POLBlockID)
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if len(p.Signature) == 0 {
		return errors.New("signature is missing")
	}

	return nil
}

// ToProto converts Proposal to protobuf
func (p *Proposal) ToProto() *tmproto.Proposal {
	if p == nil {
		return &tmproto.Proposal{}
	}
	pb := new(tmproto.Proposal)

	pb.BlockID = p.POLBlockID.ToProto()
	//pb.Type = p.Type
	pb.Height = p.Height
	pb.Round = p.Round
	pb.PolRound = p.POLRound
	pb.Timestamp = p.Timestamp
	pb.Signature = p.Signature

	return pb
}

// FromProto sets a protobuf Proposal to the given pointer.
// It returns an error if the proposal is invalid.
func ProposalFromProto(pp *tmproto.Proposal) (*Proposal, error) {
	if pp == nil {
		return nil, errors.New("nil proposal")
	}

	p := new(Proposal)

	blockID, err := BlockIDFromProto(&pp.BlockID)
	if err != nil {
		return nil, err
	}

	p.POLBlockID = *blockID
	//p.Type = pp.Type
	p.Height = pp.Height
	p.Round = pp.Round
	p.POLRound = pp.PolRound
	p.Timestamp = pp.Timestamp
	p.Signature = pp.Signature

	return p, p.ValidateBasic()
}
