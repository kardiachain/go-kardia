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

package eth

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	ethCommon "github.com/ethereum/go-ethereum/common"
	ethCore "github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode"
	"github.com/kardiachain/go-kardia/dualnode/eth/ethsmc"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// headChannelSize is the size of channel listening to ChainHeadEvent.
	headChannelSize = 5
	ServiceName = "ETH"
)

// A full Ethereum node. In additional, it provides additional interface with dual's node,
// responsible for listening to Eth blockchain's new block and submiting Eth's transaction .
type Eth struct {

	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name   string
	logger log.Logger

	// Eth's blockchain stuffs.
	geth   *node.Node
	config *EthConfig
	ethSmc *ethsmc.EthSmc

	// Dual blockchain related fields
	dualChain base.BaseBlockChain
	eventPool *event_pool.EventPool

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// TODO(namdoh,thientn): Deprecate this. This is needed solely to get Kardia's state in order
	// to get Eth's amount from Kardia's smart contract.
	kardiaChain base.BaseBlockChain
	// TODO(namdoh,thientn): Deprecate this. This is needed solely submit remove amount Tx to
	// Karida's tx pool.
	txPool *tx_pool.TxPool

	// TODO(namdoh,thientn): Hard-coded for prototyping. This need to be passed dynamically.
	smcABI        *abi.ABI
	kaiSmcAddress *common.Address

	// Cache certain Kardia state's references for ease of use.
	// TODO(namdoh@): Remove once this is extracted from dual chain's state.
	kardiaSmcs []*types.KardiaSmartcontract
}

// Eth creates a Ethereum sub node.
func NewEth(config *EthConfig, kardiaChain base.BaseBlockChain, txPool *tx_pool.TxPool, dualChain base.BaseBlockChain, dualEventPool *event_pool.EventPool, smcAddr *common.Address, smcABIStr string) (*Eth, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	// Create a specific logger for ETH Proxy.
	logger := log.New()
	logger.AddTag(ServiceName)

	bootUrls := params.RinkebyBootnodes

	datadir := defaultEthDataDir()
	// similar to cmd/eth/config.go/makeConfigNode
	ethConf := &eth.DefaultConfig
	ethConf.NetworkId = uint64(config.NetworkId)

	switch ethConf.NetworkId {
	case 1: // mainnet
		ethConf.Genesis = ethCore.DefaultGenesisBlock()
		datadir = filepath.Join(datadir, "mainnet", config.Name)
		bootUrls = params.MainnetBootnodes
	case 3: // ropsten
		ethConf.Genesis = ethCore.DefaultTestnetGenesisBlock()
		datadir = filepath.Join(datadir, "ropsten", config.Name)
		bootUrls = params.TestnetBootnodes
	case 4: // rinkeby
		ethConf.Genesis = ethCore.DefaultRinkebyGenesisBlock()
		datadir = filepath.Join(datadir, "rinkeby", config.Name)
	default: // default is rinkeby
		ethConf.Genesis = ethCore.DefaultRinkebyGenesisBlock()
		datadir = filepath.Join(datadir, "rinkeby", config.Name)
	}

	bootstrapNodes := make([]*enode.Node, 0, len(bootUrls))
	bootstrapNodesV5 := make([]*discv5.Node, 0, len(bootUrls)) // rinkeby set default bootnodes as also discv5 nodes.
	for _, url := range bootUrls {
		peer, err := enode.ParseV4(url)
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
		DataDir:          datadir,
		IPCPath:          "geth.ipc",
		Name:             config.Name,
		HTTPHost:         config.HTTPHost,
		HTTPPort:         config.HTTPPort,
		HTTPVirtualHosts: config.HTTPVirtualHosts,
		HTTPCors:         config.HTTPCors,
	}

	// similar to utils.SetP2PConfig
	nodeConfig.P2P = p2p.Config{
		BootstrapNodes:   bootstrapNodes,
		ListenAddr:       config.ListenAddr,
		MaxPeers:         config.MaxPeers,
		DiscoveryV5:      config.LightNode, // Force using discovery if light node, as in flags.go.
		BootstrapNodesV5: bootstrapNodesV5,
	}

	ethConf.LightServ = config.LightServ
	ethConf.LightPeers = config.LightPeers

	// similar to cmd/utils/flags.go
	ethConf.DatabaseCache = config.CacheSize * 75 / 100
	// Hardcode to 50% of 2048 file descriptor limit for whole process, as in flags.go/makeDatabaseHandles()
	//ethConf.DatabaseHandles = 1024

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
		url := fmt.Sprintf("[Eth]%s:Respect my authoritah!@stats.rinkeby.io", config.StatName)
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

	// TODO(namdoh@): Pass this dynamically from Kardia's state.
	actionsTmp := [...]*types.DualAction{
			&types.DualAction{
				Name: dualnode.CreateKardiaMatchAmountTx,
			},
	}
	kardiaSmcsTemp := [...]*types.KardiaSmartcontract{
		&types.KardiaSmartcontract{
			EventWatcher: &types.Watcher{
				SmcAddress:    config.ContractAddress,
				WatcherAction: dualnode.CreateDualEventFromEthTxAndEnqueue,
			},
			Actions: &types.DualActions{
				Actions: actionsTmp[:],
			},
		}}

	return &Eth{
		name:          ServiceName,
		geth:          ethNode,
		config:        config,
		ethSmc:        ethsmc.NewEthSmc(),
		kardiaChain:   kardiaChain,
		txPool:        txPool,
		dualChain:     dualChain,
		eventPool:     dualEventPool,
		kaiSmcAddress: smcAddr,
		smcABI:        &smcABI,
		kardiaSmcs:    kardiaSmcsTemp[:],
	}, nil
}

// PublishedEndpoint returns publishedEndpoint
func (p *Eth) PublishedEndpoint() string {
	return ""
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *Eth) SubscribedEndpoint() string {
	return ""
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *Eth) InternalChain() base.BlockChainAdapter {
	return p.internalChain
}

func (p *Eth) ExternalChain() base.BlockChainAdapter {
	return nil
}

// DualEventPool returns dual's eventPool
func (p *Eth) DualEventPool() *event_pool.EventPool {
	return p.eventPool
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *Eth) KardiaTxPool() *tx_pool.TxPool {
	return p.txPool
}

// DualBlockChain returns dual blockchain
func (p *Eth) DualBlockChain() base.BaseBlockChain {
	return p.dualChain
}

// KardiaBlockChain returns kardia blockchain
func (p *Eth) KardiaBlockChain() base.BaseBlockChain {
	return p.kardiaChain
}

func (p *Eth) Logger() log.Logger {
	return p.logger
}

func (p *Eth) Name() string {
	return p.name
}

func (n *Eth) SubmitTx(event *types.EventData) error {
	statedb, err := n.kardiaChain.State()
	if err != nil {
		return configs.ErrFailedGetState
	}
	switch event.Data.TxMethod {
	case configs.AddOrderFunction:
		if len(event.Data.ExtData) != configs.ExchangeV2NumOfExchangeDataField {
			return configs.ErrInsufficientExchangeData
		}
		senderAddr := common.HexToAddress(configs.KardiaAccountToCallSmc)
		originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
		fromType := string(event.Data.ExtData[configs.ExchangeV2SourcePairIndex])
		toType := string(event.Data.ExtData[configs.ExchangeV2DestPairIndex])
		// We get all releasable orders which are matched with newly added order
		releases, err := utils.CallKardiGetMatchingResultByTxId(senderAddr, n.kardiaChain, statedb, originalTx)
		if err != nil {
			n.Logger().Error("error on CallKardiGetMatchingResultByTxId", "senderAddr", senderAddr, "originalTx", originalTx, "err", err)
			return err
		}
		log.Info("Release info", "release", releases)
		if releases != "" {
			fields := strings.Split(releases, configs.ExchangeV2ReleaseFieldsSeparator)
			if len(fields) != 4 {
				log.Error("Invalid number of field", "release", releases)
				return errors.New("invalid number of field for release")
			}
			arrTypes := strings.Split(fields[configs.ExchangeV2ReleaseToTypeIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAddresses := strings.Split(fields[configs.ExchangeV2ReleaseAddressesIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAmounts := strings.Split(fields[configs.ExchangeV2ReleaseAmountsIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrTxIds := strings.Split(fields[configs.ExchangeV2ReleaseTxIdsIndex], configs.ExchangeV2ReleaseValuesSepatator)
			fromAmount, toAmount, err := utils.CallGetRate(fromType, toType, n.kardiaChain, statedb)
			log.Info("Release","fromType", fromType, "toType", toType, "fromAmount", fromAmount, "toAmount", toAmount)
			if err != nil {
				n.Logger().Error("error on getting rate", "fromType", fromType, "toType", toType, "err", err)
				return err
			}
			for i, t := range arrTypes {
				if t == configs.ETH { // The receiving end of the release is ETH
					if arrAmounts[i] == "" || arrAddresses[i] == "" || arrTxIds[i] == "" {
						log.Error("Missing release info", "originalTx", originalTx, "field", i, "releases", releases)
						continue
					}
					address := arrAddresses[i]

					amount, err1 := strconv.ParseInt(arrAmounts[i], 10, 64) //big.NewInt(0).SetString(arrAmounts[i], 10)
					if err1 != nil {
						log.Error("Error parse amount", "amount", arrAmounts[i])
						continue
					}
					// Get rate base on the dual node exchange
					// When deposit NEO(TRX)-ETH then matching order ETH-NEO(TRX)
					if t == fromType { // ETH
						fromType = toType
					} else {
						tempToAmount := toAmount
						toAmount = fromAmount
						fromAmount = tempToAmount
					}

					// Calculate the released amount by wei
					convertedAmount := big.NewFloat(float64(amount))
					convertedAmount = convertedAmount.Quo(convertedAmount, new(big.Float).SetInt(fromAmount))
					convertedAmount = convertedAmount.Mul(convertedAmount, new(big.Float).SetInt(toAmount))
					amountToRelease, err := getReleasedAmount(fromType, convertedAmount)
					if err != nil {
						log.Error("Error getting released amount", "err", err)
					}

					log.Info("Amount", "convertedAmount", convertedAmount, "amountToRelease", amountToRelease)
					if err = n.releaseTxAndCompleteRequest(arrTxIds[i], amountToRelease, address); err != nil {
						log.Error("Error release ETH", "originalTxID", arrTxIds[i], "receiver", address, "err", err)
					}
				}
			}
			return nil
		}
		log.Info("There is no matched result for tx", "originalTxId", originalTx)
	default:
		log.Warn("Unexpected method comes to exchange contract", "method", event.Data.TxMethod)
		return configs.ErrUnsupportedMethod
	}
	return configs.ErrUnsupportedMethod
}

// getReleasedAmount converts amount to ETH amount based on original chain (NEO, TRX)
func getReleasedAmount(fromType string, convertedAmount *big.Float) (*big.Int, error) {
	switch fromType {
	case configs.NEO:
		// if fromType is NEO then convert from NEO unit (10^8) to 10^18
		convertedAmount = big.NewFloat(float64(1)).Mul(convertedAmount, utils.TenPoweredByTenFloat)
		temp, _ := convertedAmount.Float64()
		return big.NewInt(int64(math.Round(temp))), nil
	case configs.TRON:
		// if fromType is TRON then convert from TRON unit (10^6) to 10^18
		convertedAmount = big.NewFloat(float64(1)).Mul(convertedAmount, utils.TenPoweredByTwelveFloat)
		temp, _ := convertedAmount.Float64()
		return big.NewInt(int64(math.Round(temp))), nil
	default:
		return nil, fmt.Errorf("invalid fromType %v", fromType)
	}
}

// releaseTxAndCompleteRequest release eth to receiver and creates a tx to complete it in kardia smart contract
func (n *Eth) releaseTxAndCompleteRequest(matchedTxId string, amount *big.Int, receiver string) error {
	releaseTxId, err := n.submitEthReleaseTx(amount, receiver)
	if err != nil {
		return err
	}
	// Update target tx id for matchedTxID to make it's matched and released
	tx, err := utils.UpdateKardiaTargetTx(n.txPool.State(), matchedTxId, releaseTxId, "target")
	if err != nil {
		log.Error("Failed to update target tx", "matchedTxId", matchedTxId, "tx", releaseTxId)
		return err
	}
	err = n.txPool.AddTx(tx)
	if err != nil {
		log.Error("Fail to add Kardia tx to update target tx", "err", err, "tx", tx)
		return err
	}
	log.Info("Submitted tx to Kardia to update target tx successully", "matchedTxId", matchedTxId,
		"releaseTxId", releaseTxId)
	return nil
}

// In case it's an exchange event (matchOrder), we will calculate matching order later
// when we submitTx to externalChain, so I simply return a basic metadata here basing on target and event hash,
// to differentiate TxMetadata inferred from events
func (n *Eth) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.ETHEREUM,
	}, nil
}

func (n *Eth) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	n.internalChain = internalChain
}

// Start starts the Ethereum node.
func (n *Eth) Start() error {
	err := n.geth.Start()

	if err != nil {
		return err
	}
	go n.syncHead()
	return nil
}

// Stop shut down the Ethereum node.
func (n *Eth) Stop() error {
	return n.geth.Stop()
}

// EthNode returns the standard Eth Node.
func (n *Eth) EthNode() *node.Node {
	return n.geth
}

// Returns the EthClient to acccess Eth subnode.
func (n *Eth) Client() (*EthClient, error) {
	rpcClient, err := n.geth.Attach()
	if err != nil {
		return nil, err
	}
	return &EthClient{ethClient: ethclient.NewClient(rpcClient), stack: n.geth}, nil
}

func (n *Eth) submitEthReleaseTx(value *big.Int, receiveAddress string) (string, error) {
	statedb, err := n.ethBlockChain().State()
	if err != nil {
		log.Error("Fail to get Ethereum state to create release tx", "err", err)
		return "", err
	}
	// TODO(thientn,namdoh): Remove hard-coded address.
	accountSign := ethCommon.HexToAddress(ethsmc.EthAccountSign)
	tx, err := CreateEthReleaseAmountTx(n.config.ContractAddress, accountSign, receiveAddress, statedb, value, n.ethSmc)
	if err != nil {
		log.Error("Fail to create Eth's tx", "err", err)
		return "", err
	}
	err = n.ethTxPool().AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Ether tx", "error", err)
		return "", err
	}
	log.Info("Add Eth release tx successfully", "txhash", tx.Hash().Hex())
	return tx.Hash().String(), nil
}

func (n *Eth) ethBlockChain() *ethCore.BlockChain {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.BlockChain()
}

func (n *Eth) ethTxPool() *ethCore.TxPool {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.TxPool()
}

// syncHead syncs with latest events from Eth network to Kardia.
func (n *Eth) syncHead() {
	var ethService *eth.Ethereum

	n.geth.Service(&ethService)

	if ethService == nil {
		log.Error("Not implement dual sync for Eth light mode yet")
		return
	}

	ethChain := ethService.BlockChain()

	chainHeadEventCh := make(chan ethCore.ChainHeadEvent, headChannelSize)
	headSubCh := ethChain.SubscribeChainHeadEvent(chainHeadEventCh)
	defer headSubCh.Unsubscribe()

	blockCh := make(chan *ethTypes.Block, 1)

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

	if n.config.DualNodeConfig != nil {
		go n.mockBlockGenerationRoutine(n.config.DualNodeConfig.Triggering, blockCh)
	}

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

func (n *Eth) handleBlock(block *ethTypes.Block) {
	// TODO(thientn): block from this event is not guaranteed newly update. May already handled before.

	// Some events has nil block.
	if block == nil {
		// TODO(thientn): could call blockchain.CurrentBlock() here.
		log.Info("handleBlock with nil block")
		return
	}

	log.Info("HandleBlock...", "header", block.Header().Hash(), "txns size", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		for _, ks := range n.kardiaSmcs {
			if n.TxMatchesWatcher(tx, ks.EventWatcher) {
				log.Info("New Eth's tx detected on smart contract", "addr", ks.EventWatcher.SmcAddress, "value", tx.Value())
				n.ExecuteSmcActions(tx, ks.EventWatcher.WatcherAction, ks.Actions)
			}
		}
	}
}

func (n *Eth) TxMatchesWatcher(tx *ethTypes.Transaction, watcher *types.Watcher) bool {
	contractAddr := ethCommon.HexToAddress(watcher.SmcAddress)
	// TODO(thientn): Make this tx matcher more robust.
	return tx.To() != nil && *tx.To() == contractAddr
}

func (n *Eth) ExecuteSmcActions(tx *ethTypes.Transaction, watcherAction string, actions *types.DualActions) {
	var err error
	switch watcherAction {
	case dualnode.CreateDualEventFromEthTxAndEnqueue:
		err = n.createDualEventFromEthTxAndEnqueue(tx, actions)
	}

	if err != nil {
		log.Error("Error handling tx", "txHash", tx.Hash(), "err", err)
		return
	}
}

func (n *Eth) createDualEventFromEthTxAndEnqueue(tx *ethTypes.Transaction, actions *types.DualActions) error {
	if n.internalChain == nil {
		panic("Internal chain needs not to be nil.")
	}

	// Compute eventSummary
	chainId := tx.ChainId()
	signer := ethTypes.NewEIP155Signer(chainId)
	address, err := ethTypes.Sender(signer, tx)
	if err != nil {
		return err
	}
	sender := address.String()
	log.Info("Sender info", "sender", sender)
	eventSummary, err := n.extractEthTxSummary(tx, sender)
	if err != nil {
		log.Error("Error when extracting Eth's tx summary.", "err", err)
		// TODO(#140): Handle smart contract failure correctly.
		panic("Not yet implemented!")

	}
	// Create dualEvent
	dualStateDB, err := n.dualChain.State()
	if err != nil {
		log.Error("Fail to get Kardia state", "error", err)
		return err
	}
	nonce := dualStateDB.GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	ethTxHash := tx.Hash()
	txHash := common.BytesToHash(ethTxHash[:])
	dualEvent := types.NewDualEvent(nonce, true /* externalChain */, types.ETHEREUM, &txHash, &eventSummary, actions)
	txMetaData, err := n.internalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	log.Info("Create DualEvent for Eth's Tx", "dualEvent", dualEvent)

	// Add to event pool
	if err = n.eventPool.AddEvent(dualEvent); err != nil {
		log.Error("Fail to add dual's event", "error", err)
		return err
	}

	log.Info("Submitted Eth's DualEvent to event pool successfully", "eventHash", dualEvent.Hash().Hex())
	return nil
}

func (n *Eth) extractEthTxSummary(tx *ethTypes.Transaction, sender string) (types.EventSummary, error) {
	input := tx.Data()
	method, err := n.ethSmc.InputMethodName(input)
	if err != nil {
		log.Error("Error when unpack Eth smc input", "error", err)
		return types.EventSummary{}, err
	}
	// Currently we only proceed deposit function from ETH
	if method != configs.ExternalDepositFunction {
		return types.EventSummary{
			TxMethod: method,
			TxValue:  tx.Value(),
			ExtData:  nil,
		}, nil
	}
	extraData := make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
	receiveAddress, destination, err := n.ethSmc.UnpackDepositInput(input)
	if err != nil {
		return types.EventSummary{}, err
	}
	if receiveAddress == "" || destination == "" {
		return types.EventSummary{}, configs.ErrInsufficientExchangeData
	}
	// Compose extraData struct for fields related to exchange
	extraData[configs.ExchangeV2SourceAddressIndex] = []byte(sender)
	extraData[configs.ExchangeV2DestAddressIndex] = []byte(receiveAddress)
	extraData[configs.ExchangeV2SourcePairIndex] = []byte(configs.ETH)
	extraData[configs.ExchangeV2DestPairIndex] = []byte(destination)
	extraData[configs.ExchangeV2AmountIndex] = tx.Value().Bytes()
	extraData[configs.ExchangeV2OriginalTxIdIndex] = tx.Hash().Bytes()
	extraData[configs.ExchangeV2TimestampIndex] = big.NewInt(time.Now().Unix()).Bytes()

	return types.EventSummary{
		TxMethod: method,
		TxValue:  tx.Value(),
		ExtData:  extraData,
	}, nil
}

func (n *Eth) mockBlockGenerationRoutine(triggeringConfig *dev.TriggeringConfig, blockCh chan *ethTypes.Block) {
	contractAddr := ethCommon.HexToAddress(n.config.ContractAddress)
	for {
		for timeout := range triggeringConfig.TimeIntervals {
			time.Sleep(time.Duration(timeout) * time.Millisecond)

			block := triggeringConfig.GenerateEthBlock(contractAddr)
			log.Info("Generating an Eth block to trigger a new DualEvent", "block", block)
			blockCh <- block
		}

		if !triggeringConfig.RepeatInfinitely {
			break
		}
	}
}

/****************************** EthClient ******************************/

// Provides read/write functions to data in Ethereum subnode.
// This is implements with a mixture of direct access on the node , or internal RPC calls.
type EthClient struct {
	ethClient *ethclient.Client
	stack     *node.Node // The running Ethereum node
}

// SyncDetails returns the current sync status of the node.
func (e *EthClient) NodeSyncStatus() (*ethereum.SyncProgress, error) {
	return e.ethClient.SyncProgress(context.Background())
}