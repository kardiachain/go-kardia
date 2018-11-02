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

package neo

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	kardiabc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"
	"fmt"
)

// The method from master contract that does not need to be handled
const SkippedMethod = "removeNeo"
const KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"

// DualNeo provides interfaces for Neo Dual node, responsible for detecting updates
// that relates to NEO from Kardia, sending tx to NEO network and check tx status
type DualNeo struct {
	kardiaBc   *kardiabc.BlockChain
	txPool     *kardiabc.TxPool
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    *dualbc.DualBlockChain
	eventPool *dualbc.EventPool // Event pool of DUAL service.

	// Chain head subscription for new blocks.
	chainHeadCh  chan kardiabc.ChainHeadEvent
	chainHeadSub event.Subscription

	// Neo related URLs and address
	submitTxUrl        string
	checkTxUrl         string
	neoReceiverAddress string
}

func NewDualNeo(kardiaBc *kardiabc.BlockChain, txPool *kardiabc.TxPool, dualBc *dualbc.DualBlockChain, dualEventPool *dualbc.EventPool, smcAddr *common.Address, smcABIStr string, ) (*DualNeo, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &DualNeo{
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		smcAddress: smcAddr,
		smcABI:     &smcABI,

		chainHeadCh: make(chan kardiabc.ChainHeadEvent, 5),

		submitTxUrl:        DefaultNeoConfig.SubmitTxUrl,
		checkTxUrl:         DefaultNeoConfig.CheckTxUrl,
		neoReceiverAddress: DefaultNeoConfig.ReceiverAddress,
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

// Set URL to submit tx to NEO
func (p *DualNeo) SetSubmitTxUrl(url string) {
	p.submitTxUrl = url
}

// Set URL to retrieve tx status from NEO
func (p *DualNeo) SetCheckTxUrl(url string) {
	p.checkTxUrl = url
}

// Set NEO address receiver
func (p *DualNeo) SetNeoReceiver(address string) {
	p.neoReceiverAddress = address
}

// Returns Neo related configs set to this instance
func (p *DualNeo) ReportConfig() string {
	return fmt.Sprintf("CheckTX URL : %v SubmitTX URL : %v Receiver: %v", p.checkTxUrl, p.submitTxUrl, p.neoReceiverAddress)
}

func (p *DualNeo) Start() {
	// Start event loop
	go p.loop()
}

func (p *DualNeo) loop() {
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

func (p *DualNeo) handleBlock(block *types.Block) {
	smcUpdate := false
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			eventSummary, err := p.extractKardiaTxSummary(tx)
			if err != nil {
				log.Error("Error when extracting Kardia's tx summary.")
				// TODO(#140): Handle smart contract failure correctly.
				panic("Not yet implemented!")
			}
			log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value", eventSummary.TxValue, "hash", tx.Hash().Fingerprint())

			// New tx that updates smc, check input method for more filter.
			if eventSummary.TxMethod == SkippedMethod {
				// Not set flag here. If the block contains only the removeEth/removeNeo, skip look up the amount to avoid infinite loop.
				log.Info("Skip tx updating smc to remove Neo", "method", eventSummary.TxMethod)
				continue
			}

			smcUpdate = true
		}
	}

	if !smcUpdate {
		return
	}
	log.Info("Detect smc update, running VM call to check sending value")

	statedb, err := p.kardiaBc.StateAt(block.Root())
	if err != nil {
		log.Error("Error getting block state in dual process", "height", block.Height())
		return
	}

	senderAddr := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
	neoSendValue := p.callKardiaMasterGetNeoToSend(senderAddr, statedb)
	log.Info("Kardia smc calls getNeoToSend", "neo", neoSendValue)
	if neoSendValue != nil && neoSendValue.Cmp(big.NewInt(0)) != 0 {
		// TODO: create new NEO tx to send NEO
		// Temporarily hard code the recipient
		amountToRelease := decimal.NewFromBigInt(neoSendValue, 10).Div(decimal.NewFromBigInt(common.BigPow(10, 18), 10))
		log.Info("Original amount neo to release", "amount", amountToRelease, "neodual", "neodual")
		convertedAmount := amountToRelease.Mul(decimal.NewFromBigInt(big.NewInt(10), 0))
		log.Info("Converted amount to release", "converted", convertedAmount, "neodual", "neodual")
		if convertedAmount.LessThan(decimal.NewFromFloat(1.0)) {
			log.Info("Too little amount to send", "amount", convertedAmount, "neodual", "neodual")
		} else {
			// temporarily hard code for the exchange rate
			log.Info("Sending to neo", "amount", convertedAmount, "neodual", "neodual")
			go p.releaseNeo(p.neoReceiverAddress, big.NewInt(convertedAmount.IntPart()))
			// Create Kardia tx removeNeo to acknowledge the neosend, otherwise getEthToSend will keep return >0
			gAccount := KardiaAccountToCallSmc
			addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[gAccount])
			addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)

			tx := kardia.CreateKardiaRemoveAmountTx(addrKey, statedb, neoSendValue, 2)
			if err := p.txPool.AddLocal(tx); err != nil {
				log.Error("Fail to add Kardia tx to removeNeo", err, "tx", tx, "neodual", "neodual")
			} else {
				log.Info("Creates removeNeo tx", tx.Hash().Hex(), "neodual", "neodual")
			}
		}
	}
}

// TODO(namdoh): Remove this function once Neo's code path is refactored. Currently it
// is kept to not break the existing Neo's flow.
func (p *DualNeo) callKardiaMasterGetNeoToSend(from common.Address, statedb *state.StateDB) *big.Int {
	getNeoToSend, err := p.smcABI.Pack("getNeoToSend")
	if err != nil {
		log.Error("Fail to pack Kardia smc getEthToSend", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}
	ret, err := kardia.CallStaticKardiaMasterSmc(from, *p.smcAddress, p.kardiaBc, getNeoToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(ret)
}

func (p *DualNeo) extractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
	// New tx that updates smc, check input method for more filter.
	method, err := p.smcABI.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}

	return types.EventSummary{
		TxMethod: method.Name,
		TxValue:  tx.Value(),
	}, nil
}

func (p *DualNeo) releaseNeo(address string, amount *big.Int) {
	log.Info("Release: ", "amount", amount, "address", address, "neodual", "neodual")
	txid, err := p.callReleaseNeo(address, amount)
	if err != nil {
		log.Error("Error calling rpc", "err", err, "neodual", "neodual")
	}
	log.Info("Tx submitted", "txid", txid, "neodual", "neodual")
	if txid == "fail" || txid == "" {
		log.Info("Failed to release, retry tx", "txid", txid)
		p.retryTx(address, amount)
	} else {
		txStatus := p.loopCheckingTx(txid)
		if !txStatus {
			p.retryTx(address, amount)
		}
	}
}

// Call Api to release Neo
func (p *DualNeo) callReleaseNeo(address string, amount *big.Int) (string, error) {
	body := []byte(`{
  "jsonrpc": "2.0",
  "method": "dual_sendeth",
  "params": ["` + address + `",` + amount.String() + `],
  "id": 1
}`)
	log.Info("Release neo", "message", string(body), "neodual", "neodual")

	rs, err := http.Post(p.submitTxUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	bytesRs, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return "", err
	}
	var f interface{}
	json.Unmarshal(bytesRs, &f)
	if f == nil {
		return "", errors.New("Nil return")
	}
	m := f.(map[string]interface{})
	var txid string
	if m["result"] == nil {
		return "", errors.New("Nil return")
	}
	txid = m["result"].(string)
	log.Info("tx result neo", "txid", txid, "neodual", "neodual")
	return txid, nil
}

// Call Neo api to check status
// txid is "not found" : pending tx or tx is failed, need to loop checking
// to cover both case
func (p *DualNeo) checkTxNeo(txid string) bool {
	log.Info("Checking tx id", "txid", txid)
	url := p.checkTxUrl + txid
	rs, err := http.Get(url)
	if err != nil {
		return false
	}
	bytesRs, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return false
	}
	var f interface{}
	json.Unmarshal(bytesRs, &f)

	if f == nil {
		return false
	}

	m := f.(map[string]interface{})

	if m["txid"] == nil {
		return false
	}

	txid = m["txid"].(string)
	if txid != "not found" {
		log.Info("Checking tx result neo", "txid", txid, "neodual", "neodual")
		return true
	} else {
		log.Info("Checking tx result neo failed", "neodual", "neodual")
		return false
	}
}

// retry send and loop checking tx status until it is successful
func (p *DualNeo) retryTx(address string, amount *big.Int) {
	attempt := 0
	interval := 30
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount, "neodual", "neodual")
		txid, err := p.callReleaseNeo(address, amount)
		if err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid, "neodual", "neodual")
			result := p.loopCheckingTx(txid)
			if result {
				log.Info("tx is successful", "neodual", "neodual")
				return
			} else {
				log.Info("tx is not successful, retry in 5 sconds", "txid", txid, "neodual", "neodual")
			}
		} else {
			log.Info("Posting tx failed, retry in 5 seconds", "txid", txid, "neodual", "neodual")
		}
		attempt++
		if attempt > 1 {
			log.Info("Trying 2 time but still fail, give up now", "txid", txid, "neodual", "neodual")
			return
		}
		sleepDuration := time.Duration(interval) * time.Second
		time.Sleep(sleepDuration)
		interval += 30
	}
}

// Continually check tx status for 10 times, interval is 10 seconds
func (p *DualNeo) loopCheckingTx(txid string) bool {
	attempt := 0
	for {
		time.Sleep(10 * time.Second)
		attempt++
		success := p.checkTxNeo(txid)
		if !success && attempt > 10 {
			log.Info("Tx fail, need to retry", "attempt", attempt, "neodual", "neodual")
			return false
		}

		if success {
			log.Info("Tx is successful", "txid", txid, "neodual", "neodual")
			return true
		}
	}
}
