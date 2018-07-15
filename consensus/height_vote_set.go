package types

import (
	"sync"

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
