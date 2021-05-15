package blockchain

import (
	"fmt"
	"math/big"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/behaviour"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/p2p/conn"
	"github.com/kardiachain/go-kardia/lib/service"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	bcproto "github.com/kardiachain/go-kardia/proto/kardiachain/blockchain"
	kaiproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
	kaitime "github.com/kardiachain/go-kardia/types/time"
)

type mockPeer struct {
	service.Service
	id p2p.ID
}

func (mp mockPeer) FlushStop()           {}
func (mp mockPeer) ID() p2p.ID           { return mp.id }
func (mp mockPeer) RemoteIP() net.IP     { return net.IP{} }
func (mp mockPeer) RemoteAddr() net.Addr { return &net.TCPAddr{IP: mp.RemoteIP(), Port: 8800} }

func (mp mockPeer) IsOutbound() bool   { return true }
func (mp mockPeer) IsPersistent() bool { return true }
func (mp mockPeer) CloseConn() error   { return nil }

func (mp mockPeer) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{
		DefaultNodeID: "",
		ListenAddr:    "",
	}
}
func (mp mockPeer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp mockPeer) SocketAddr() *p2p.NetAddress   { return &p2p.NetAddress{} }

func (mp mockPeer) Send(byte, []byte) bool    { return true }
func (mp mockPeer) TrySend(byte, []byte) bool { return true }

func (mp mockPeer) Set(string, interface{}) {}
func (mp mockPeer) Get(string) interface{}  { return struct{}{} }

//nolint:unused
type mockBlockStore struct {
	blocks map[uint64]*types.Block
}

func (ml *mockBlockStore) Height() uint64 {
	return uint64(len(ml.blocks))
}

func (ml *mockBlockStore) LoadBlock(height uint64) *types.Block {
	return ml.blocks[height]
}

func (ml *mockBlockStore) SaveBlock(block *types.Block, part *types.PartSet, commit *types.Commit) {
	ml.blocks[block.Height()] = block
}

type mockBlockApplier struct {
}

// XXX: Add whitelist/blacklist?
func (mba *mockBlockApplier) ApplyBlock(
	state cstate.LastestBlockState, blockID types.BlockID, block *types.Block,
) (cstate.LastestBlockState, uint64, error) {
	state.LastBlockHeight++
	return state, 0, nil
}

type mockSwitchIo struct {
	mtx                 sync.Mutex
	switchedToConsensus bool
	numStatusResponse   int
	numBlockResponse    int
	numNoBlockResponse  int
}

func (sio *mockSwitchIo) sendBlockRequest(peerID p2p.ID, height uint64) error {
	return nil
}

func (sio *mockSwitchIo) sendStatusResponse(base, height uint64, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numStatusResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockToPeer(block *types.Block, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numBlockResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockNotFound(height uint64, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numNoBlockResponse++
	return nil
}

func (sio *mockSwitchIo) trySwitchToConsensus(state cstate.LastestBlockState, skipWAL bool) bool {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.switchedToConsensus = true
	return true
}

func (sio *mockSwitchIo) broadcastStatusRequest() error {
	return nil
}

type testReactorParams struct {
	logger      log.Logger
	genDoc      *genesis.Genesis
	privVals    []types.PrivValidator
	startHeight uint64
	mockA       bool
}

var chainID = "1"

func newTestReactor(p testReactorParams) *BlockchainReactor {
	store, state, _ := newReactorStore(p.genDoc, p.privVals, p.startHeight)
	reporter := behaviour.NewMockReporter()
	logger := log.New()

	var appl blockApplier

	if p.mockA {
		appl = &mockBlockApplier{}
	} else {
		configs.AddDefaultContract()
		configs.AddDefaultStakingContractAddress()
		stakingUtil, err := staking.NewSmcStakingUtil()
		if err != nil {
			fmt.Println(err)
			return nil
		}
		stateDB := memorydb.New()
		kaiDb := kvstore.NewStoreDB(stateDB)
		chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, kaiDb, p.genDoc, stakingUtil)
		if genesisErr != nil {
			fmt.Println(genesisErr)
			return nil
		}
		stateStore := cstate.NewStore(kaiDb.DB())
		bc, err := blockchain.NewBlockChain(logger, kaiDb, chainConfig)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		txPool := tx_pool.NewTxPool(tx_pool.DefaultTxPoolConfig, chainConfig, bc)
		bOper := blockchain.NewBlockOperations(logger, bc, txPool, nil, stakingUtil)
		appl = cstate.NewBlockExecutor(stateStore, p.logger, cstate.EmptyEvidencePool{}, bOper)
		stateStore.Save(state)
	}
	r := newReactor(state, store, reporter, appl, configs.TestFastSyncConfig())
	return r
}

// This test is left here and not deleted to retain the termination cases for
// future improvement
// func TestReactorTerminationScenarios(t *testing.T) {

// 	config := cfg.ResetTestRoot("blockchain_reactor_v2_test")
// 	defer os.RemoveAll(config.RootDir)
// 	genDoc, privVals := randGenesisDoc(chainID, 1, false, 30)
// 	refStore, _, _ := newReactorStore(genDoc, privVals, 20)

// 	params := testReactorParams{
// 		logger:      log.TestingLogger(),
// 		genDoc:      genDoc,
// 		privVals:    privVals,
// 		startHeight: 10,
// 		bufferSize:  100,
// 		mockA:       true,
// 	}

// 	type testEvent struct {
// 		evType string
// 		peer   string
// 		height int64
// 	}

// 	tests := []struct {
// 		name   string
// 		params testReactorParams
// 		msgs   []testEvent
// 	}{
// 		{
// 			name:   "simple termination on max peer height - one peer",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 11},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 12},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P1", height: 13},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "simple termination on max peer height - two peers",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 11},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "termination on max peer height - two peers, noBlock error",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveNB", peer: "P1", height: 11},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "ReceiveB", peer: "P2", height: 11},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "termination on max peer height - two peers, remove one peer",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "RemovePeer", peer: "P1"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "ReceiveB", peer: "P2", height: 11},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		tt := tt
// 		t.Run(tt.name, func(t *testing.T) {
// 			reactor := newTestReactor(params)
// 			reactor.Start()
// 			reactor.reporter = behaviour.NewMockReporter()
// 			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
// 			reactor.io = mockSwitch
// 			// time for go routines to start
// 			time.Sleep(time.Millisecond)

// 			for _, step := range tt.msgs {
// 				switch step.evType {
// 				case "AddPeer":
// 					reactor.scheduler.send(bcAddNewPeer{peerID: p2p.ID(step.peer)})
// 				case "RemovePeer":
// 					reactor.scheduler.send(bcRemovePeer{peerID: p2p.ID(step.peer)})
// 				case "ReceiveS":
// 					reactor.scheduler.send(bcStatusResponse{
// 						peerID: p2p.ID(step.peer),
// 						height: step.height,
// 						time:   time.Now(),
// 					})
// 				case "ReceiveB":
// 					reactor.scheduler.send(bcBlockResponse{
// 						peerID: p2p.ID(step.peer),
// 						block:  refStore.LoadBlock(step.height),
// 						size:   10,
// 						time:   time.Now(),
// 					})
// 				case "ReceiveNB":
// 					reactor.scheduler.send(bcNoBlockResponse{
// 						peerID: p2p.ID(step.peer),
// 						height: step.height,
// 						time:   time.Now(),
// 					})
// 				case "BlockReq":
// 					reactor.scheduler.send(rTrySchedule{time: time.Now()})
// 				case "Process":
// 					reactor.processor.send(rProcessBlock{})
// 				}
// 				// give time for messages to propagate between routines
// 				time.Sleep(time.Millisecond)
// 			}

// 			// time for processor to finish and reactor to switch to consensus
// 			time.Sleep(20 * time.Millisecond)
// 			assert.True(t, mockSwitch.hasSwitchedToConsensus())
// 			reactor.Stop()
// 		})
// 	}
// }

func TestReactorHelperMode(t *testing.T) {
	var (
		channelID = byte(0x40)
	)

	genDoc, privVals := randGenesisDoc(chainID, 1, false, 30)

	params := testReactorParams{
		logger:      log.TestingLogger(),
		genDoc:      genDoc,
		privVals:    privVals,
		startHeight: 20,
		mockA:       true,
	}

	type testEvent struct {
		peer  string
		event interface{}
	}

	tests := []struct {
		name   string
		params testReactorParams
		msgs   []testEvent
	}{
		{
			name:   "status request",
			params: params,
			msgs: []testEvent{
				{"P1", bcproto.StatusRequest{}},
				{"P1", bcproto.BlockRequest{Height: 13}},
				{"P1", bcproto.BlockRequest{Height: 20}},
				{"P1", bcproto.BlockRequest{Height: 22}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			reactor := newTestReactor(params)
			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
			reactor.io = mockSwitch
			err := reactor.Start()
			require.NoError(t, err)

			for i := 0; i < len(tt.msgs); i++ {
				step := tt.msgs[i]
				switch ev := step.event.(type) {
				case bcproto.StatusRequest:
					old := mockSwitch.numStatusResponse
					msg, err := EncodeMsg(&ev)
					assert.NoError(t, err)
					reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, msg)
					assert.Equal(t, old+1, mockSwitch.numStatusResponse)
				case bcproto.BlockRequest:
					if ev.Height > params.startHeight {
						old := mockSwitch.numNoBlockResponse
						msg, err := EncodeMsg(&ev)
						assert.NoError(t, err)
						reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, msg)
						assert.Equal(t, old+1, mockSwitch.numNoBlockResponse)
					} else {
						old := mockSwitch.numBlockResponse
						msg, err := EncodeMsg(&ev)
						assert.NoError(t, err)
						assert.NoError(t, err)
						reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, msg)
						assert.Equal(t, old+1, mockSwitch.numBlockResponse)
					}
				}
			}
			err = reactor.Stop()
			require.NoError(t, err)
		})
	}
}

func TestReactorSetSwitchNil(t *testing.T) {
	genDoc, privVals := randGenesisDoc(chainID, 1, false, 30)

	reactor := newTestReactor(testReactorParams{
		logger:   log.TestingLogger(),
		genDoc:   genDoc,
		privVals: privVals,
	})
	reactor.SetSwitch(nil)

	assert.Nil(t, reactor.Switch)
	assert.Nil(t, reactor.io)
}

//----------------------------------------------
// utility funcs

func makeTxs(height uint64) (txs []*types.Transaction) {
	for i := 0; i < 10; i++ {
		txs = append(txs, TestTx)
	}
	return txs
}

func makeBlock(height uint64, state cstate.LastestBlockState, lastCommit *types.Commit) *types.Block {
	block := types.NewBlock(&types.Header{
		Height:             height,
		ValidatorsHash:     state.Validators.Hash(),
		NextValidatorsHash: state.NextValidators.Hash(),
		ProposerAddress:    state.Validators.Validators[0].Address,
	}, makeTxs(height), lastCommit, nil)
	return block
}

//func makeBlock(height uint64, state cstate.LastestBlockState, lastCommit *types.Commit) *types.Block {
//	block := types.NewBlock(&types.Header{
//		Height: height,
//	}, makeTxs(height), lastCommit, nil)
//	return block
//}

func randGenesisDoc(chainID string, numValidators int, randPower bool, minPower int64) (
	*genesis.Genesis, []types.PrivValidator) {
	validators := make([]*genesis.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)
	alloc := make(map[common.Address]genesis.GenesisAccount)
	minStake := "12500000000000000000000000"
	genesisBalance, _ := new(big.Int).SetString("500000000000000000000000000", 10)
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = &genesis.GenesisValidator{
			Address:          val.Address.String(),
			StartWithGenesis: true,
			SelfDelegate:     minStake,
			Name:             "test",
			CommissionRate:   "5",
			MaxRate:          "20",
			MaxChangeRate:    "5",
		}
		privValidators[i] = privVal
		alloc[val.Address] = genesis.GenesisAccount{
			Balance: genesisBalance,
		}
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))
	g := &genesis.Genesis{
		Timestamp:       kaitime.Now(),
		ChainID:         chainID,
		Validators:      validators,
		ConsensusParams: configs.TestConsensusParams(),
		Config:          configs.TestChainConfig,
		Alloc:           alloc,
	}
	return g, privValidators
}

// Why are we importing the entire blockExecutor dependency graph here
// when we have the facilities to
func newReactorStore(
	genDoc *genesis.Genesis,
	privVals []types.PrivValidator,
	maxBlockHeight uint64) (blockStore, cstate.LastestBlockState, *cstate.BlockExecutor) {
	if len(privVals) != 1 {
		panic("only support one validator")
	}
	logger := log.New()
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
	stakingUtil, err := staking.NewSmcStakingUtil()
	if err != nil {
		fmt.Println(err)
		return nil, cstate.LastestBlockState{}, nil
	}
	stateDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(stateDB)
	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, kaiDb, genDoc, stakingUtil)
	if genesisErr != nil {
		fmt.Println(genesisErr)
		return nil, cstate.LastestBlockState{}, nil
	}
	stateStore := cstate.NewStore(kaiDb.DB())
	bc, err := blockchain.NewBlockChain(logger, kaiDb, chainConfig)
	if err != nil {
		fmt.Println(err)
		return nil, cstate.LastestBlockState{}, nil
	}
	txPool := tx_pool.NewTxPool(tx_pool.DefaultTxPoolConfig, chainConfig, bc)
	bOper := blockchain.NewBlockOperations(logger, bc, txPool, nil, stakingUtil)

	state, err := stateStore.LoadStateFromDBOrGenesisDoc(genDoc)
	if err != nil {
		panic(fmt.Errorf("error constructing state from genesis file: %w", err))
	}
	eventBus, err := createAndStartEventBus(logger)
	if err != nil {
		return nil, cstate.LastestBlockState{}, nil
	}
	blockExec := cstate.NewBlockExecutor(stateStore, logger, cstate.EmptyEvidencePool{}, bOper)
	blockExec.SetEventBus(eventBus)
	stateStore.Save(state)

	// add blocks in
	for blockHeight := uint64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)
		if blockHeight > 1 {
			lastBlockMeta := bOper.LoadBlockMeta(blockHeight - 1)
			lastBlock := bOper.LoadBlock(blockHeight - 1)
			vote, err := MakeVote(
				lastBlock.Header().Height,
				lastBlockMeta.BlockID,
				state.Validators,
				privVals[0],
				chainID,
				time.Now(),
			)
			if err != nil {
				panic(err)
			}
			lastCommit = types.NewCommit(vote.Height, vote.Round,
				lastBlockMeta.BlockID, []types.CommitSig{vote.CommitSig()})
		}

		thisBlock := makeBlock(blockHeight, state, lastCommit)

		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartsHeader: thisParts.Header()}

		state, _, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		if err != nil {
			fmt.Printf("%+v\n%+v\n%+v\n%+v\n\n", lastCommit, state, blockID, thisBlock)
			// TODO: need to fix unit test for successfully apply block here
			//panic(fmt.Errorf("error apply block: %w", err))
		}

		bOper.SaveBlock(thisBlock, thisParts, lastCommit)
	}
	return bOper, state, blockExec
}

func MakeVote(
	height uint64,
	blockID types.BlockID,
	valSet *types.ValidatorSet,
	privVal types.PrivValidator,
	chainID string,
	now time.Time,
) (*types.Vote, error) {
	addr := privVal.GetAddress()
	idx, _ := valSet.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   uint32(idx),
		Height:           height,
		Round:            0,
		Timestamp:        now,
		Type:             kaiproto.PrecommitType,
		BlockID:          blockID,
	}
	v := vote.ToProto()

	if err := privVal.SignVote(chainID, v); err != nil {
		return nil, err
	}

	vote.Signature = v.Signature

	return vote, nil
}

func createAndStartEventBus(logger log.Logger) (*types.EventBus, error) {
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.New("module", "events"))
	if err := eventBus.Start(); err != nil {
		return nil, err
	}
	return eventBus, nil
}
