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
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	kpubsub "github.com/kardiachain/go-kardiamain/lib/pubsub"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/staking"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/kardiachain/go-kardiamain/types/evidence"
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
	Height      int64
	Round       int64
	PrivVal     types.PrivValidator
	VotingPower int64
}

var testMinPower int64 = 10

func newValidatorStub(privValidator types.PrivValidator, valIndex int64, round int64) *validatorStub {
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

	partSetHash := hash
	blockPartsHeaders := types.PartSetHeader{Total: uint32(123), Hash: partSetHash}
	privVal := vs.PrivVal

	vote := &types.Vote{
		ValidatorIndex:   uint32(vs.Index),
		ValidatorAddress: privVal.GetAddress(),
		Height:           uint64(vs.Height),
		Round:            uint32(vs.Round),
		Timestamp:        time.Now(),
		Type:             voteType,
		BlockID:          types.BlockID{Hash: partSetHash, PartsHeader: blockPartsHeaders},
	}

	chainID := "kaicon"
	p := vote.ToProto()
	err := privVal.SignVote(chainID, p)
	if err != nil {
		return nil, err
	}

	return vote, nil
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
	cs.Start()
}

// Create proposal block from cs but sign it with vs.
func decideProposal(
	cs *ConsensusState,
	vs *validatorStub,
	height int64,
	round int64,
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
	proposal = types.NewProposal(uint64(height), uint32(round), polRound, propBlockID)
	privVal := vs.PrivVal
	p := proposal.ToProto()
	if err := privVal.SignProposal(chainID, p); err != nil {
		panic(err)
	}

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
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag("test state")

	// var DBInfo storage.DbInfo
	bc, chainConfig, err := GetBlockchain()
	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	if err != nil {
		return nil, nil
	}

	staking, _ := staking.NewSmcStakingnUtil()

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	evPool := evidence.NewPool(kaiDb.DB(), kaiDb.DB())
	bOper := blockchain.NewBlockOperations(logger, bc, txPool, evPool, staking)

	// evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(blockDB, evPool, bOper)

	// Initialization for consensus.
	block := bc.CurrentBlock()

	// var validatorSet *types.ValidatorSet
	validatorSet, privSet := types.RandValidatorSet(nValidators, 10)
	// state, err := cstate.LoadStateFromDBOrGenesisDoc(kaiDb.DB(), config.Genesis)
	state := cstate.LastestBlockState{
		ChainID:                     "kaicon",
		LastBlockHeight:             uint64(block.Height()),
		LastBlockID:                 block.Header().LastBlockID,
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: uint64(0),
	}

	consensusState := NewConsensusState(
		logger,
		configs.DefaultConsensusConfig(),
		state,
		bOper,
		blockExec,
		evPool,
	)

	consensusState.SetPrivValidator(privSet[0])
	// Get State
	vss := make([]*validatorStub, nValidators)

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privSet[i], int64(i), int64(consensusState.Round))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return consensusState, vss
}

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*configs.ChainConfig, common.Hash, error) {
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	return genesis.SetupGenesisBlock(log.New(), db, g, &configs.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
}

func GetBlockchain() (*blockchain.BlockChain, *configs.ChainConfig, error) {
	// Start setting up blockchain
	initValue := g.ToCell(int64(math.Pow10(6)))
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}

	stakingSmcAddress := configs.GetContractAddressAt(7).String()
	var genesisContracts = map[string]string{
		stakingSmcAddress: configs.GenesisContracts[stakingSmcAddress],
	}

	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, kaiDb)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, false)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, nil, err
	}

	return bc, chainConfig, nil
}

func newState(vs *validatorStub, state cstate.LastestBlockState) (*ConsensusState, error) {
	// Create a specific logger for KARDIA service.
	logger := log.New()
	logger.AddTag("test state")

	bc, chainConfig, err := GetBlockchain()
	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	if err != nil {
		return nil, err
	}

	staking, _ := staking.NewSmcStakingnUtil()

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	evPool := evidence.NewPool(kaiDb.DB(), kaiDb.DB())
	bOper := blockchain.NewBlockOperations(logger, bc, txPool, evPool, staking)

	// evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(blockDB, evPool, bOper)

	// Initialization for consensus.
	// block := bc.CurrentBlock()

	consensusState := NewConsensusState(
		logger,
		configs.DefaultConsensusConfig(),
		state,
		bOper,
		blockExec,
		evPool,
	)

	consensusState.SetPrivValidator(vs.PrivVal)

	return consensusState, nil
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

func ensurePrecommit() {
	time.Sleep(500 * time.Millisecond)
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
