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
	"sync"

	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p/discover"
	"github.com/kardiachain/go-kardiamain/types"
)

type RoundVoteSet struct {
	Prevotes   *types.VoteSet
	Precommits *types.VoteSet
}

var (
	GotVoteFromUnwantedRoundError = errors.New("Peer has sent a vote that does not match our round for more than one round")
)

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
	height  *cmn.BigInt
	valSet  *types.ValidatorSet

	mtx               sync.Mutex
	round             *cmn.BigInt               // max tracked round
	roundVoteSets     map[int]RoundVoteSet      // keys: [0...round]
	peerCatchupRounds map[discover.NodeID][]int // keys: peer.ID; values: at most 2 rounds
}

func NewHeightVoteSet(logger log.Logger, chainID string, height *cmn.BigInt, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
		logger:  logger,
		chainID: chainID,
	}
	hvs.Reset(height, valSet)
	return hvs
}

func (hvs *HeightVoteSet) Reset(height *cmn.BigInt, valSet *types.ValidatorSet) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = make(map[int]RoundVoteSet)
	hvs.peerCatchupRounds = make(map[discover.NodeID][]int)

	hvs.addRound(0)
	hvs.round = cmn.NewBigInt32(0)
}

func (hvs *HeightVoteSet) addRound(round int) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		cmn.PanicSanity("addRound() for an existing round")
	}
	hvs.logger.Trace("addRound(round)", "round", round)
	prevotes := types.NewVoteSet(hvs.chainID, hvs.height, cmn.NewBigInt32(round), types.PrevoteType, hvs.valSet)
	precommits := types.NewVoteSet(hvs.chainID, hvs.height, cmn.NewBigInt32(round), types.PrecommitType, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round int) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	hvs.logger.Trace("Set round", "hvs.round", hvs.round, "round", round)
	if !hvs.round.EqualsInt(0) && hvs.round.Add(1).IsGreaterThanInt(round) {
		cmn.PanicSanity("SetRound() must increment hvs.round")
	}
	for r := hvs.round.Int32() + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = cmn.NewBigInt32(round)
}

// Duplicate votes return added=false, err=nil.
// By convention, peerID is "" if origin is self.
func (hvs *HeightVoteSet) AddVote(vote *types.Vote, peerID discover.NodeID) (added bool, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(vote.Type) {
		return
	}
	voteSet := hvs.getVoteSet(vote.Round.Int32(), vote.Type)
	if voteSet == nil {
		hvs.logger.Trace("Retrived VoteSet is nil", "H/R/T", cmn.Fmt("%v/%v/%v", vote.Height, vote.Round, types.GetReadableVoteTypeString(vote.Type)))
		if rndz := hvs.peerCatchupRounds[peerID]; len(rndz) < 2 {
			hvs.addRound(vote.Round.Int32())
			voteSet = hvs.getVoteSet(vote.Round.Int32(), vote.Type)
			hvs.peerCatchupRounds[peerID] = append(rndz, vote.Round.Int32())
		} else {
			// punish peer
			err = GotVoteFromUnwantedRoundError
			return
		}
	}

	added, err = voteSet.AddVote(vote)
	return
}

// Get all prevotes of the specified round.
func (hvs *HeightVoteSet) Prevotes(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.PrevoteType)
}

// Get vote set of the given round for specific type.
func (hvs *HeightVoteSet) getVoteSet(round int, t types.SignedMsgType) *types.VoteSet {
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch t {
	case types.PrevoteType:
		return rvs.Prevotes
	case types.PrecommitType:
		return rvs.Precommits
	default:
		cmn.PanicSanity(cmn.Fmt("Unexpected vote type %X", t))
		return nil
	}
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
func (hvs *HeightVoteSet) SetPeerMaj23(round int, t types.SignedMsgType, peerID discover.NodeID, blockID types.BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(t) {
		return fmt.Errorf("SetPeerMaj23: Invalid vote type %v", t)
	}
	voteSet := hvs.getVoteSet(round, t)
	if voteSet == nil {
		return nil // something we don't know about yet
	}
	return voteSet.SetPeerMaj23(peerID, blockID)
}

// Returns last round and blockID that has +2/3 prevotes for a particular block
// or nil. Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLInfo() (polRound int, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round.Int32(); r >= 0; r-- {
		rvs := hvs.getVoteSet(r, types.PrevoteType)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, types.BlockID{}
}

func (hvs *HeightVoteSet) Precommits(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.PrecommitType)
}
