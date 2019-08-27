package main

import (
	"flag"
	"github.com/kardiachain/go-kardia/node"
)

const (
	LevelDb = iota
	MongoDb
)

// args which are used in terminal
type FlagArgs struct {
	logLevel string
	logTag   string

	// path to config file, if config is defined then it is priority used.
	config string

	// Kardia node's related flags
	name                string
	listenAddr          string
	maxPeers            int
	rpcEnabled          bool
	rpcAddr             string
	rpcPort             int
	bootNode            string
	peer                string
	clearDataDir        bool
	mainChainValIndexes string
	isZeroFee           bool
	isPrivate           bool
	networkId           uint64
	chainId             uint64
	serviceName         string
	noProxy             bool
	peerProxyIP         string

	// Ether/Kardia dualnode related flags
	ethDual         bool

	// Neo/Kardia dualnode related flags
	neoDual bool

	// TRON dualnode
	tronDual bool

	// Private/Kardia dualnode related flags
	privateNetworkId   uint64
	privateValIndexes  string
	privateNodeName    string
	privateChainId     uint64
	privateServiceName string
	privateAddr        string

	// Dualnode's related flags
	dualProtocolName    string
	dualChain           bool
	dualChainValIndexes string
	isPrivateDual       bool
	dualNetworkId       uint64
	publishedEndpoint   string
	subscribedEndpoint  string

	// Development's related flags
	dev            bool
	proposal       int
	votingStrategy string
	mockDualEvent  bool
	devDualChainID uint64
	txs            bool
	txsDelay       int
	numTxs         int
	blockSize      int
	workers        int
	workerCap      int
	maxPending     int
	maxAll         int
	dualEvent      bool

	db             int
	dbUri          string
	dbName         string
	dbDrop         bool
}

func InitFlags(args *FlagArgs) {
	flag.StringVar(&args.logLevel, "loglevel", "info", "minimum log verbosity to display")
	flag.StringVar(&args.logTag, "logtag", "", "filter logging records based on the tag value")
	flag.StringVar(&args.config, "config", "", "path to config file, if config is defined then it is priority used.")

	// Node's related flags
	flag.StringVar(&args.name, "name", "", "Name of node")
	flag.StringVar(&args.listenAddr, "addr", ":30301", "listen address")
	flag.BoolVar(&args.rpcEnabled, "rpc", false, "whether to open HTTP RPC endpoints")
	flag.StringVar(&args.rpcAddr, "rpcaddr", "", "HTTP-RPC server listening interface")
	flag.IntVar(&args.rpcPort, "rpcport", node.DefaultHTTPPort, "HTTP-RPC server listening port")
	flag.StringVar(&args.bootNode, "bootNode", "", "Enode address of node that will be used by the p2p discovery protocol")
	flag.StringVar(&args.peer, "peer", "", "Comma separated enode URLs for P2P static peer")
	flag.BoolVar(&args.clearDataDir, "clearDataDir", false, "remove contents in data dir")
	flag.StringVar(&args.mainChainValIndexes, "mainChainValIndexes", "1,2,3", "Indexes of Main chain validators")
	flag.BoolVar(&args.isZeroFee, "zeroFee", false, "zeroFee is enabled then no gas is charged in transaction. Any gas that sender spends in a transaction will be refunded")
	flag.BoolVar(&args.isPrivate, "private", false, "private is true then peerId will be checked through smc to make sure that it has permission to access the chain")
	flag.Uint64Var(&args.networkId, "networkId", 0, "Your chain's networkId. NetworkId must be greater than 0")
	flag.Uint64Var(&args.chainId, "chainId", 0, "ChainID is used to validate which node is allowed to send message through P2P in the same blockchain")
	flag.StringVar(&args.serviceName, "serviceName", "", "ServiceName is used for displaying as log's prefix")

	// DB type
	flag.IntVar(&args.db, "dbType", LevelDb, "dbType is type of db that will be used to store chain data, current supported types are leveldb and mongodb.")
	flag.StringVar(&args.dbUri, "dbUri", "", "mongodb uri")
	flag.StringVar(&args.dbName, "dbName", "", "mongodb dbName")
	flag.BoolVar(&args.dbDrop, "dbDrop", true, "option drops db")

	// Dualnode's related flags
	flag.BoolVar(&args.ethDual, "dual", false, "whether to run in dual mode")
	flag.BoolVar(&args.tronDual, "trondual", false, "whether to run TRON dual node")
	flag.BoolVar(&args.neoDual, "neodual", false, "whether to run NEO dual node")
	flag.BoolVar(&args.dualChain, "dualchain", false, "run dual chain for group consensus")
	flag.StringVar(&args.dualChainValIndexes, "dualChainValIndexes", "", "Indexes of Dual chain validators")
	flag.BoolVar(&args.isPrivateDual, "privateDual", false, "privateDual is true then peerId will be checked through smc to make sure that it has permission to access the dualchain")
	flag.Uint64Var(&args.privateNetworkId, "privateNetworkId", 0, "Privatechain Network ID. Private Network ID must be greater than 0")
	flag.StringVar(&args.privateValIndexes, "privateValIndexes", "", "Indexes of private chain validators")
	flag.StringVar(&args.privateNodeName, "privateNodeName", "", "Name of private node")
	flag.Uint64Var(&args.privateChainId, "privateChainId", 0, "privateChainId is used to validate which node is allowed to send message through P2P in the private blockchain")
	flag.StringVar(&args.privateServiceName, "privateServiceName", "", "privateServiceName is used for displaying as log's prefix")
	flag.StringVar(&args.privateAddr, "privateAddr", ":5000", "listened address for private chain")
	flag.Uint64Var(&args.dualNetworkId, "dualNetworkId", 100, "dualNetworkID is used to differentiate amongst Kardia-based dual groups")
	flag.BoolVar(&args.noProxy, "noProxy", false, "When triggered, Kardia node is standalone and is not registered in proxy.")
	flag.StringVar(&args.peerProxyIP, "peerProxyIP", "", "IP of the peer proxy for this node to register.")
	flag.StringVar(&args.publishedEndpoint, "publishedEndpoint", "", "0MQ Endpoint that message will be published to")
	flag.StringVar(&args.subscribedEndpoint, "subscribedEndpoint", "", "0MQ Endpoint that dual node subscribes to get dual message.")
	flag.StringVar(&args.dualProtocolName, "dualProtocolName", "", "dualProtocolName is used to set protocol name for dual node.")

	// NOTE: The flags below are only applicable for dev environment. Please add the applicable ones
	// here and DO NOT add non-dev flags.
	flag.BoolVar(&args.dev, "dev", false, "deploy node with dev environment")
	flag.StringVar(&args.votingStrategy, "votingStrategy", "",
		"specify the voting script or strategy to simulate voting. Note that this flag only has effect when --dev flag is set")
	flag.IntVar(&args.proposal, "proposal", 1,
		"specify which node is the proposer. The index starts from 1, and every node needs to use the same proposer index."+
			" Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.mockDualEvent, "mockDualEvent",
		false, "generate fake dual events to trigger dual consensus. Note that this flag only has effect when --dev flag is set.")
	flag.IntVar(&args.maxPeers, "maxpeers", 25,
		"maximum number of network peers (network disabled if set to 0. Note that this flag only has effect when --dev flag is set")
	flag.BoolVar(&args.txs, "txs", false, "generate random transfer txs")
	flag.IntVar(&args.txsDelay, "txsDelay", 10, "delay in seconds between batches of generated txs")
	flag.IntVar(&args.numTxs, "numTxs", 10, "number of of generated txs in one batch")
	flag.IntVar(&args.blockSize, "blockSize", 7192, "number of txs in block")
	flag.IntVar(&args.workers, "workers", 3, "number of workers for broadcast")
	flag.IntVar(&args.workerCap, "workerCap", 512, "number of workerCap for broadcast")
	flag.IntVar(&args.maxPending, "maxPending", 128, "maximum pending txs for every address")
	flag.IntVar(&args.maxAll, "maxAll", 5120000, "maximum all txs")
	flag.BoolVar(&args.dualEvent, "dualEvent", false, "generate initial dual event")
}
