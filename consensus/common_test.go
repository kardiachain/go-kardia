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
	"math/big"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	kpubsub "github.com/kardiachain/go-kardia/lib/pubsub"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence"
)

const (
	testSubscriber = "test-client"
)

var (
	ensureTimeout = time.Millisecond * 200
)

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index       int64 // Validator index. NOTE: we don't assume validator set changes.
	Height      uint64
	Round       uint32
	PrivVal     types.PrivValidator
	VotingPower int64
}

var testMinPower int64 = 10

func newValidatorStub(privValidator types.PrivValidator, valIndex int64, round uint32) *validatorStub {
	return &validatorStub{
		Index:       valIndex,
		PrivVal:     privValidator,
		VotingPower: testMinPower,
		Round:       round,
	}
}

func (vs *validatorStub) signVote(
	voteType kproto.SignedMsgType,
	hash common.Hash,
	header types.PartSetHeader) (*types.Vote, error) {
	privVal := vs.PrivVal
	vote := &types.Vote{
		ValidatorIndex:   uint32(vs.Index),
		ValidatorAddress: privVal.GetAddress(),
		Height:           uint64(vs.Height),
		Round:            uint32(vs.Round),
		Timestamp:        time.Now(),
		Type:             voteType,
		BlockID:          types.BlockID{Hash: hash, PartsHeader: header},
	}

	v := vote.ToProto()
	err := privVal.SignVote("kaicon", v)
	vote.Signature = v.Signature

	return vote, err
}

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType kproto.SignedMsgType, hash common.Hash, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

func signVotes(
	voteType kproto.SignedMsgType,
	hash common.Hash,
	header types.PartSetHeader,
	vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, voteType, hash, header)
	}
	return votes
}

func incrementHeight(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Height++
	}
}

func incrementRound(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Round++
	}
}

type ValidatorStubsByPower []*validatorStub

func (vss ValidatorStubsByPower) Len() int {
	return len(vss)
}

func (vss ValidatorStubsByPower) Less(i, j int) bool {
	vssi := (vss[i].PrivVal).GetAddress()
	vssj := (vss[j].PrivVal).GetAddress()

	if vss[i].VotingPower == vss[j].VotingPower {
		return vssi == vssj
	}

	return vss[i].VotingPower > vss[j].VotingPower
}

func (vss ValidatorStubsByPower) Swap(i, j int) {
	it := vss[i]
	vss[i] = vss[j]
	vss[i].Index = int64(i)
	vss[j] = it
	vss[j].Index = int64(j)
}

//-------------------------------------------------------------------------------
// Functions for transitioning the consensus state

func startTestRound(cs *ConsensusState, height uint64, round uint32) {
	cs.enterNewRound(height, round)
	cs.startRoutines(0)
}

// Create proposal block from cs but sign it with vs.
func decideProposal(
	cs *ConsensusState,
	vs *validatorStub,
	height uint64,
	round uint32,
) (proposal *types.Proposal, block *types.Block) {
	cs.mtx.Lock()
	block, blockParts := cs.createProposalBlock()
	validRound := cs.ValidRound
	chainID := cs.state.ChainID
	cs.mtx.Unlock()
	if block == nil {
		panic("Failed to createProposalBlock. Did you forget to add commit for previous block?")
	}

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()}
	proposal = types.NewProposal(height, round, polRound, propBlockID)
	privVal := vs.PrivVal
	p := proposal.ToProto()
	if err := privVal.SignProposal(chainID, p); err != nil {
		panic(err)
	}
	proposal.Signature = p.Signature
	return
}

func addVotes(cs *ConsensusState, votes ...*types.Vote) {
	for _, vote := range votes {
		cs.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(
	to *ConsensusState,
	voteType kproto.SignedMsgType,
	hash common.Hash,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *ConsensusState, round uint32, vs *validatorStub, blockHash common.Hash) {
	prevotes := cs.Votes.Prevotes(round)
	privVal := vs.PrivVal
	address := privVal.GetAddress()
	var vote *types.Vote

	if vote = prevotes.GetByAddress(address); vote == nil {
		t.Log("Failed to find prevote from validator")
	}
}

func validatePrecommit(
	t *testing.T,
	cs *ConsensusState,
	thisRound,
	lockRound uint32,
	vs *validatorStub,
	votedBlockHash,
	lockedBlockHash common.Hash,
) {
	precommits := cs.Votes.Precommits(uint32(thisRound))
	privVal := vs.PrivVal
	address := privVal.GetAddress()
	var vote *types.Vote

	if vote = precommits.GetByAddress(address); vote == nil {
		t.Log("Failed to find precommit from validator")
	}
}

func validateLastPrecommit(t *testing.T, cs *ConsensusState, vs *validatorStub, blockHash common.Hash) {
	votes := cs.LastCommit
	privVal := vs.PrivVal
	address := privVal.GetAddress()
	var vote *types.Vote
	if vote = votes.GetByAddress(address); vote == nil {
		t.Log("Failed to find precommit from validator")
	}
}

func randState(nValidators int) (*ConsensusState, []*validatorStub) {
	// var validatorSet *types.ValidatorSet
	validatorSet, privSet := types.RandValidatorSet(nValidators, 10)
	// state, err := cstate.LoadStateFromDBOrGenesisDoc(kaiDb.DB(), config.Genesis)
	state := cstate.LatestBlockState{
		ChainID:                     "kaicon",
		InitialHeight:               1,
		LastBlockHeight:             0,
		LastBlockID:                 types.NewZeroBlockID(),
		LastBlockTime:               time.Now(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		NextValidators:              validatorSet.CopyIncrementProposerPriority(1),
		LastHeightValidatorsChanged: uint64(0),
	}

	// Get State
	vss := make([]*validatorStub, nValidators)
	cs, _ := newState(privSet[0], state)
	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privSet[i], int64(i), cs.Round)
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
}

func setupGenesis(g *genesis.Genesis, db kaidb.Database) (*configs.ChainConfig, common.Hash, error) {
	return genesis.SetupGenesisBlock(db, g)
}

func GetBlockchain() (*blockchain.BlockChain, *configs.ChainConfig, error) {
	// Start setting up blockchain
	//initValue := g.ToCell(int64(math.Pow10(6)))
	initValue, _ := big.NewInt(0).SetString("10000000000000000", 10)
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}

	configs.AddDefaultContract()

	for address := range genesisAccounts {
		genesisAccounts[address] = initValue
	}

	genesisContracts := make(map[string]string)
	for key, contract := range configs.GetContracts() {
		configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
		if key != configs.StakingContractKey {
			genesisContracts[contract.Address] = contract.ByteCode
		}
	}

	blockDB := memorydb.New()
	genesis := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, blockDB)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(blockDB, nil, nil)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, nil, err
	}

	return bc, chainConfig, nil
}

func newState(vs types.PrivValidator, state cstate.LatestBlockState) (*ConsensusState, error) {
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag("test state")

	bc, chainConfig, err := GetBlockchain()
	blockDB := memorydb.New()
	kaiDb := rawdb.NewStoreDB(blockDB)
	if err != nil {
		return nil, err
	}

	staking, _ := staking.NewSmcStakingUtil()

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	stateStore := cstate.NewStore(kaiDb.DB())
	evPool, _ := evidence.NewPool(stateStore, kaiDb.DB(), bc)
	bOper := blockchain.NewBlockOperations(logger, bc, txPool, evPool, staking)

	// evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(stateStore, logger, evPool, bOper)

	csCfg := configs.TestConsensusConfig()
	// Initialization for consensus.
	// block := bc.CurrentBlock()
	cs := NewConsensusState(
		logger,
		csCfg,
		state,
		bOper,
		blockExec,
		evPool,
	)

	cs.SetPrivValidator(vs)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().New("module", "events"))
	err = eventBus.Start()
	if err != nil {
		panic(err)
	}
	cs.SetEventBus(eventBus)

	return cs, nil
}

func ensurePrevote(voteCh <-chan kpubsub.Message, height uint64, round uint32) {
	ensureVote(voteCh, height, round, kproto.PrevoteType)
}

func ensureVote(voteCh <-chan kpubsub.Message, height uint64, round uint32,
	voteType kproto.SignedMsgType) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewVote event")
	case msg := <-voteCh:
		voteEvent, ok := msg.Data().(types.EventDataVote)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataVote, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		vote := voteEvent.Vote
		if vote.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, vote.Height))
		}
		if vote.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, vote.Round))
		}
		if vote.Type != voteType {
			panic(fmt.Sprintf("expected type %v, got %v", voteType, vote.Type))
		}
	}
}

func ensurePrecommit(voteCh <-chan kpubsub.Message, height uint64, round uint32) {
	ensureVote(voteCh, height, round, kproto.PrecommitType)
}

func ensureNewProposal(proposalCh <-chan kpubsub.Message, height uint64, round uint32) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if proposalEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, proposalEvent.Height))
		}
		if proposalEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, proposalEvent.Round))
		}
	}
}

func ensureNewRound(roundCh <-chan kpubsub.Message, height uint64, round uint32) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewRound event")
	case msg := <-roundCh:
		newRoundEvent, ok := msg.Data().(types.EventDataNewRound)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewRound, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if newRoundEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, newRoundEvent.Height))
		}
		if newRoundEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, newRoundEvent.Round))
		}
	}
}

func ensureNewBlock(blockCh <-chan kpubsub.Message, height uint64) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlock event")
	case msg := <-blockCh:
		blockEvent, ok := msg.Data().(types.EventDataNewBlock)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewBlock, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if blockEvent.Block.Height() != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, blockEvent.Block.Height()))
		}
	}
}

func ensureNewTimeout(timeoutCh <-chan kpubsub.Message, height uint64, round uint32, timeout int64) {
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNewEvent(timeoutCh, height, round, timeoutDuration,
		"Timeout expired while waiting for NewTimeout event")
}

func ensureNewEvent(ch <-chan kpubsub.Message, height uint64, round uint32, timeout time.Duration, errorMessage string) {
	select {
	case <-time.After(timeout):
		panic(errorMessage)
	case msg := <-ch:
		roundStateEvent, ok := msg.Data().(types.EventDataRoundState)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataRoundState, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if roundStateEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, roundStateEvent.Height))
		}
		if roundStateEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, roundStateEvent.Round))
		}
		// TODO: We could check also for a step at this point!
	}
}

func subscribeToVoter(cs *ConsensusState, addr common.Address) <-chan kpubsub.Message {
	votesSub, err := cs.eventBus.SubscribeUnbuffered(context.Background(), testSubscriber, types.EventQueryVote)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, types.EventQueryVote))
	}
	ch := make(chan kpubsub.Message)
	go func() {
		for msg := range votesSub.Out() {
			vote := msg.Data().(types.EventDataVote)
			// we only fire for our own votes
			if addr.Equal(vote.Vote.ValidatorAddress) {
				ch <- msg
			}
		}
	}()
	return ch
}

func ensureNewBlockHeader(blockCh <-chan kpubsub.Message, height uint64, blockHash common.Hash) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlockHeader event")
	case msg := <-blockCh:
		blockHeaderEvent, ok := msg.Data().(types.EventDataNewBlockHeader)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewBlockHeader, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if blockHeaderEvent.Header.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, blockHeaderEvent.Header.Height))
		}
		if !blockHeaderEvent.Header.Hash().Equal(blockHash) {
			panic(fmt.Sprintf("expected header %X, got %X", blockHash, blockHeaderEvent.Header.Hash()))
		}
	}
}

func ensureNewUnlock(unlockCh <-chan kpubsub.Message, height uint64, round uint32) {
	ensureNewEvent(unlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewUnlock event")
}

func ensureProposal(proposalCh <-chan kpubsub.Message, height uint64, round uint32, propID types.BlockID) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if proposalEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, proposalEvent.Height))
		}
		if proposalEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, proposalEvent.Round))
		}
		if !proposalEvent.BlockID.Equal(propID) {
			panic(fmt.Sprintf("Proposed block does not match expected block (%v != %v)", proposalEvent.BlockID, propID))
		}
	}
}

func validatePrevoteAndPrecommit(
	t *testing.T,
	cs *ConsensusState,
	thisRound,
	lockRound uint32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash common.Hash,
) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
}

func ensureNoNewRoundStep(stepCh <-chan kpubsub.Message) {
	ensureNoNewEvent(
		stepCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving NewRoundStep event")
}

func ensureNoNewTimeout(stepCh <-chan kpubsub.Message, timeout int64) {
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNoNewEvent(
		stepCh,
		timeoutDuration,
		"We should be stuck waiting, not receiving NewTimeout event")
}

func ensureNoNewUnlock(unlockCh <-chan kpubsub.Message) {
	ensureNoNewEvent(
		unlockCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving Unlock event")
}

func ensureNewValidBlock(validBlockCh <-chan kpubsub.Message, height uint64, round uint32) {
	ensureNewEvent(validBlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewValidBlock event")
}

func ensurePrecommitTimeout(ch <-chan kpubsub.Message) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for the Precommit to Timeout")
	case <-ch:
	}
}

//-------------------------------------------------------------------------------

func ensureNoNewEvent(ch <-chan kpubsub.Message, timeout time.Duration,
	errorMessage string) {
	select {
	case <-time.After(timeout):
		break
	case <-ch:
		panic(errorMessage)
	}
}
