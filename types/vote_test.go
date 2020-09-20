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
	"bytes"
	"testing"
)

// NOTE: privValidators are in order
func randVoteSet(
	height uint64,
	round uint32,
	signedMsgType byte,
	numValidators int,
	votingPower uint64,
) (*VoteSet, *ValidatorSet, []PrivValidator) {
	valSet, privValidators := RandValidatorSet(numValidators, votingPower)
	return NewVoteSet("test_chain_id", height, round, signedMsgType, valSet), valSet, privValidators
}

func TestVoteCreationAndCopy(t *testing.T) {
	vote := CreateEmptyVote()

	voteCopy := vote.Copy()

	if rlpHash(vote) != rlpHash(voteCopy) {
		t.Fatal("Error, Vote Copy wrong")
	}
	if &voteCopy == &vote {
		t.Fatal("Address of vote and vote2 are the same")
	}

}
func TestVoteByteEncoding(t *testing.T) {
	firstVote := CreateEmptyVote()

	firstByte := firstVote.SignBytes("KAI")
	secondByte := firstVote.SignBytes("ETH")

	if bytes.Equal(firstByte, secondByte) {
		t.Fatal("SignBytes expected to be different for different votes")
	}
}

func TestVoteTypeFunctions(t *testing.T) {
	firstVote := CreateEmptyVote()
	secondVote := firstVote.Copy()
	firstVote.Type = byte(0x01)  //Prevote
	secondVote.Type = byte(0x02) //Precommit

	if GetReadableVoteTypeString(firstVote.Type) != "Prevote" || GetReadableVoteTypeString(secondVote.Type) != "Precommit" {
		t.Fatal("Issue translating vote types from bytes to string")
	}

	if !IsVoteTypeValid(firstVote.Type) || !IsVoteTypeValid(secondVote.Type) || IsVoteTypeValid(byte(0xFF)) {
		t.Fatal("Valid vote type not found")
	}

}
