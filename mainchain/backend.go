package kai

import (
	"fmt"

	bcReactor "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/internal/kaiapi"
	"github.com/kardiachain/go-kardia/rpc"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kai/accounts"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kai/state/pruner"
	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/filters"
	"github.com/kardiachain/go-kardia/mainchain/oracles"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	"github.com/kardiachain/go-kardia/mainchain/tracers"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/types/evidence"
)

type Kardiachain struct {
	config      *Config
	chainConfig *configs.ChainConfig
	nodeConfig  *node.Config

	// Handlers
	txPool     *tx_pool.TxPool
	blockExec  *cstate.BlockExecutor // TODO: remove this after finish merging block executor, block operations, blockchain
	blockchain *blockchain.BlockChain
	csManager  *consensus.ConsensusManager
	txpoolR    *tx_pool.Reactor
	evR        *evidence.Reactor
	bcR        p2p.Reactor // for fast-syncing

	// DB interfaces
	chainDb kaidb.Database // Block chain database

	eventBus  *types.EventBus
	staking   *staking.StakingSmcUtil
	validator *staking.ValidatorSmcUtil

	bloomRequests     chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer      *BloomIndexer                  // Bloom indexer operating during block imports
	closeBloomHandler chan struct{}

	APIBackend *KaiAPIBackend

	accMan *accounts.Manager

	sw *p2p.Switch // p2p connections

	// Channel for shutting down the service
	shutdownChan chan bool
}

// New creates a new Ethereum object (including the
// initialisation of the common Ethereum object)
func New(stack *node.Node, config *Config) (*Kardiachain, error) {
	logger := log.New()

	if config.NoPruning && config.TrieDirtyCache > 0 {
		if config.SnapshotCache > 0 {
			config.TrieCleanCache += config.TrieDirtyCache * 3 / 5
			config.SnapshotCache += config.TrieDirtyCache * 2 / 5
		} else {
			config.TrieCleanCache += config.TrieDirtyCache
		}
		config.TrieDirtyCache = 0
	}
	log.Info("Allocated trie memory caches", "clean", common.StorageSize(config.TrieCleanCache)*1024*1024, "dirty", common.StorageSize(config.TrieDirtyCache)*1024*1024)

	storeDb, err := stack.OpenDatabase("chaindata", 16, 32, "chaindata")
	if err != nil {
		return nil, err
	}
	chainDb := storeDb.DB()
	if err := pruner.RecoverPruning(stack.ResolvePath(""), chainDb, stack.ResolvePath(config.TrieCleanCacheJournal)); err != nil {
		log.Error("Failed to recover state", "error", err)
	}

	// EventBus and IndexerService must be started before the handshake because
	// we might need to index the txs of the replayed block as this might not have happened
	// when the node stopped last time (i.e. the node stopped after it saved the block
	// but before it indexed the txs, or, endblocker panicked)
	eventBus, err := createAndStartEventBus(logger)
	if err != nil {
		return nil, err
	}

	stakingUtil, err := staking.NewSmcStakingUtil()
	if err != nil {
		return nil, err
	}
	validator, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return nil, err
	}

	kai := &Kardiachain{
		config:       config,
		chainConfig:  config.Genesis.Config,
		nodeConfig:   stack.Config(),
		sw:           stack.P2PSwitch(),
		chainDb:      chainDb,
		eventBus:     eventBus,
		staking:      stakingUtil,
		validator:    validator,
		bloomIndexer: NewBloomIndexer(chainDb, configs.BloomBitsBlocksClient, configs.HelperTrieConfirmations),
		shutdownChan: make(chan bool),
	}

	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	log.Info("Initialising Kardiachain protocol", "network", config.NetworkId, "dbversion", dbVer)

	cacheConfig := &blockchain.CacheConfig{
		TrieCleanLimit:      config.TrieCleanCache,
		TrieCleanJournal:    stack.ResolvePath(config.TrieCleanCacheJournal),
		TrieCleanRejournal:  config.TrieCleanCacheRejournal,
		TrieCleanNoPrefetch: config.NoPrefetch,
		TrieDirtyLimit:      config.TrieDirtyCache,
		TrieDirtyDisabled:   config.NoPruning,
		TrieTimeLimit:       config.TrieTimeout,
		SnapshotLimit:       config.SnapshotCache,
		Preimages:           config.Preimages,
	}

	// Create a new blockchain to attach to this Kardia object
	kai.blockchain, err = blockchain.NewBlockChain(chainDb, cacheConfig, config.Genesis)
	if err != nil {
		return nil, err
	}
	// TODO: enable this
	// kai.bloomIndexer.Start(kai.blockchain)

	kai.APIBackend = &KaiAPIBackend{kai, nil}

	stateDB := cstate.NewStore(chainDb)
	evPool, err := evidence.NewPool(stateDB, chainDb, kai.blockchain)
	if err != nil {
		return nil, err
	}

	// Initialize the blacklist before starting node
	err = tx_pool.UpdateBlacklist(tx_pool.InitialBlacklistRequestTimeout)
	if err != nil {
		return nil, err
	}
	// log.Info("Updated blacklisted addresses", "addresses", tx_pool.StringifyBlacklist())
	kai.txPool = tx_pool.NewTxPool(config.TxPool, kai.chainConfig, kai.blockchain)
	kai.txpoolR = tx_pool.NewReactor(config.TxPool, kai.txPool)
	kai.txpoolR.SetLogger(logger)

	bOper := blockchain.NewBlockOperations(logger, kai.blockchain, kai.txPool, evPool, stakingUtil)

	kai.evR = evidence.NewReactor(evPool)
	kai.evR.SetLogger(logger)
	blockExec := cstate.NewBlockExecutor(stateDB, logger, evPool, bOper)
	kai.blockExec = blockExec

	state, err := stateDB.LoadStateFromDBOrGenesisDoc(config.Genesis)
	if err != nil {
		return nil, err
	}

	// state starting configs
	// Set private validator for consensus manager.
	privValidator := types.NewDefaultPrivValidator(stack.Config().NodeKey())
	// Determine whether we should do fast sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	config.FastSync.Enable = config.FastSync.Enable && !onlyValidatorIsUs(state, privValidator.GetAddress())
	// Make BlockchainReactor. Don't start fast sync if we're doing a state sync first.
	bcR := bcReactor.NewBlockchainReactor(state, blockExec, bOper, config.FastSync)
	kai.bcR = bcR
	consensusState := consensus.NewConsensusState(
		log.New(),
		config.Consensus,
		state,
		bOper,
		blockExec,
		evPool,
	)
	kai.csManager = consensus.NewConsensusManager(consensusState, config.FastSync)
	// Set private validator for consensus manager.
	kai.csManager.SetPrivValidator(privValidator)
	kai.csManager.SetEventBus(kai.eventBus)

	// init gas price oracle
	kai.APIBackend.gpo = oracles.NewGasPriceOracle(kai.APIBackend, config.GasOracle)
	kai.accMan = stack.AccountManager()

	stack.RegisterAPIs(kai.APIs())
	stack.RegisterLifecycle(kai)

	return kai, nil
}

func (k *Kardiachain) IsListening() bool                  { return true } // Always listening
func (k *Kardiachain) NetVersion() uint64                 { return k.config.NetworkId }
func (k *Kardiachain) TxPool() *tx_pool.TxPool            { return k.txPool }
func (k *Kardiachain) BlockChain() *blockchain.BlockChain { return k.blockchain }
func (k *Kardiachain) DB() kaidb.Database                 { return k.chainDb }
func (k *Kardiachain) Config() *configs.ChainConfig       { return k.blockchain.Config() }

func (k *Kardiachain) Start() error {
	k.sw.AddReactor("BLOCKCHAIN", k.bcR)
	k.sw.AddReactor("CONSENSUS", k.csManager)
	k.sw.AddReactor("TXPOOL", k.txpoolR)
	k.sw.AddReactor("EVIDENCE", k.evR)
	return nil
}

// Stop implements Service, terminating all internal goroutines used by the
// Kardia protocol.
func (k *Kardiachain) Stop() error {
	log.Info("Stopping Kardiachain backend")
	close(k.shutdownChan)
	k.bcR.Stop()
	k.sw.Stop()
	k.csManager.Stop()
	k.blockExec.Stop()
	k.blockchain.Stop()
	return nil
}

func (k *Kardiachain) APIs() []rpc.API {
	apis := kaiapi.GetAPIs(k.APIBackend)

	return append(apis, []rpc.API{
		{
			Namespace: "kai",
			Version:   "1.0",
			Service:   NewPublicKaiAPI(k),
			Public:    true,
		},
		{
			Namespace: "kai",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(k.APIBackend, true),
			Public:    true,
		},
		{
			Namespace: "tx",
			Version:   "1.0",
			Service:   NewPublicTransactionAPI(k),
			Public:    true,
		},
		{
			Namespace: "account",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(k),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   tracers.NewTracerAPI(k.APIBackend),
			Public:    true,
		},
		// Web3 endpoints support
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicWeb3API(k),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(k),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(k.APIBackend, false),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   &publicWeb3API{k.nodeConfig},
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicNodeAccountAPI(k.accMan),
			Public:    true,
		},
		{
			Namespace: "txpool",
			Version:   "1.0",
			Service:   NewPublicTxPoolAPI(k),
			Public:    true,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   NewPublicNetAPI(k.config.NetworkId),
			Public:    true,
		},
		{
			Namespace: "web3",
			Version:   "1.0",
			Service:   &publicWeb3API{k.nodeConfig},
			Public:    true,
		},
	}...)
}

func onlyValidatorIsUs(state cstate.LatestBlockState, privValAddress common.Address) bool {
	if state.Validators.Size() > 1 {
		return false
	}
	addr, _ := state.Validators.GetByIndex(0)
	return privValAddress.Equal(addr)
}

func createAndStartEventBus(logger log.Logger) (*types.EventBus, error) {
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.New("module", "events"))
	if err := eventBus.Start(); err != nil {
		return nil, err
	}
	return eventBus, nil
}
