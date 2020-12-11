package consensus

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"

	"github.com/kardiachain/go-kardia/configs"

	"github.com/kardiachain/go-kardia/mainchain/blockchain"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
)

// WALGenerateNBlocks generates a consensus WAL. It does this by spinning up a
// stripped down version of node (proxy app, event bus, consensus state) with a
// persistent kvstore application and special consensus wal instance
// (byteBufferWAL) and waits until numBlocks are created.
// If the node fails to produce given numBlocks, it returns an error.
func WALGenerateNBlocks(t *testing.T, wr io.Writer, numBlocks int) (err error) {
	logger := log.TestingLogger().New("wal_generator", "wal_generator")
	blockStoreDB := memorydb.New()
	stateDB := blockStoreDB
	stateStore := cstate.NewStore(stateDB)

	genesisObj := &genesis.Genesis{
		ChainID:         "kai",
		Consensus:       configs.DefaultConsensusConfig(),
		ConsensusParams: types.DefaultConsensusParams(),
	}

	_, privValidator := types.RandValidator(true, 1)

	state, err := cstate.MakeGenesisState(genesisObj)
	if err != nil {
		return fmt.Errorf("failed to make genesis state: %w", err)
	}
	stateStore.Save(state)

	blockStore := kvstore.NewStoreDB(blockStoreDB)

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, blockStore, genesisObj, nil)
	if genesisErr != nil {
		return err
	}

	bc, err := blockchain.NewBlockChain(log.New("blockchain"), blockStore, chainConfig, false)

	if err != nil {
		return fmt.Errorf("failed to new blockchain: %w", err)
	}

	blockOperator := blockchain.NewBlockOperations(log.New("blockchain"), bc, nil, nil, nil)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.New("module", "events"))

	if err := eventBus.Start(); err != nil {
		return fmt.Errorf("failed to start event bus: %w", err)
	}

	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	//blockExec := cstate.NewBlockExecutor(stateStore, nil, nil)
	consensusState := NewConsensusState(log.New("consensus state"), configs.DefaultConsensusConfig(), state, blockOperator, nil, nil)
	consensusState.SetEventBus(eventBus)
	consensusState.SetPrivValidator(privValidator)

	// set consensus wal to buffered WAL, which will write all incoming msgs to buffer
	numBlocksWritten := make(chan struct{})
	wal := newByteBufferWAL(logger, NewWALEncoder(wr), int64(numBlocks), numBlocksWritten)
	// see wal.go#103
	if err := wal.Write(EndHeightMessage{0}); err != nil {
		t.Error(err)
	}

	consensusState.wal = wal

	if err := consensusState.Start(); err != nil {
		return fmt.Errorf("failed to start consensus state: %w", err)
	}

	select {
	case <-numBlocksWritten:
		if err := consensusState.Stop(); err != nil {
			t.Error(err)
		}
		return nil
	case <-time.After(1 * time.Minute):
		if err := consensusState.Stop(); err != nil {
			t.Error(err)
		}
		return fmt.Errorf("waited too long for tendermint to produce %d blocks (grep logs for `wal_generator`)", numBlocks)
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
