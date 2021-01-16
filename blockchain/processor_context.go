package blockchain

import (
	"fmt"

	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/types"
)

type processorContext interface {
	applyBlock(blockID types.BlockID, block *types.Block) error
	verifyCommit(chainID string, blockID types.BlockID, height uint64, commit *types.Commit) error
	saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
	kaiState() cstate.LatestBlockState
	setState(cstate.LatestBlockState)
}

type pContext struct {
	store   blockStore
	applier blockApplier
	state   cstate.LatestBlockState
}

func newProcessorContext(st blockStore, ex blockApplier, s cstate.LatestBlockState) *pContext {
	return &pContext{
		store:   st,
		applier: ex,
		state:   s,
	}
}

func (pc *pContext) applyBlock(blockID types.BlockID, block *types.Block) error {
	newState, _, err := pc.applier.ApplyBlock(pc.state, blockID, block)
	pc.state = newState
	return err
}

func (pc pContext) kaiState() cstate.LatestBlockState {
	return pc.state
}

func (pc *pContext) setState(state cstate.LatestBlockState) {
	pc.state = state
}

func (pc pContext) verifyCommit(chainID string, blockID types.BlockID, height uint64, commit *types.Commit) error {
	return pc.state.Validators.VerifyCommitLight(chainID, blockID, height, commit)
}

func (pc *pContext) saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	pc.store.SaveBlock(block, blockParts, seenCommit)
}

type mockPContext struct {
	applicationBL  []uint64
	verificationBL []uint64
	state          cstate.LatestBlockState
}

func newMockProcessorContext(
	state cstate.LatestBlockState,
	verificationBlackList []uint64,
	applicationBlackList []uint64) *mockPContext {
	return &mockPContext{
		applicationBL:  applicationBlackList,
		verificationBL: verificationBlackList,
		state:          state,
	}
}

func (mpc *mockPContext) applyBlock(blockID types.BlockID, block *types.Block) error {
	for _, h := range mpc.applicationBL {
		if h == block.Height() {
			return fmt.Errorf("generic application error")
		}
	}
	mpc.state.LastBlockHeight = block.Height()
	return nil
}

func (mpc *mockPContext) verifyCommit(chainID string, blockID types.BlockID, height uint64, commit *types.Commit) error {
	for _, h := range mpc.verificationBL {
		if h == height {
			return fmt.Errorf("generic verification error")
		}
	}
	return nil
}

func (mpc *mockPContext) saveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {

}

func (mpc *mockPContext) setState(state cstate.LatestBlockState) {
	mpc.state = state
}

func (mpc *mockPContext) kaiState() cstate.LatestBlockState {
	return mpc.state
}
