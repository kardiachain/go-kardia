package consensus

import (
	"crypto/ecdsa"
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
	"github.com/kardiachain/go-kardiamain/lib/p2p/enode"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/types/evidence"

	"github.com/kardiachain/go-kardiamain/types"
)

const (
	testSubscriber = "test-client"
)

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index         int64 // Validator index. NOTE: we don't assume validator set changes.
	Height        int64
	Round         int64
	PrivValidator *ecdsa.PrivateKey
	VotingPower   int64
}

var testMinPower int64 = 10

func newValidatorStub(privValidator *ecdsa.PrivateKey, valIndex int64) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
	}
}

func (vs *validatorStub) signVote(
	voteType byte,
	hash common.Hash,
	header types.PartSetHeader) (*types.Vote, error) {

	partSetHash := hash
	blockPartsHeaders := types.PartSetHeader{Total: uint32(123), Hash: partSetHash}
	privVal := types.NewPrivValidator(vs.PrivValidator)

	vote := &types.Vote{
		ValidatorIndex:   common.NewBigInt64(vs.Index),
		ValidatorAddress: privVal.GetAddress(),
		Height:           common.NewBigInt64(vs.Height),
		Round:            common.NewBigInt64(vs.Round),
		Timestamp:        big.NewInt(time.Now().Unix()),
		Type:             voteType,
		BlockID:          types.BlockID{Hash: partSetHash, PartsHeader: blockPartsHeaders},
	}

	chainID := "kaicon"

	err := privVal.SignVote(chainID, vote)
	if err != nil {
		panic(fmt.Sprintf("Error signing vote: %v", err))
		return nil, err
	}

	return vote, nil
}

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType byte, hash common.Hash, header types.PartSetHeader) *types.Vote {
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
	vssi := types.NewPrivValidator(vss[i].PrivValidator).GetAddress()
	vssj := types.NewPrivValidator(vss[j].PrivValidator).GetAddress()

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

func startTestRound(cs *ConsensusState, height *common.BigInt, round *common.BigInt) {
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
	proposal = types.NewProposal(common.NewBigInt64(height), common.NewBigInt64(round), polRound, propBlockID)
	privVal := types.NewPrivValidator(vs.PrivValidator)

	if err := privVal.SignProposal(chainID, proposal); err != nil {
		panic(err)
	}

	return
}

func addVotes(cs *ConsensusState, votes ...*types.Vote) {
	for i, vote := range votes {
		var peerID enode.ID
		peer := common.U256Bytes(big.NewInt(int64(i + 1)))
		copy(peerID[:], peer)
		cs.AddVote(vote, peerID)
	}
}

func signAddVotes(
	to *ConsensusState,
	voteType byte,
	hash common.Hash,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, vs *validatorStub, blockHash common.Hash) {
	prevotes := cs.Votes.Prevotes(round)
	privVal := types.NewPrivValidator(vs.PrivValidator)
	address := privVal.GetAddress()
	var vote *types.Vote

	time.Sleep(time.Second * 10)
	if vote = prevotes.GetByAddress(address); vote == nil {
		t.Log("Failed to find prevote from validator")
	}
}

func validatePrecommit(
	t *testing.T,
	cs *ConsensusState,
	thisRound,
	lockRound int,
	vs *validatorStub,
	votedBlockHash,
	lockedBlockHash common.Hash,
) {
	precommits := cs.Votes.Precommits(int(thisRound))
	privVal := types.NewPrivValidator(vs.PrivValidator)
	address := privVal.GetAddress()
	var vote *types.Vote

	if vote = precommits.GetByAddress(address); vote == nil {
		t.Log("Failed to find precommit from validator")
	}
}

func validateLastPrecommit(t *testing.T, cs *ConsensusState, vs *validatorStub, blockHash common.Hash) {
	votes := cs.LastCommit
	privVal := types.NewPrivValidator(vs.PrivValidator)
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

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	evPool := evidence.NewPool(kaiDb.DB(), kaiDb.DB())
	// evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(evPool)

	// Initialization for consensus.
	block := bc.CurrentBlock()

	// var validatorSet *types.ValidatorSet
	validatorSet, privSet := types.RandValidatorSet(nValidators, 10)
	// state, err := cstate.LoadStateFromDBOrGenesisDoc(kaiDb.DB(), config.Genesis)
	state := cstate.LastestBlockState{
		ChainID:                     "kaicon",
		LastBlockHeight:             common.NewBigUint64(block.Height()),
		LastBlockID:                 block.Header().LastBlockID,
		LastBlockTime:               block.Time().Uint64(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		LastHeightValidatorsChanged: common.NewBigInt32(-1),
	}

	consensusState := NewConsensusState(
		logger,
		configs.DefaultConsensusConfig(),
		state,
		blockchain.NewBlockOperations(logger, bc, txPool, evPool),
		blockExec,
		evPool,
	)

	consensusState.SetPrivValidator(types.NewPrivValidator(privSet[0]))
	// Get State
	vss := make([]*validatorStub, nValidators)

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privSet[i], int64(i))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return consensusState, vss
}

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*types.ChainConfig, common.Hash, error) {
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	return genesis.SetupGenesisBlock(log.New(), db, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
}

func GetBlockchain() (*blockchain.BlockChain, *types.ChainConfig, error) {
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

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	evPool := evidence.NewPool(kaiDb.DB(), kaiDb.DB())
	// evReactor := evidence.NewReactor(evPool)
	blockExec := cstate.NewBlockExecutor(evPool)

	// Initialization for consensus.
	// block := bc.CurrentBlock()

	consensusState := NewConsensusState(
		logger,
		configs.DefaultConsensusConfig(),
		state,
		blockchain.NewBlockOperations(logger, bc, txPool, evPool),
		blockExec,
		evPool,
	)

	consensusState.SetPrivValidator(types.NewPrivValidator(vs.PrivValidator))

	return consensusState, nil
}

func ensurePrevote() {
	time.Sleep(time.Second * 30)
}

func ensurePrecommit() {
	time.Sleep(time.Second * 30)
}
