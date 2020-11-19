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

package kardia

import (
	"math/big"
	"sync"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/dualnode/utils"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/kai/tx_pool"
	"github.com/kardiachain/go-kardiamain/ksml"
	message "github.com/kardiachain/go-kardiamain/ksml/proto"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

const KARDIA_PROXY = "KARDIA_PROXY"

// Proxy of Kardia's chain to interface with dual's node, responsible for listening to the chain's
// new block and submiting Kardia's transaction.
type KardiaProxy struct {
	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name   string
	logger log.Logger

	// Kardia's mainchain stuffs.
	kardiaBc     base.BlockChain
	txPool       tx_pool.TxPool
	chainHeadCh  chan events.ChainHeadEvent // Used to subscribe for new blocks.
	chainHeadSub event.Subscription

	// Dual blockchain related fields
	dualBc    base.BlockChain
	eventPool *event_pool.Pool

	// The external blockchain that this dual node's interacting with.
	externalChain base.BlockChainAdapter

	// TODO(sontranrad@,namdoh@): Hard-coded, need to be cleaned up.
	kaiSmcAddress *common.Address
	smcABI        *abi.ABI

	mtx sync.Mutex
}

type MatchRequestInput struct {
	SrcPair     string
	DestPair    string
	SrcAddress  string
	DestAddress string
	Amount      *big.Int
}

type CompleteRequestInput struct {
	// RequestID is ID of request stored in Kardia exchange smc
	RequestID *big.Int
	// Pair is original direction of competed request, for ETH-NEO request, pair is "ETH-NEO"
	Pair string
}

func (p *KardiaProxy) Init(kardiaBc base.BlockChain, txPool tx_pool.TxPool, dualBc base.BlockChain, dualEventPool *event_pool.Pool,
	publishedEndpoint, subscribedEndpoint *string) error {
	// Create a specific logger for Kardia Proxy.
	logger := log.New()
	logger.AddTag(KARDIA_PROXY)

	p.logger = logger
	p.name = configs.KAI
	p.kardiaBc = kardiaBc
	p.txPool = txPool
	p.dualBc = dualBc
	p.eventPool = dualEventPool
	p.chainHeadCh = make(chan events.ChainHeadEvent, 5)

	// Start subscription to blockchain head event.
	p.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(p.chainHeadCh)
	return nil
}

// PublishedEndpoint returns publishedEndpoint
func (p *KardiaProxy) PublishedEndpoint() string {
	return ""
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *KardiaProxy) SubscribedEndpoint() string {
	return ""
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *KardiaProxy) InternalChain() base.BlockChainAdapter {
	return nil
}

func (p *KardiaProxy) ExternalChain() base.BlockChainAdapter {
	return p.externalChain
}

// DualEventPool returns dual's eventPool
func (p *KardiaProxy) DualEventPool() *event_pool.Pool {
	return p.eventPool
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *KardiaProxy) KardiaTxPool() tx_pool.TxPool {
	return p.txPool
}

// DualBlockChain returns dual blockchain
func (p *KardiaProxy) DualBlockChain() base.BlockChain {
	return p.dualBc
}

// KardiaBlockChain returns kardia blockchain
func (p *KardiaProxy) KardiaBlockChain() base.BlockChain {
	return p.kardiaBc
}

func (p *KardiaProxy) Logger() log.Logger {
	return p.logger
}

func (p *KardiaProxy) Name() string {
	return p.name
}

func (p *KardiaProxy) SubmitTx(event *types.EventData) error {
	msg, err := event.GetEventMessage()
	if err != nil {
		return err
	}
	if event.Actions != nil && len(event.Actions) > 0 {
		smc := common.HexToAddress(msg.MasterSmartContract)
		parser := ksml.NewParser(p.Name(), p.PublishedEndpoint(), utils.PublishMessage, p.kardiaBc, p.txPool, &smc, event.Actions, msg, true)
		return parser.ParseParams()
	}
	return nil
}

// ComputeTxMetadata precomputes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *KardiaProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	// Temporarily return simple MetaData for other events
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}

func (p *KardiaProxy) Start() {
	// Start event
	go p.loop()
}

func (p *KardiaProxy) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	p.externalChain = externalChain
}

func (p *KardiaProxy) loop() {
	if p.externalChain == nil {
		panic("External chain needs not to be nil.")
	}
	for {
		select {
		case ev := <-p.chainHeadCh:
			if ev.Block != nil {
				// New block
				// TODO(thietn): concurrency improvement. Consider call new go routine, or have height atomic counter.
				p.handleBlock(ev.Block)
			}
		case err := <-p.chainHeadSub.Err():
			log.Error("Error while listening to new blocks", "error", err)
			return
		}
	}
}

func (p *KardiaProxy) handleBlock(block *types.Block) {
	for _, tx := range block.Transactions() {
		evt, a := p.TxMatchesWatcher(tx)
		if evt != nil && a != nil {
			log.Info("New Kardia's tx detected on smart contract", "addr", tx.To().Hex(), "value", tx.Value())
			if err := p.executeAction(block, tx, evt, a); err != nil {
				log.Error("error while executing watcher action", "err", err)
			}
		}
	}
}

// TxMatchesWatcher checks if tx.To matches with watched smart contract, if matched return watched event
func (p *KardiaProxy) TxMatchesWatcher(tx *types.Transaction) (*types.Watcher, *abi.ABI) {
	db := p.kardiaBc.DB()
	if tx.To() == nil {
		return nil, nil
	}
	a := db.ReadSmartContractAbi(tx.To().Hex())
	if a != nil {
		// get method and input data from tx
		input := tx.Data()
		method, err := a.MethodById(input)
		if err != nil {
			p.logger.Error("cannot get method from input", "err", err, "address", tx.To().Hex())
			return nil, nil
		}
		// get event from smc address and method
		return db.ReadEvent(tx.To().Hex(), method.Name), a
	}
	return nil, nil
}

// Detects update on kardia master smart contract and creates corresponding dual event to submit to
// dual event pool
func (p *KardiaProxy) executeAction(block *types.Block, tx *types.Transaction, action *types.Watcher, abi *abi.ABI) error {
	// TODO: @lew
	// Double check to ensure the signer
	sender, err := types.Sender(types.HomesteadSigner{}, tx)
	if err != nil {
		return err
	}
	method, params, err := ksml.GetMethodAndParams(*abi, tx.Data())
	if err != nil || method == "" {
		return err
	}
	// get master smart contract
	masterSmc, _ := p.kardiaBc.DB().ReadEvents(tx.To().Hex())
	eventMessage := &message.EventMessage{
		MasterSmartContract: masterSmc,
		TransactionId:       tx.Hash().Hex(),
		From:                sender.Hex(),
		To:                  tx.To().Hex(),
		Method:              method,
		Params:              params,
		Amount:              tx.Value().Uint64(),
		Sender:              sender.Hex(),
		BlockNumber:         block.Height(),
		Timestamp:           block.Header().Time,
	}
	if len(action.WatcherActions) > 0 {
		parser := ksml.NewParser(p.Name(), p.PublishedEndpoint(), utils.PublishMessage, p.kardiaBc, p.txPool, tx.To(), action.WatcherActions, eventMessage, false)
		if err := parser.ParseParams(); err != nil {
			return err
		}
		if len(parser.GlobalParams) > 0 {
			for _, p := range parser.GlobalParams {
				v, err := ksml.InterfaceToString(p)
				if err != nil {
					return err
				}
				eventMessage.Params = append(eventMessage.Params, v)
			}
		}
	}
	txHash := tx.Hash()
	dualEvent := types.NewDualEvent(p.dualBc.CurrentBlock().Height(), false /* externalChain */, types.KARDIA, &txHash, eventMessage, action.DualActions)
	txMetadata, err := p.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error computing tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetadata
	signedEvent, err := types.SignEvent(dualEvent, &p.dualBc.Config().BaseAccount.PrivateKey)
	if err != nil {
		return err
	}
	log.Info("Create DualEvent for Kardia's Tx", "dualEvent", signedEvent.Hash().Hex())
	if err := p.DualEventPool().AddEvent(signedEvent); err != nil {
		p.Logger().Error("error while adding dual event", "err", err, "event", signedEvent.Hash().Hex())
		return err
	}
	log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
		"eventHash", dualEvent.Hash().String())
	return nil
}

func (p *KardiaProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	panic("this function is not implemented")
}

func (p *KardiaProxy) Lock() {
	p.mtx.Lock()
}

func (p *KardiaProxy) UnLock() {
	p.mtx.Unlock()
}
