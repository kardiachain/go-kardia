package consensus

import (
	"fmt"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/types"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
)

// WALGenerateNBlocks generates a consensus WAL. It does this by spinning up a
// stripped down version of node (proxy app, event bus, consensus state) with a
// persistent kvstore application and special consensus wal instance
// (byteBufferWAL) and waits until numBlocks are created.
// If the node fails to produce given numBlocks, it returns an error.
func WALGenerateNBlocks(t *testing.T, wr io.Writer, numBlocks int) (err error) {
	logger := log.New("wal_generator")

	db := memorydb.New()
	storeDB := kvstore.NewStoreDB(db)
	stateStore := cstate.NewStore(db)
	initValue, _ := big.NewInt(0).SetString("10000000000000000", 10)
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}

	configs.AddDefaultContract()

	for address, _ := range genesisAccounts {
		genesisAccounts[address] = initValue
	}

	genesisContracts := make(map[string]string)
	for key, contract := range configs.GetContracts() {
		configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
		if key != configs.StakingContractKey {
			genesisContracts[contract.Address] = contract.ByteCode
		}
	}

	validatorSet, privSet := types.RandValidatorSet(1, 1000000)
	state := cstate.LastestBlockState{
		ChainID:                     "kaicon",
		LastBlockHeight:             0,
		LastBlockID:                 types.NewZeroBlockID(),
		LastBlockTime:               time.Now(),
		Validators:                  validatorSet,
		LastValidators:              validatorSet,
		NextValidators:              validatorSet.CopyIncrementProposerPriority(1),
		LastHeightValidatorsChanged: uint64(0),
	}

	stateStore.Save(state)

	gs := &genesis.Genesis{
		Config:          configs.TestnetChainConfig,
		GasLimit:        configs.BlockGasLimit,
		Alloc:           genesis.GenesisAlloc{},
		ConsensusParams: configs.DefaultConsensusParams(),
		Consensus:       configs.TestConsensusConfig(),
	}

	stakingUtil, _ := staking.NewSmcStakingUtil()
	chainConfig, _, err := genesis.SetupGenesisBlock(log.New("genesis"), storeDB, gs, stakingUtil)
	if err != nil {
		return err
	}

	bc, err := blockchain.NewBlockChain(log.New("blockchain"), storeDB, chainConfig, false)
	if err != nil {
		return err
	}
	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)
	evPool := cstate.EmptyEvidencePool{}
	bOper := blockchain.NewBlockOperations(log.New("block_operations"), bc, txPool, evPool, stakingUtil)
	blockExec := cstate.NewBlockExecutor(stateStore, evPool, bOper)

	csCfg := configs.TestConsensusConfig()
	cs := NewConsensusState(
		logger, csCfg, state, bOper, blockExec, evPool,
	)

	cs.SetPrivValidator(privSet[0])

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().New("module", "events"))

	err = eventBus.Start()
	if err != nil {
		return err
	}
	cs.SetEventBus(eventBus)

	// set consensus wal to buffered WAL, which will write all incoming msgs to buffer
	numBlocksWritten := make(chan struct{})
	wal := newByteBufferWAL(logger, NewWALEncoder(wr), int64(numBlocks), numBlocksWritten)
	// see wal.go#103
	if err := wal.Write(EndHeightMessage{0}); err != nil {
		t.Error(err)
	}

	cs.wal = wal

	if err := cs.Start(); err != nil {
		return fmt.Errorf("failed to start consensus state: %w", err)
	}

	select {
	case <-numBlocksWritten:

		if err := cs.Stop(); err != nil {
			t.Error(err)
		}
		return nil
	case <-time.After(1 * time.Minute):

		if err := cs.Stop(); err != nil {
			t.Error(err)
		}
		return fmt.Errorf("waited too long for kardiachain to produce %d blocks (grep logs for `wal_generator`)", numBlocks)
	}
}

var fixedTime, _ = time.Parse(time.RFC3339, "2017-01-02T15:04:05Z")

// byteBufferWAL is a WAL which writes all msgs to a byte buffer. Writing stops
// when the heightToStop is reached. Client will be notified via
// signalWhenStopsTo channel.
type byteBufferWAL struct {
	enc               *WALEncoder
	stopped           bool
	heightToStop      int64
	signalWhenStopsTo chan<- struct{}

	logger log.Logger
}

func newByteBufferWAL(logger log.Logger, enc *WALEncoder, nBlocks int64, signalStop chan<- struct{}) *byteBufferWAL {
	return &byteBufferWAL{
		enc:               enc,
		heightToStop:      nBlocks,
		signalWhenStopsTo: signalStop,
		logger:            logger,
	}
}

// Save writes message to the internal buffer except when heightToStop is
// reached, in which case it will signal the caller via signalWhenStopsTo and
// skip writing.
func (w *byteBufferWAL) Write(m WALMessage) error {

	if w.stopped {
		w.logger.Debug("WAL already stopped. Not writing message", "msg", m)
		return nil
	}

	if endMsg, ok := m.(EndHeightMessage); ok {
		w.logger.Debug("WAL write end height message", "height", endMsg.Height, "stopHeight", w.heightToStop)
		if endMsg.Height == w.heightToStop {
			w.logger.Debug("Stopping WAL at height", "height", endMsg.Height)
			w.signalWhenStopsTo <- struct{}{}
			w.stopped = true
			return nil
		}
	}

	w.logger.Debug("WAL Write Message", "msg", m)
	err := w.enc.Encode(&TimedWALMessage{fixedTime, m})
	if err != nil {
		panic(fmt.Sprintf("failed to encode the msg %v", m))
	}

	return nil
}

func (w *byteBufferWAL) WriteSync(m WALMessage) error {
	return w.Write(m)
}

func (w *byteBufferWAL) FlushAndSync() error { return nil }

func (w *byteBufferWAL) SearchForEndHeight(
	height int64,
	options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return nil, false, nil
}

func (w *byteBufferWAL) Start() error { return nil }
func (w *byteBufferWAL) Stop() error  { return nil }
func (w *byteBufferWAL) Wait()        {}
