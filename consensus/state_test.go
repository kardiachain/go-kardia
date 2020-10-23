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

package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	cstypes "github.com/kardiachain/go-kardiamain/consensus/types"
	"github.com/kardiachain/go-kardiamain/lib/common"
	kpubsub "github.com/kardiachain/go-kardiamain/lib/pubsub"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateProposerSelection0(t *testing.T) {
	cs1, vss := randState(4)
	height, round := cs1.Height, cs1.Round

	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)

	// set validator
	startTestRound(cs1, height, round)

	// Wait for new round so proposer is set.
	ensureNewRound(newRoundCh, height, round)
	prop := cs1.GetRoundState().Validators.GetProposer()
	pv := cs1.privValidator

	if prop.Address != pv.GetAddress() {
		t.Fatalf("expected proposer to be validator %d. Got %X", 0, prop.Address)
	}

	// Wait for complete proposal.
	ensureNewProposal(proposalCh, height, round)

	rs := cs1.GetRoundState()
	signAddVotes(cs1, kproto.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:]...)
	incrementRound(vss[1:]...)

	// Wait for new round so next validator is set.
	ensureNewRound(newRoundCh, height+1, 0)

	// check validator
	prop = cs1.GetRoundState().Validators.GetProposer()
	addr := vss[0].PrivVal.GetAddress()

	if !prop.Address.Equal(addr) {
		panic(fmt.Sprintf("expected validator %d. Got %X", 0, addr))
	}
}

//starting from round 3 instead of 1
func TestStateProposerSelection2(t *testing.T) {
	cs1, vss := randState(4)
	height := cs1.Height

	// this time we jump in at round 3
	incrementRound(vss[1:]...)
	incrementRound(vss[1:]...)

	var round uint32 = 3
	startTestRound(cs1, height, round)

	time.Sleep(3000 * time.Millisecond)

	// everyone just votes nil. we get a new proposer each round
	for i := uint32(0); uint32(i) < uint32(len(vss)); i++ {
		prop := cs1.GetRoundState().Validators.GetProposer()
		priVal := (vss[(uint32(i)+2)%uint32(len(vss))].PrivVal)
		correctProposer := priVal.GetAddress()
		if prop.Address != correctProposer {
			panic(fmt.Sprintf(
				"expected RoundState.Validators.GetProposer() to be validator %d. Got %X",
				int(i+2)%len(vss),
				prop.Address))
		}

		rs := cs1.GetRoundState()
		signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), rs.ProposalBlockParts.Header(), vss[1:]...)
		incrementRound(vss[1:]...)
		time.Sleep(4000 * time.Millisecond)
	}

}

func TestStateBadProposal(t *testing.T) {
	cs1, vss := randState(2)
	height, round := cs1.Height, cs1.Round
	vs2 := vss[1]
	privVal2 := vss[1].PrivVal

	partSize := types.BlockPartSizeBytes

	propBlock, _ := cs1.createProposalBlock() //changeProposer(t, cs1, vs2)

	round++

	incrementRound(vss[1:]...)

	// make the block bad by tampering with statehash
	propBlockParts := propBlock.MakePartSet(uint32(partSize))
	blockID := types.BlockID{Hash: propBlock.Hash(), PartsHeader: propBlockParts.Header()}
	proposal := types.NewProposal(uint64(vs2.Height), round, uint32(0), blockID)
	p := proposal.ToProto()
	if err := privVal2.SignProposal("Kaicon", p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature
	// set the proposal block
	if err := cs1.setProposal(proposal); err != nil {
		t.Fatal(err)
	}

	// start the machine
	startTestRound(cs1, height, round)

	// add bad prevote from vs2 and wait for it
	signAddVotes(cs1, kproto.PrecommitType, propBlock.Hash(), propBlock.MakePartSet(uint32(partSize)).Header(), vs2)
	time.Sleep(300 * time.Millisecond)

	ensurePrecommit()
	validatePrecommit(t, cs1, round, uint32(0), vss[1], common.BytesToHash(nil), common.BytesToHash(nil))

	// wait for precommit
	signAddVotes(cs1, kproto.PrecommitType, propBlock.Hash(), propBlock.MakePartSet(uint32(partSize)).Header(), vs2)

}

// propose, prevote, and precommit a block
func TestStateFullRound1(t *testing.T) {
	cs, vss := randState(1)
	height, round := cs.Height, cs.Round
	voteCh := subscribeUnBuffered(cs.eventBus, types.EventQueryVote)
	startTestRound(cs, height, round)
	// ensureNewProposal(propCh, height, round)
	propBlockHash := cs.GetRoundState().ProposalBlock.Hash()
	// wait for prevote
	ensurePrevote(newRoundCh, height, round)
	validatePrevote(t, cs, round, vss[0], propBlockHash)

}

// run through propose, prevote, precommit commit with two validators
// where the first validator has to wait for votes from the second
func TestStateFullRound2(t *testing.T) {
	cs1, vss := randState(2)
	vs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	// we should be stuck in limbo waiting for more prevotes
	rs := cs1.GetRoundState()
	propBlockHash, propPartSetHeader := rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header()

	// prevote arrives from vs2:
	signAddVotes(cs1, kproto.PrevoteType, propBlockHash, propPartSetHeader, vs2)
	ensurePrevote()
	time.Sleep(5000 * time.Millisecond)

	ensurePrecommit()
	validatePrecommit(t, cs1, round, uint32(0), vss[0], propBlockHash, propBlockHash)
	// precommit arrives from vs2:
	signAddVotes(cs1, kproto.PrecommitType, propBlockHash, propPartSetHeader, vs2)
	// time.Sleep(6000 * time.Millisecond)

}

// two validators, 4 rounds.
// two vals take turns proposing. val1 locks on first one, precommits nil on everything else
func TestStateLockNoPOL(t *testing.T) {
	cs1, vss := randState(2)
	vs2 := vss[1]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	// start round and wait for prevote
	cs1.enterNewRound(height, round)
	cs1.Start()

	roundState := cs1.GetRoundState()
	theBlockHash := roundState.ProposalBlock.Hash()
	thePartSetHeader := roundState.ProposalBlockParts.Header()

	// we should now be stuck in limbo forever, waiting for more prevotes
	// prevote arrives from vs2:
	signAddVotes(cs1, kproto.PrevoteType, theBlockHash, thePartSetHeader, vs2)

	time.Sleep(5000 * time.Millisecond)
	//ensure precommit
	ensurePrecommit()
	validatePrecommit(t, cs1, round, round, vss[0], theBlockHash, theBlockHash)

	// we should now be stuck in forever, waiting for more precommits
	// lets add one for a different block
	hash := make([]byte, len(theBlockHash))
	copy(hash, theBlockHash.Bytes())
	hash[0] = (hash[0] + 1) % 255
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(hash), thePartSetHeader, vs2)

	round++ // moving to the next round
	t.Log("#### ONTO ROUND 1")

	incrementRound(vs2)

	// now we're on a new round and not the proposer, so wait for timeout
	rs := cs1.GetRoundState()

	time.Sleep(8000 * time.Millisecond)
	// we should have prevoted our locked block
	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], rs.LockedBlock.Hash())

	// add a conflicting prevote from the other validator
	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(hash), rs.LockedBlock.MakePartSet(uint32(partSize)).Header(), vs2)

	time.Sleep(8000 * time.Millisecond)
	ensurePrecommit()
	validatePrecommit(t, cs1, round, 0, vss[0], common.BytesToHash(nil), theBlockHash)

	// now we're going to enter prevote again, but with invalid args
	// and then prevote wait, which should timeout. then wait for precommit
	time.Sleep(3000 * time.Millisecond)

	// add conflicting precommit from vs2
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(hash), rs.LockedBlock.MakePartSet(uint32(partSize)).Header(), vs2)

	// (note we're entering precommit for a second time this round, but with invalid args
	// then we enterPrecommitWait and timeout into NewRound
	round++ // entering new round
	t.Log("#### ONTO ROUND 2")

	incrementRound(vs2)

	rs = cs1.GetRoundState()
	// now we're on a new round and are the proposer
	if rs.ProposalBlock.Hash() != rs.LockedBlock.Hash() {
		panic(fmt.Sprintf(
			"Expected proposal block to be locked block. Got %v, Expected %v",
			rs.ProposalBlock,
			rs.LockedBlock))
	}

	time.Sleep(5000 * time.Millisecond)
	validatePrevote(t, cs1, round, vss[0], rs.LockedBlock.Hash())
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(hash), rs.ProposalBlock.MakePartSet(uint32(partSize)).Header(), vs2) // NOTE: conflicting precommits at same height

	cs2, _ := randState(2) // needed so generated block is different than locked block
	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs2, vs2, vs2.Height, vs2.Round+1)

	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}

	incrementRound(vs2)

	round++ // entering new round
	t.Log("#### ONTO ROUND 3")

	//ensure prevote
	ensurePrevote()

	// now we're on a new round and not the proposer
	// validatePrevote(t, cs1, round, vss[0], cs1.LockedBlock.Hash())

	// prevote for proposed block
	signAddVotes(cs1, kproto.PrevoteType, propBlock.Hash(), propBlock.MakePartSet(uint32(partSize)).Header(), vs2)
	signAddVotes(cs1, kproto.PrecommitType, propBlock.Hash(), propBlock.MakePartSet(uint32(partSize)).Header(), vs2) // NOTE: conflicting precommits at same height
}

// 4 vals in two rounds,
// in round one: v1 precommits, other 3 only prevote so the block isn't committed
// in round two: v1 prevotes the same block that the node is locked on
// the others prevote a new block hence v1 changes lock and precommits the new block with the others
func TestStateLockPOLRelockThenChangeLock(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	rs := cs1.GetRoundState()
	theBlockHash := rs.ProposalBlock.Hash()
	theBlockParts := rs.ProposalBlockParts.Header()

	signAddVotes(cs1, kproto.PrevoteType, theBlockHash, theBlockParts, vs2, vs3, vs4)

	time.Sleep(8000 * time.Millisecond)
	ensurePrecommit()
	validatePrecommit(t, cs1, round, round, vss[0], theBlockHash, theBlockHash)

	// add precommits from the rest
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	// before we timeout to the new round set the new proposal
	cs2, _ := randState(2)
	prop, propBlock := decideProposal(cs2, vs2, vs2.Height, vs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}
	propBlockParts := propBlock.MakePartSet(uint32(partSize))
	propBlockHash := propBlock.Hash()
	require.NotEqual(t, propBlockHash, theBlockHash)

	incrementRound(vs2, vs3, vs4)

	round++ // moving to the next round
	t.Log("### ONTO ROUND 1")

	time.Sleep(5000 * time.Millisecond)
	ensurePrevote()
	// go to prevote, node should prevote for locked block (not the new proposal) - this is relocking
	validatePrevote(t, cs1, round, vss[0], theBlockHash)

	// // now lets add prevotes from everyone else for the new block
	signAddVotes(cs1, kproto.PrevoteType, propBlockHash, propBlockParts.Header(), vs2, vs3, vs4)

	// more prevote creating a majority on the new block and this is then committed
	signAddVotes(cs1, kproto.PrecommitType, propBlockHash, propBlockParts.Header(), vs2, vs3)

}

// 4 vals, v1 locks on proposed block in the first round but the other validators only prevote
// In the second round, v1 misses the proposal but sees a majority prevote an unknown block so
// v1 should unlock and precommit nil. In the third round another block is proposed, all vals
// prevote and now v1 can lock onto the third block and precommit that
func TestStateLockPOLUnlockOnUnknownBlock(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	/*
		Round0 (cs1, A) // A A A A// A nil nil nil
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	rs := cs1.GetRoundState()
	firstBlockHash := rs.ProposalBlock.Hash()
	firstBlockParts := rs.ProposalBlockParts.Header()

	time.Sleep(3000 * time.Millisecond)

	signAddVotes(cs1, kproto.PrevoteType, firstBlockHash, firstBlockParts, vs2, vs3, vs4)

	time.Sleep(2000 * time.Millisecond)
	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], firstBlockHash)

	time.Sleep(2000 * time.Millisecond)
	ensurePrecommit()
	validatePrecommit(t, cs1, round, round, vss[0], firstBlockHash, firstBlockHash)

	// add precommits from the rest
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	// before we timeout to the new round set the new proposal
	cs2, err := newState(vs2, cs1.state)
	if err != nil {
		t.Fatal("Failed to create new state cs2")
	}
	prop, propBlock := decideProposal(cs2, vs2, vs2.Height, vs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}
	secondBlockParts := propBlock.MakePartSet(uint32(partSize))
	secondBlockHash := propBlock.Hash()
	require.NotEqual(t, secondBlockHash, firstBlockHash)

	incrementRound(vs2, vs3, vs4)

	// // timeout to new round
	time.Sleep(2000 * time.Millisecond)

	round++ // moving to the next round

	time.Sleep(3000 * time.Millisecond)
	t.Log("### ONTO ROUND 1")

	// now we're on a new round but v1 misses the proposal

	// go to prevote, node should prevote for locked block (not the new proposal) - this is relocking
	time.Sleep(3000 * time.Millisecond)

	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], firstBlockHash)

	// now lets add prevotes from everyone else for the new block
	signAddVotes(cs1, kproto.PrevoteType, secondBlockHash, secondBlockParts.Header(), vs2, vs3, vs4)

	time.Sleep(3000 * time.Millisecond)
	ensurePrecommit()
	// we should have unlocked and locked on the new block, sending a precommit for this new block
	validatePrecommit(t, cs1, round, uint32(0), vss[0], common.BytesToHash(nil), common.BytesToHash(nil))

	// more prevote creating a majority on the new block and this is then committed
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	// before we timeout to the new round set the new proposal
	cs3, err := newState(vs3, cs1.state)
	if err != nil {
		t.Fatal("Failed to create new state cs3")
	}
	prop, propBlock = decideProposal(cs3, vs3, vs3.Height, vs3.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}
	thirdPropBlockParts := propBlock.MakePartSet(uint32(partSize))
	thirdPropBlockHash := propBlock.Hash()
	require.NotEqual(t, secondBlockHash, thirdPropBlockHash)

	incrementRound(vs2, vs3, vs4)

	// timeout to new round
	time.Sleep(3000 * time.Millisecond)

	round++ // moving to the next round
	// ensureNewRound(newRoundCh, height, round)
	t.Log("### ONTO ROUND 2")

	/*
		Round2 (vs3, C) // C C C C // C nil nil nil)
	*/

	time.Sleep(6000 * time.Millisecond)
	ensurePrevote()
	// we are no longer locked to the first block so we should be able to prevote
	validatePrevote(t, cs1, round, vss[0], thirdPropBlockHash)

	signAddVotes(cs1, kproto.PrevoteType, thirdPropBlockHash, thirdPropBlockParts.Header(), vs2, vs3, vs4)

	time.Sleep(6000 * time.Millisecond)
	ensurePrecommit()
	// we have a majority, now vs1 can change lock to the third block
	validatePrecommit(t, cs1, round, round, vss[0], thirdPropBlockHash, thirdPropBlockHash)
}

// 4 vals, one precommits, other 3 polka at next round, so we unlock and precomit the polka
func TestStateLockPOLUnlock(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	// partSize := types.BlockPartSizeBytes

	// everything done from perspective of cs1

	/*
		Round1 (cs1, B) // B B B B // B nil B nil

		eg. didn't see the 2/3 prevotes
	*/

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)

	rs := cs1.GetRoundState()
	theBlockHash := rs.ProposalBlock.Hash()
	theBlockParts := rs.ProposalBlockParts.Header()

	time.Sleep(3000 * time.Millisecond)
	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], theBlockHash)

	signAddVotes(cs1, kproto.PrevoteType, theBlockHash, theBlockParts, vs2, vs3, vs4)

	time.Sleep(3000 * time.Millisecond)
	ensurePrecommit()
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], theBlockHash, theBlockHash)

	// add precommits from the rest
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs4)
	signAddVotes(cs1, kproto.PrecommitType, theBlockHash, theBlockParts, vs3)

	// before we time out into new round, set next proposal block
	prop, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round+1)
	if prop == nil || propBlock == nil {
		t.Fatal("Failed to create proposal block with vs2")
	}

	// timeout to new round
	time.Sleep(3000 * time.Millisecond)
	rs = cs1.GetRoundState()
	lockedBlockHash := rs.LockedBlock.Hash()

	incrementRound(vs2, vs3, vs4)
	round++ // moving to the next round

	t.Log("#### ONTO ROUND 1")
	/*
		Round2 (vs2, C) // B nil nil nil // nil nil nil _

		cs1 unlocks!
	*/

	time.Sleep(8000 * time.Millisecond)

	// go to prevote, prevote for locked block (not proposal)
	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], lockedBlockHash)
	// now lets add prevotes from everyone else for nil (a polka!)
	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	// the polka makes us unlock and precommit nil
	time.Sleep(3000 * time.Millisecond)
	ensurePrecommit()

	// we should have unlocked and committed nil
	// NOTE: since we don't relock on nil, the lock round is 0
	validatePrecommit(t, cs1, round, uint32(0), vss[0], common.BytesToHash(nil), common.BytesToHash(nil))

	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3)
}

// 4 vals, 3 Prevotes for nil from the higher round.
// P0 waits for timeoutPropose in the next round before entering prevote
func TestWaitingTimeoutProposeOnNewRound(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	// start round
	startTestRound(cs1, height, round)
	time.Sleep(3000 * time.Millisecond)

	ensurePrevote()

	incrementRound(vss[1:]...)
	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	round++ // moving to the next round

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepPrevote) // P0 does not prevote before timeoutPropose expires

	time.Sleep(6000 * time.Millisecond)

	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], common.BytesToHash(nil))
}

// 4 vals, 3 Precommits for nil from the higher round.
// P0 jump to higher round, precommit and start precommit wait
func TestRoundSkipOnNilPolkaFromHigherRound(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	// start round
	startTestRound(cs1, height, round)
	time.Sleep(3000 * time.Millisecond)

	ensurePrevote()

	incrementRound(vss[1:]...)
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	// moving to the next round
	round++

	ensurePrecommit()
	validatePrecommit(t, cs1, round, uint32(0), vss[0], common.BytesToHash(nil), common.BytesToHash(nil))

}

// 4 vals, 3 Prevotes for nil in the current round.
// P0 wait for timeoutPropose to expire before sending prevote.
func TestWaitTimeoutProposeOnNilPolkaForTheCurrentRound(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, uint32(2)

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)

	// ensureNewRound(newRoundCh, height, round)

	incrementRound(vss[1:]...)
	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	time.Sleep(6000 * time.Millisecond)

	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], common.BytesToHash(nil))
}

// P0 emit NewValidBlock event upon receiving 2/3+ Precommit for B
func TestEmitNewValidBlockEventOnCommitWithoutBlock(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, uint32(2)

	incrementRound(vs2, vs3, vs4)

	partSize := types.BlockPartSizeBytes

	_, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round)
	propBlockHash := propBlock.Hash()
	propBlockParts := propBlock.MakePartSet(uint32(partSize))

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)

	time.Sleep(3000 * time.Millisecond)
	// vs2, vs3 and vs4 send precommit for propBlock
	signAddVotes(cs1, kproto.PrecommitType, propBlockHash, propBlockParts.Header(), vs2, vs3, vs4)

	time.Sleep(3000 * time.Millisecond)

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepCommit)
	assert.True(t, rs.ProposalBlock.Hash() == common.BytesToHash(nil))
	// assert.True(t, rs.ProposalBlockParts.Header().Equals(propBlockParts.Header()))

}

// P0 receives 2/3+ Precommit for B for round 0, while being in round 1. It emits NewValidBlock event.
// After receiving block, it executes block and moves to the next height.
func TestCommitFromPreviousRound(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, uint32(2)

	partSize := types.BlockPartSizeBytes

	_, propBlock := decideProposal(cs1, vs2, vs2.Height, vs2.Round)
	propBlockHash := propBlock.Hash()
	propBlockParts := propBlock.MakePartSet(uint32(partSize))

	// start round in which PO is not proposer
	startTestRound(cs1, height, round)
	time.Sleep(3000 * time.Millisecond)

	// vs2, vs3 and vs4 send precommit for propBlock for the previous round
	signAddVotes(cs1, kproto.PrecommitType, propBlockHash, propBlockParts.Header(), vs2, vs3, vs4)

	time.Sleep(3000 * time.Millisecond)

	rs := cs1.GetRoundState()
	assert.True(t, rs.Step == cstypes.RoundStepCommit)
	assert.True(t, rs.CommitRound == uint32(vs2.Round))

}

// 2 vals precommit votes for a block but node times out waiting for the third. Move to next round
// and third precommit arrives which leads to the commit of that header and the correct
// start of the next round
func TestStartNextHeightCorrectlyAfterTimeout(t *testing.T) {
	cs1, vss := randState(4)

	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	height, round := cs1.Height, cs1.Round

	// start round and wait for propose and prevote
	startTestRound(cs1, height, round)
	time.Sleep(3000 * time.Millisecond)

	rs := cs1.GetRoundState()
	theBlockHash := rs.ProposalBlock.Hash()
	theBlockParts := rs.ProposalBlockParts.Header()

	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], theBlockHash)

	signAddVotes(cs1, kproto.PrevoteType, theBlockHash, theBlockParts, vs2, vs3, vs4)

	time.Sleep(3000 * time.Millisecond)
	ensurePrecommit()
	// the proposed block should now be locked and our precommit added
	validatePrecommit(t, cs1, round, round, vss[0], theBlockHash, theBlockHash)

	// add precommits
	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2)
	signAddVotes(cs1, kproto.PrecommitType, theBlockHash, theBlockParts, vs3)

	// wait till timeout occurs
	time.Sleep(3000 * time.Millisecond)

	// majority is now reached
	signAddVotes(cs1, kproto.PrecommitType, theBlockHash, theBlockParts, vs4)

	rs = cs1.GetRoundState()
	assert.False(t, rs.TriggeredTimeoutPrecommit, "triggeredTimeoutPrecommit should be false at the beginning of each round")
}

// 4 vals.
// polka P0 at R0 for B0. We lock B0 on P0 at R0. P0 unlocks value at R1.
// P0 proposes B0 at R3.
func TestProposeValidBlock(t *testing.T) {
	cs1, vss := randState(4)
	vs2, vs3, vs4 := vss[1], vss[2], vss[3]
	_, round := cs1.Height, cs1.Round

	partSize := types.BlockPartSizeBytes

	// start round and wait for propose and prevote
	startTestRound(cs1, cs1.Height, round)

	time.Sleep(1000 * time.Millisecond)

	rs := cs1.GetRoundState()
	propBlock := rs.ProposalBlock
	propBlockHash := propBlock.Hash()

	time.Sleep(2000 * time.Millisecond)
	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], propBlockHash)

	// the others sign a polka
	signAddVotes(cs1, kproto.PrevoteType, propBlockHash, propBlock.MakePartSet(uint32(partSize)).Header(), vs2, vs3, vs4)

	time.Sleep(2000 * time.Millisecond)

	ensurePrecommit()
	// we should have precommitted
	validatePrecommit(t, cs1, round, round, vss[0], propBlockHash, propBlockHash)

	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	time.Sleep(2000 * time.Millisecond)

	incrementRound(vs2, vs3, vs4)
	round++ // moving to the next round

	t.Log("### ONTO ROUND 2")

	// timeout of propose
	time.Sleep(9000 * time.Millisecond)

	ensurePrevote()
	validatePrevote(t, cs1, round, vss[0], propBlockHash)

	signAddVotes(cs1, kproto.PrevoteType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)

	//ensure unlock
	time.Sleep(2500 * time.Millisecond)

	ensurePrecommit()
	// we should have precommitted
	validatePrecommit(t, cs1, round, uint32(0), vss[0], common.BytesToHash(nil), common.BytesToHash(nil))

	incrementRound(vs2, vs3, vs4)
	incrementRound(vs2, vs3, vs4)

	signAddVotes(cs1, kproto.PrecommitType, common.BytesToHash(nil), types.PartSetHeader{}, vs2, vs3, vs4)
	// time.Sleep(3000 * time.Millisecond)

	round += 2 // moving to the next round

	t.Log("### ONTO ROUND 3")

	// time.Sleep(1000 * time.Millisecond)

	round++ // moving to the next round

	t.Log("### ONTO ROUND 4")

	time.Sleep(3000 * time.Millisecond)

	rs = cs1.GetRoundState()
	assert.True(t, rs.ProposalBlock.Hash() == propBlockHash)
	assert.True(t, rs.ProposalBlock.Hash() == rs.ValidBlock.Hash())

}

// subscribe subscribes test client to the given query and returns a channel with cap = 1.
func subscribe(eventBus *types.EventBus, q kpubsub.Query) <-chan kpubsub.Message {
	sub, err := eventBus.Subscribe(context.Background(), testSubscriber, q)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, q))
	}
	return sub.Out()
}

// subscribe subscribes test client to the given query and returns a channel with cap = 0.
func subscribeUnBuffered(eventBus *types.EventBus, q kpubsub.Query) <-chan kpubsub.Message {
	sub, err := eventBus.SubscribeUnbuffered(context.Background(), testSubscriber, q)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, q))
	}
	return sub.Out()
}
