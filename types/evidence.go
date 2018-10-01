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
	"crypto/ecdsa"
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                       // height of the equivocation
	Address() []byte                                     // address of the equivocating validator
	Hash() []byte                                        // hash of the evidence
	Verify(chainID string, pubKey ecdsa.PublicKey) error // verify the evidence
	Equal(Evidence) bool                                 // check equality of evidence

	String() string
}

// DuplicateVoteEvidence contains evidence a validator signed two conflicting votes.
type DuplicateVoteEvidence struct {
	PubKey ecdsa.PublicKey
	VoteA  *Vote
	VoteB  *Vote
}
