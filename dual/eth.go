package dual

import (
	"encoding/hex"
	"fmt"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/dual/ethsmc"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"math/big"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/params"
	"github.com/kardiachain/go-kardia/lib/log"
)

const (
	// // headChannelSize is the size of channel listening to ChainHeadEvent.
	headChannelSize = 5
	// GenesisAccount used for matchEth tx
	genesisAccount = "0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28"
)

var DefaultEthKardiaConfig = EthKardiaConfig{
	Name:            "GethKardia", // Don't need to change, default instance name for geth is "geth".
	ListenAddr:      ":30303",
	MaxPeers:        10,
	LightNode:       false,
	LightPeers:      5,
	LightServ:       0,
	StatName:        "eth-kardia-1",
	ContractAddress: ethsmc.EthContractAddress,

	CacheSize: 1024,
}

// EthKardiaConfig provides configuration when starting Eth subnode.
type EthKardiaConfig struct {
	ContractAddress string // address of Eth smart contract to watch.

	// Network configs
	Name        string
	ListenAddr  string
	MaxPeers    int
	LightNode   bool   // Starts with light sync, otherwise starts with fast sync.
	LightPeers  int    // Max number of light peers.
	LightServ   int    // Max percentage of time allowed for serving LES requests (0-90)"
	ReportStats bool   // Reports node statistics to network centralized statistics collection system.
	StatName    string // Node name to use when report to Rinkeby stats collection.

	// Performance configs
	CacheSize int // Cache memory size in MB for database & trie. This must be small enough to leave enough memory for separate Kardia chain cache.
}

// EthKarida is a full Ethereum node running inside Karida
type EthKardia struct {
	geth   *node.Node
	config *EthKardiaConfig
	ethSmc *ethsmc.EthSmc
	kChain *blockchain.BlockChain
	txPool *blockchain.TxPool
}

// EthKardia creates a Ethereum sub node.
func NewEthKardia(config *EthKardiaConfig, kChain *blockchain.BlockChain, txPool *blockchain.TxPool) (*EthKardia, error) {
	datadir := defaultEthDataDir()

	// Creates datadir with testnet follow eth standards.
	// TODO(thientn) : options to choose different networks.
	datadir = filepath.Join(datadir, "rinkeby")
	bootUrls := params.RinkebyBootnodes
	bootstrapNodes := make([]*discover.Node, 0, len(bootUrls))
	bootstrapNodesV5 := make([]*discv5.Node, 0, len(bootUrls)) // rinkeby set default bootnodes as also discv5 nodes.
	for _, url := range bootUrls {
		peer, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodes = append(bootstrapNodes, peer)

		peerV5, err := discv5.ParseNode(url)
		if err != nil {
			log.Error("BootstrapV5 URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodesV5 = append(bootstrapNodesV5, peerV5)
	}

	// similar to utils.SetNodeConfig
	nodeConfig := &node.Config{
		DataDir: datadir,
		IPCPath: "geth.ipc",
		Name:    config.Name,
	}

	// similar to utils.SetP2PConfig
	nodeConfig.P2P = p2p.Config{
		BootstrapNodes:   bootstrapNodes,
		ListenAddr:       config.ListenAddr,
		MaxPeers:         config.MaxPeers,
		DiscoveryV5:      config.LightNode, // Force using discovery if light node, as in flags.go.
		BootstrapNodesV5: bootstrapNodesV5,
	}

	// similar to cmd/eth/config.go/makeConfigNode
	ethConf := &eth.DefaultConfig
	ethConf.NetworkId = 4 // Rinkeby Id
	ethConf.Genesis = core.DefaultRinkebyGenesisBlock()

	ethConf.LightServ = config.LightServ
	ethConf.LightPeers = config.LightPeers

	// similar to cmd/utils/flags.go
	ethConf.DatabaseCache = config.CacheSize * 75 / 100
	ethConf.TrieCache = config.CacheSize * 25 / 100
	// Hardcode to 50% of 2048 file descriptor limit for whole process, as in flags.go/makeDatabaseHandles()
	ethConf.DatabaseHandles = 1024

	// Creates new node.
	ethNode, err := node.New(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("protocol node: %v", err)
	}
	if config.LightNode {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return les.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	} else {
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return eth.New(ctx, ethConf) }); err != nil {
			return nil, fmt.Errorf("ethereum service: %v", err)
		}
	}

	// Registers ethstats service to report node stat to testnet system.
	if config.ReportStats {
		url := fmt.Sprintf("[EthKardia]%s:Respect my authoritah!@stats.rinkeby.io", config.StatName)
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			// Retrieve both eth and les services
			var ethServ *eth.Ethereum
			ctx.Service(&ethServ)

			var lesServ *les.LightEthereum
			ctx.Service(&lesServ)

			return ethstats.New(url, ethServ, lesServ)
		}); err != nil {
			log.Error("Failed to register the Ethereum Stats service", "err", err)
		}
	}
	return &EthKardia{
		geth: ethNode, config: config, ethSmc: ethsmc.NewEthSmc(), kChain: kChain, txPool: txPool}, nil
}

// defaultEthDataDir returns default Eth root datadir.
func defaultEthDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home == "" {
		panic("Fail to get OS home directory")
	}
	return filepath.Join(home, ".ethereum")
}

// Copy from go-kardia/node
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// Start starts the Ethereum node.
func (n *EthKardia) Start() error {
	err := n.geth.Start()

	if err != nil {
		return err
	}
	go n.syncHead()
	return nil
}

// Stop shut down the Ethereum node.
func (n *EthKardia) Stop() error {
	return n.geth.Stop()
}

// EthNode returns the standard Eth Node.
func (n *EthKardia) EthNode() *node.Node {
	return n.geth
}

// Client return the KardiaEthClient to acess Eth subnode.
func (n *EthKardia) Client() (*KardiaEthClient, error) {
	rpcClient, err := n.geth.Attach()
	if err != nil {
		return nil, err
	}
	return &KardiaEthClient{ethClient: ethclient.NewClient(rpcClient), stack: n.geth}, nil
}

func (n *EthKardia) BlockChain() *core.BlockChain {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.BlockChain()
}

func (n *EthKardia) TxPool() *core.TxPool {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.TxPool()
}

// syncHead syncs with latest events from Eth network to Kardia.
func (n *EthKardia) syncHead() {
	var ethService *eth.Ethereum

	n.geth.Service(&ethService)

	if ethService == nil {
		log.Error("Not implement dual sync for Eth light mode yet")
		return
	}

	ethChain := ethService.BlockChain()

	chainHeadEventCh := make(chan core.ChainHeadEvent, headChannelSize)
	headSubCh := ethChain.SubscribeChainHeadEvent(chainHeadEventCh)
	defer headSubCh.Unsubscribe()

	blockCh := make(chan *types.Block, 1)

	// Follow other examples.
	// Listener to exhaust extra event while sending block to our channel.
	go func() {
	ListenerLoop:
		for {
			select {
			// Gets chain head events, drop if overload.
			case head := <-chainHeadEventCh:
				select {
				case blockCh <- head.Block:
					// Block field would be nil here.
				default:
					// TODO(thientn): improves performance/handling here.
				}
			case <-headSubCh.Err():
				break ListenerLoop
			}
		}
	}()

	// Handler loop for new blocks.
	for {
		select {
		case block := <-blockCh:
			if !n.config.LightNode {
				go n.handleBlock(block)
			}
		}
	}
}

func (n *EthKardia) handleBlock(block *types.Block) {
	// TODO(thientn): block from this event is not guaranteed newly update. May already handled before.

	// Some events has nil block.
	if block == nil {
		// TODO(thientn): could call blockchain.CurrentBlock() here.
		log.Info("handleBlock with nil block")
		return
	}

	header := block.Header()
	txns := block.Transactions()

	log.Info("handleBlock...", "header", header, "txns size", len(txns))

	/* Can be use to check contract state, but currently has memory leak.
	b := n.BlockChain()
	state, err := b.State()
	if err != nil {
		log.Error("Get Geth state() error", "err", err)
		return
	}
	*/

	contractAddr := ethCommon.HexToAddress(n.config.ContractAddress)

	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == contractAddr {
			log.Error("New tx detected on smart contract", "addr", contractAddr.Hex(), "value", tx.Value())
			// TODO(thientn): parse input & create Kardia tx
			go n.updateKardiaSmc(tx)
		}
	}
}

func (n *EthKardia) updateKardiaSmc(tx *types.Transaction) {
	input := tx.Data()
	method, err := n.ethSmc.InputMethodName(input)
	if err != nil {
		log.Error("Error when unpack Eth smc input", "error", err)
		return
	}
	if method == "deposit" {
		ethValue := tx.Value()
		neoAddr, err := n.ethSmc.UnpackDepositInput(input)
		if err != nil {
			log.Error("Error when unpack Eth deposit tx input", "error", err, "tx", tx)
		}
		log.Info("Create Kardia tx to update matchEth", "value", ethValue, "neoAddr", neoAddr, "eth tx hash", tx.Hash().Hex())
		n.sendKardiaMatchEth(ethValue)

	} else if method == "release" {
		log.Info("Confirmed Eth release tx", "tx", tx)
	}
}

func (n *EthKardia) sendKardiaMatchEth(amount *big.Int) {
	statedb, err := n.kChain.State()
	if err != nil {
		log.Error("fail to get Kardia state to send MatchEth", "error", err)
		return
	}

	addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[genesisAccount])
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := CreateKardiaMatchAmountTx(addrKey, statedb, amount, 1)
	if err := n.txPool.AddLocal(tx); err != nil {
		log.Error("Fail to add Kardia MatchEth tx", "error", err)
	} else {
		log.Info("Added Kardia tx MatchEth", "tx", tx.Hash().Hex())
	}

}

func (n *EthKardia) SendEthFromContract(value *big.Int) {

	statedb, err := n.BlockChain().State()
	if err != nil {
		log.Error("Fail to get Ethereum state to create release tx", "err", err)
		return
	}
	contractAddr := ethCommon.HexToAddress(ethsmc.EthContractAddress)
	nonce := statedb.GetNonce(contractAddr)
	if nonce == 0 {
		log.Error("Eth state return 0 for nonce of contract address, SKIPPING TX CREATION", "addr", ethsmc.EthContractAddress)
	}
	tx := n.ethSmc.CreateEthReleaseTx(value, nonce)

	if err := n.TxPool().AddLocal(tx); err != nil {
		log.Error("Fail to add Ether tx", "error", err)
	}
}
