package types

import (
	"sync"

	cmn "github.com/kardiachain/go-kardia/libs/common"
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
	height  int64
	valSet  *types.ValidatorSet

	mtx           sync.Mutex
	round         int                  // max tracked round
	roundVoteSets map[int]RoundVoteSet // keys: [0...round]
	// TODO(huny@): Do we need the peer catch up rounds?
	// peerCatchupRounds map[p2p.ID][]int     // keys: peer.ID; values: at most 2 rounds
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

// Returns last round and blockID that has +2/3 prevotes for a particular block
// or nil. Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLInfo() (polRound int, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- {
		rvs := hvs.getVoteSet(r, types.VoteTypePrevote)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, types.BlockID{}
}
