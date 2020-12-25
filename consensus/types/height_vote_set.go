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
	"sync"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
)

type RoundVoteSet struct {
	Prevotes   *types.VoteSet
	Precommits *types.VoteSet
}

/*
Keeps track of all VoteSets from round 0 to round 'round'.
Also keeps track of up to one RoundVoteSet greater than
'round' from each peer, to facilitate catchup syncing of commits.
A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
We let each peer provide us with up to 2 unexpected "catchup" rounds.
One for their LastCommit round, and another for the official commit round.
*/
type HeightVoteSet struct {
	logger  log.Logger
	chainID string
	height  uint64
	valSet  *types.ValidatorSet

	mtx               sync.Mutex
	round             uint32                  // max tracked round
	roundVoteSets     map[uint32]RoundVoteSet // keys: [0...round]
	peerCatchupRounds map[p2p.ID][]uint32     // keys: peer.ID; values: at most 2 rounds
}

func NewHeightVoteSet(logger log.Logger, chainID string, height uint64, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		logger:  logger,
		chainID: chainID,
	}

	// Reset HVS
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = make(map[uint32]RoundVoteSet)
	hvs.peerCatchupRounds = make(map[p2p.ID][]uint32)
	hvs.addRound(1)
	hvs.round = 1

	return hvs
}

func (hvs *HeightVoteSet) addRound(round uint32) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		cmn.PanicSanity("addRound() for an existing round")
	}
	hvs.logger.Trace("addRound(round)", "round", round)
	prevotes := types.NewVoteSet(hvs.chainID, hvs.height, round, kproto.PrevoteType, hvs.valSet)
	precommits := types.NewVoteSet(hvs.chainID, hvs.height, round, kproto.PrecommitType, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round uint32) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.logger.Trace("Set round", "hvs.round", hvs.round, "round", round)
	if (hvs.round == 0) && (round < hvs.round-1) {
		cmn.PanicSanity("SetRound() must increment hvs.round")
	}
	for r := hvs.round - 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

// Duplicate votes return added=false, err=nil.
// By convention, peerID is "" if origin is self.
func (hvs *HeightVoteSet) AddVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(vote.Type) {
		return false, ErrNilVoteType
	}
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		hvs.logger.Trace("Retrived VoteSet is nil", "H/R/T", cmn.Fmt("%v/%v/%v", vote.Height, vote.Round, types.GetReadableVoteTypeString(vote.Type)))
		if rndz := hvs.peerCatchupRounds[peerID]; len(rndz) < 2 {
			hvs.addRound(vote.Round)
			voteSet = hvs.getVoteSet(vote.Round, vote.Type)
			hvs.peerCatchupRounds[peerID] = append(rndz, vote.Round)
		} else {
			// punish peer
			return false, ErrGotVoteFromUnwantedRound
		}
	}
	return voteSet.AddVote(vote)
}

// Get all prevotes of the specified round.
func (hvs *HeightVoteSet) Prevotes(round uint32) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, kproto.PrevoteType)
}

// Get vote set of the given round for specific type.
func (hvs *HeightVoteSet) getVoteSet(round uint32, signedMsgType kproto.SignedMsgType) *types.VoteSet {
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch signedMsgType {
	case kproto.PrevoteType:
		return rvs.Prevotes
	case kproto.PrecommitType:
		return rvs.Precommits
	default:
		cmn.PanicSanity(cmn.Fmt("Unexpected vote type %X", signedMsgType))
		return nil
	}
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
func (hvs *HeightVoteSet) SetPeerMaj23(round uint32, signedMsgType kproto.SignedMsgType, peerID p2p.ID, blockID types.BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(signedMsgType) {
		return fmt.Errorf("SetPeerMaj23: Invalid vote type %v", signedMsgType)
	}
	voteSet := hvs.getVoteSet(round, signedMsgType)
	if voteSet == nil {
		return nil // something we don't know about yet
	}
	return voteSet.SetPeerMaj23(peerID, blockID)
}

// Returns last round and blockID that has +2/3 prevotes for a particular block
// or nil. Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLInfo() (polRound uint32, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 1; r-- {
		rvs := hvs.getVoteSet(r, kproto.PrevoteType)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return 0, types.BlockID{}
}

func (hvs *HeightVoteSet) Precommits(round uint32) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, kproto.PrecommitType)
}
