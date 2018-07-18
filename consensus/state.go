package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/types"
)

// ConsensusState handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator,
// and from a timer.
type ConsensusState struct {
	Logger log.Logger

	// config details
	config        *cfg.ConsensusConfig
	privValidator types.PrivValidator // for signing votes

	// services for creating and executing blocks
	// TODO: encapsulate all of this in one "BlockManager"
	blockExec  *sm.BlockExecutor
	blockStore sm.BlockStore
	mempool    sm.Mempool
	evpool     sm.EvidencePool

	// internal state
	mtx sync.Mutex
	cstypes.RoundState
	state sm.State // State until height-1.

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    TimeoutTicker

	// we use eventBus to trigger msg broadcasts in the reactor,
	// and to notify external subscribers, eg. through a websocket
	eventBus *types.EventBus

	// a Write-Ahead Log ensures we can recover from any kind of crash
	// and helps us avoid signing conflicting votes
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay
	doWALCatchup bool // determines if we even try to do the catchup

	// for tests where we want to limit the number of transitions the state makes
	nSteps int

	// some functions can be overwritten for testing
	decideProposal func(height int64, round int)
	doPrevote      func(height int64, round int)
	setProposal    func(proposal *types.Proposal) error

	// closed when we finish shutting down
	done chan struct{}

	// synchronous pubsub between consensus state and reactor.
	// state only emits EventNewRoundStep, EventVote and EventProposalHeartbeat
	evsw tmevents.EventSwitch

	// for reporting metrics
	metrics *Metrics
}

// Creates the next block to propose and returns it. Returns nil block upon
// error.
func (cs *ConsensusState) createProposalBlock() (block *types.Block) {
	var commit *types.Commit
	if cs.Height == 1 {
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = &types.Commit{}
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()
	} else {
		// This shouldn't happen.
		cs.Logger.Error(`enterPropose: Cannot propose anything: No commit for 
		                 the previous block.`)
		return
	}

	// Mempool validated transactions
	txs := cs.mempool.Reap(cs.state.ConsensusParams.BlockSize.MaxTxs)
	block, parts := cs.state.MakeBlock(cs.Height, txs, commit)
	evidence := cs.evpool.PendingEvidence()
	block.AddEvidence(evidence)
	return block, parts
}
