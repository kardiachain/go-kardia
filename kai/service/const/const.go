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

package service

// Constants to match up protocol versions and messages
const (
	kai1 = 1
)

// ProtocolVersions are the supported versions of the protocol (first is primary).
var ProtocolVersions = []uint{kai1}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{19}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// protocol message codes
const (
	// Protocol messages belonging to kai1
	StatusMsg              = 0x00
	TxMsg                  = 0x01
	CsNewRoundStepMsg      = 0x02 // Consensus message
	CsProposalMsg          = 0x03 // Proposal message
	CsVoteMsg              = 0x04 // Vote message
	CsCommitStepMsg        = 0x05 // Commit step message
	CsHasVoteMsg           = 0x06 // Has vote message
	CsProposalPOLMsg       = 0x07 // Proposal message
	CsBlockMsg             = 0x08 // Block message
	CsVoteSetMaj23Message  = 0x09 // VoteSetMaj23 message
	CsVoteSetBitsMessage   = 0x10 // VoteSetBitsMessage message
	CsProposalBlockPartMsg = 0x11 // CsProposalBlockPartMsg message
	CsValidBlockMsg        = 0x12 // CsValidBlockMsg message
)
