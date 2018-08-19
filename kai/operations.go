package kai

import (
	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/storage"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"
)

// Operation keeps current state and provides blockchain operatons to be called by consensus engine.
type Operation struct {
	config      *configs.ChainConfig
	state       *state.StateDB    // State DB to apply state changes
	chainDb     *storage.Database // Blockchain database
	blockchain  *blockchain.BlockChain
	kaiService  *Kardia
	txPool      *blockchain.TxPool
	pendingTxns []*types.Transaction

	header   *types.Header
	receipts *types.Receipts

	gasPool *blockchain.GasPool
	// need private key to sign
}

func NewOperation(n *node.Node) *Operation {
	p := &Operation{}

	var kService *Kardia
	if err := n.Service(&kService); err != nil {
		log.Error("Cannot get Kardia Service", "err", err)
		return p
	}
	p.config = kService.ChainConfig()
	p.kaiService = kService
	p.blockchain = kService.BlockChain()
	p.txPool = kService.TxPool()

	currentState, err := p.blockchain.State()
	if err != nil {
		log.Error("Fail to get head state", "err", err)
		return nil
	}
	p.state = currentState

	// TODO: finds variable for gas limit from header
	var gasLimit uint64 = 100
	p.gasPool = new(blockchain.GasPool).AddGas(gasLimit)

	// FIXME: create header
	return p
}

// CollectTransactions creates list of proposed transactions from the pool, also cache this list.
func (p *Operation) CollectTransactions() []*types.Transaction {
	pending, err := p.txPool.Pending()
	if err != nil {
		log.Error("Fail to get pending txns", "err", err)
		return nil
	}

	// TODO: do basic verification & check with gas & sort by nonce
	// check code NewTransactionsByPriceAndNonce
	pendingTxns := make([]*types.Transaction, 0)
	for _, txns := range pending {
		for _, txn := range txns {
			pendingTxns = append(pendingTxns, txn)
		}
	}
	p.pendingTxns = pendingTxns
	return pendingTxns
}

// CommitTransactions execute & commit the cache list of pending transactions.
func (p *Operation) CommitTransactions() (types.Receipts, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
	)

	counter := 0
	for _, txn := range p.pendingTxns {
		p.state.Prepare(txn.Hash(), common.Hash{}, counter)
		snap := p.state.Snapshot()
		receipt, _, err := blockchain.ApplyTransaction(p.blockchain, nil, p.gasPool, p.state, p.header, txn, usedGas, vm.Config{})
		if err != nil {
			p.state.RevertToSnapshot(snap)

			// TODO: check error type and jump to next txn if possible
			return nil, 0, err
		}
		counter++
		receipts = append(receipts, receipt)
	}

	p.receipts = &receipts
	return receipts, *usedGas, nil
}
