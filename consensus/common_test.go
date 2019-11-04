package consensus

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"

	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index  *cmn.BigInt // Validator index. NOTE: we don't assume validator set changes.
	Height *cmn.BigInt
	Round  *cmn.BigInt
	types.PrivValidator
}

var testMinPower int64 = 10

func NewValidatorStub(privValidator types.PrivValidator, valIndex *cmn.BigInt) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
	}
}

func (vs *validatorStub) signVote(
	voteType byte,
	hash common.Hash,
	header types.PartSetHeader) (*types.Vote, error) {
	addr := vs.PrivValidator.GetAddress()
	vote := &types.Vote{
		ValidatorIndex:   vs.Index,
		ValidatorAddress: addr,
		Height:           vs.Height,
		Round:            vs.Round,
		Timestamp:        big.NewInt(time.Now().Unix()),
		Type:             voteType,
		BlockID:          types.BlockID{Hash: hash, PartsHeader: header},
	}
	err := vs.PrivValidator.SignVote("kai", vote)
	return vote, err
}

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType byte, hash cmn.Hash, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

func signVotes(
	voteType byte,
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
		vs.Height.Add(1)
	}
}

func incrementRound(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Round.Add(1)
	}
}

type ValidatorStubsByAddress []*validatorStub

func (vss ValidatorStubsByAddress) Len() int {
	return len(vss)
}

func (vss ValidatorStubsByAddress) Less(i, j int) bool {
	return vss[i].GetAddress().Equal(vss[j].GetAddress())
}

func (vss ValidatorStubsByAddress) Swap(i, j int) {
	it := vss[i]
	vss[i] = vss[j]
	vss[i].Index = cmn.NewBigInt32(i)
	vss[j] = it
	vss[j].Index = cmn.NewBigInt32(j)
}

//-------------------------------------------------------------------------------
// Functions for transitioning the consensus state

func startTestRound(cs *ConsensusState, height *cmn.BigInt, round *cmn.BigInt) {
	cs.enterNewRound(height, round)
	//cs.startRoutines(0)
}

// Create proposal block from cs1 but sign it with vs.
func decideProposal(
	cs1 *ConsensusState,
	vs *validatorStub,
	height *cmn.BigInt,
	round *cmn.BigInt) (proposal *types.Proposal, block *types.Block) {
	cs1.mtx.Lock()
	block, blockParts := cs1.createProposalBlock()
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID
	cs1.mtx.Unlock()
	if block == nil {
		panic("Failed to createProposalBlock. Did you forget to add commit for previous block?")
	}

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()}
	proposal = types.NewProposal(height, round, polRound, propBlockID)
	if err := vs.SignProposal(chainID, proposal); err != nil {
		panic(err)
	}
	return
}

func addVotes(to *ConsensusState, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(
	to *ConsensusState,
	voteType byte,
	hash common.Hash,
	header types.PartSetHeader,
	vss ...*validatorStub) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *ConsensusState, round *cmn.BigInt, privVal *validatorStub, blockHash cmn.Hash) {
	prevotes := cs.Votes.Prevotes(round.Int32())
	address := privVal.GetAddress()
	var vote *types.Vote
	if vote = prevotes.GetByAddress(address); vote == nil {
		panic("Failed to find prevote from validator")
	}
	if blockHash.IsZero() {
		if !vote.BlockID.Hash.IsZero() {
			panic(fmt.Sprintf("Expected prevote to be for nil, got %X", vote.BlockID.Hash))
		}
	} else {
		if !vote.BlockID.Hash.Equal(blockHash) {
			panic(fmt.Sprintf("Expected prevote to be for %X, got %X", blockHash, vote.BlockID.Hash))
		}
	}
}

func validateLastPrecommit(t *testing.T, cs *ConsensusState, privVal *validatorStub, blockHash common.Hash) {
	votes := cs.LastCommit
	address := privVal.GetAddress()
	var vote *types.Vote
	if vote = votes.GetByAddress(address); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !vote.BlockID.Hash.Equal(blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash))
	}
}

func validatePrecommit(
	t *testing.T,
	cs *ConsensusState,
	thisRound,
	lockRound *cmn.BigInt,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash common.Hash) {
	precommits := cs.Votes.Precommits(thisRound.Int32())
	address := privVal.GetAddress()
	var vote *types.Vote
	if vote = precommits.GetByAddress(address); vote == nil {
		panic("Failed to find precommit from validator")
	}

	if votedBlockHash.IsZero() {
		if vote.BlockID.Hash.IsZero() {
			panic("Expected precommit to be for nil")
		}
	} else {
		if !vote.BlockID.Hash.Equal(votedBlockHash) {
			panic("Expected precommit to be for proposal block")
		}
	}

	if lockedBlockHash.IsZero() {
		if !cs.LockedRound.Equals(lockRound) || cs.LockedBlock != nil {
			panic(fmt.Sprintf(
				"Expected to be locked on nil at round %d. Got locked at round %d with block %v",
				lockRound.Int32(),
				cs.LockedRound.Int32(),
				cs.LockedBlock))
		}
	} else {
		if cs.LockedRound != lockRound || !cs.LockedBlock.Hash().Equal(lockedBlockHash) {
			panic(fmt.Sprintf(
				"Expected block to be locked on round %d, got %d. Got locked block %X, expected %X",
				lockRound.Int32(),
				cs.LockedRound.Int32(),
				cs.LockedBlock.Hash(),
				lockedBlockHash))
		}
	}

}

func validatePrevoteAndPrecommit(
	t *testing.T,
	cs *ConsensusState,
	thisRound,
	lockRound *cmn.BigInt,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash common.Hash) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
}

func randGenesis(numValidators int, randPower bool, minPower int64) (*genesis.Genesis, []types.PrivValidator) {
	genDoc, privValidators := randGenesisDoc(numValidators, randPower, minPower)
	return genDoc, privValidators
}

func randConsensusState(nValidators int) (*ConsensusState, []*validatorStub) {
	// Get State
	genesis, privVals := randGenesis(nValidators, false, 10)

	baseAccount := &types.BaseAccount{
		Address:    privVals[0].GetAddress(),
		PrivateKey: *privVals[0].GetPrivKey(),
	}

	vss := make([]*validatorStub, nValidators)

	cs := newConsensusStateWithConfig(genesis.Validators, &Config{
		Genesis:     genesis,
		BaseAccount: baseAccount,
	})

	for i := 0; i < nValidators; i++ {
		vss[i] = NewValidatorStub(privVals[i], cmn.NewBigInt32(i))
	}

	return cs, vss
}

func newConsensusStateWithConfig(validators []*types.Validator, conf *Config) *ConsensusState {
	logger := log.New()
	blockDB := memorydb.New()
	storeDB := kvstore.NewStoreDB(blockDB)
	chainConfig := &types.ChainConfig{
		BaseAccount: conf.BaseAccount,
		Kaicon: &types.KaiconConfig{
			Period: 15,
			Epoch:  10,
		},
	}

	conf.Genesis.Config = chainConfig

	chainConfig, _, err := genesis.SetupGenesisBlock(logger, storeDB, conf.Genesis, conf.BaseAccount)

	if err != nil {
		panic(err)
	}

	nBlockchain, err := blockchain.NewBlockChain(logger, storeDB, chainConfig, false)

	if err != nil {
		panic(err)
	}

	txPool := tx_pool.NewTxPool(logger, conf.TxPool, chainConfig, nBlockchain)
	blockStore := blockchain.NewBlockOperations(logger, nBlockchain, txPool)

	block := nBlockchain.CurrentBlock()
	blockID := types.BlockID{
		Hash:        block.Hash(),
		PartsHeader: block.MakePartSet(types.BlockPartSizeBytes).Header(),
	}

	validatorSet := types.NewValidatorSet(validators, 0 /*start height*/, 100000000000 /*end height*/)

	state := state.LastestBlockState{
		ChainID:                     "kaicon", // TODO(thientn): considers merging this with protocolmanger.ChainID
		LastBlockHeight:             cmn.NewBigUint64(block.Height()),
		LastBlockID:                 blockID,
		LastBlockTime:               block.Time(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: cmn.NewBigInt32(-1),
	}

	return NewConsensusState(logger, configs.DefaultConsensusConfig(), state, blockStore)

}

type Config struct {
	// Protocol options
	NetworkId uint64 // Network

	ChainId uint64

	// The genesis block, which is inserted if the database is empty.
	// If nil, the Kardia main net block is used.
	Genesis *genesis.Genesis `toml:",omitempty"`

	// Transaction pool options
	TxPool tx_pool.TxPoolConfig

	// DbInfo stores configuration information to setup database
	DBInfo storage.DbInfo

	// acceptTxs accept tx sync processes
	AcceptTxs uint32

	// IsZeroFee is true then sender will be refunded all gas spent for a transaction
	IsZeroFee bool

	// isPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	// ServiceName is used to display as log's prefix
	ServiceName string

	// BaseAccount defines account which is used to execute internal smart contracts
	BaseAccount *types.BaseAccount
}
