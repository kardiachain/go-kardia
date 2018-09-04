package types

import (
	"fmt"
	"sync"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p/discover"
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
	chainID string
	height  *cmn.BigInt
	valSet  *types.ValidatorSet

	mtx           sync.Mutex
	round         *cmn.BigInt          // max tracked round
	roundVoteSets map[int]RoundVoteSet // keys: [0...round]
	// TODO(huny@): Do we need the peer catch up rounds?
	// peerCatchupRounds map[p2p.ID][]int     // keys: peer.ID; values: at most 2 rounds
}

func NewHeightVoteSet(chainID string, height *cmn.BigInt, valSet *types.ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{
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
	//namdoh@ hvs.peerCatchupRounds = make(map[p2p.ID][]int)

	hvs.addRound(0)
	hvs.round = cmn.NewBigInt(0)
}

func (hvs *HeightVoteSet) addRound(round int) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		cmn.PanicSanity("addRound() for an existing round")
	}
	// log.Debug("addRound(round)", "round", round)
	prevotes := types.NewVoteSet(hvs.chainID, hvs.height, cmn.NewBigInt(int64(round)), types.VoteTypePrevote, hvs.valSet)
	precommits := types.NewVoteSet(hvs.chainID, hvs.height, cmn.NewBigInt(int64(round)), types.VoteTypePrecommit, hvs.valSet)
	hvs.roundVoteSets[round] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round int) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	log.Trace("Set round", "hvs.round", hvs.round, "round", round)
	if !hvs.round.EqualsInt(0) && hvs.round.Add(1).IsGreaterThanInt(round) {
		cmn.PanicSanity("SetRound() must increment hvs.round")
	}
	for r := hvs.round.Int32() + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = cmn.NewBigInt(int64(round))
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
		panic("HeightVoteSet.AddVote - not yet implemented")
		// TODO(namdoh): Re-enable this later.
		//if rndz := hvs.peerCatchupRounds[peerID]; len(rndz) < 2 {
		//	hvs.addRound(vote.Round)
		//	voteSet = hvs.getVoteSet(vote.Round, vote.Type)
		//	hvs.peerCatchupRounds[peerID] = append(rndz, vote.Round)
		//} else {
		//	// punish peer
		//	err = GotVoteFromUnwantedRoundError
		//	return
		//}
	}
	added, err = voteSet.AddVote(vote)
	return
}

// Get all prevotes of the specified round.
func (hvs *HeightVoteSet) Prevotes(round int) *types.VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, types.VoteTypePrevote)
}

// Get vote set of the given round for specific type.
func (hvs *HeightVoteSet) getVoteSet(round int, type_ byte) *types.VoteSet {
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch type_ {
	case types.VoteTypePrevote:
		return rvs.Prevotes
	case types.VoteTypePrecommit:
		return rvs.Precommits
	default:
		cmn.PanicSanity(cmn.Fmt("Unexpected vote type %X", type_))
		return nil
	}
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
func (hvs *HeightVoteSet) SetPeerMaj23(round int, type_ byte, peerID discover.NodeID, blockID types.BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(type_) {
		return fmt.Errorf("SetPeerMaj23: Invalid vote type %v", type_)
	}
	voteSet := hvs.getVoteSet(round, type_)
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
		rvs := hvs.getVoteSet(r, types.VoteTypePrevote)
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
	return hvs.getVoteSet(round, types.VoteTypePrecommit)
}
