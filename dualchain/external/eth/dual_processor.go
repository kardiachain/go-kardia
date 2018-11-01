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

package dual

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
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/service"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	kardiabc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"
)

type DualProcessor struct {
	// TODO(namdoh): Remove reference to kardiaBc, only Kardia's TxPool is needed here.
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
}

func NewDualProcessor(kardiaBc *kardiabc.BlockChain, txPool *kardiabc.TxPool, dualBc *dualbc.DualBlockChain, dualEventPool *dualbc.EventPool, smcAddr *common.Address, smcABIStr string) (*DualProcessor, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &DualProcessor{
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		smcAddress: smcAddr,
		smcABI:     &smcABI,

		chainHeadCh: make(chan kardiabc.ChainHeadEvent, 5),
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *DualProcessor) Start() {
	// Start event loop
	go p.loop()
}

func (p *DualProcessor) loop() {
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

func (p *DualProcessor) handleBlock(block *types.Block) {
	smcUpdate := false
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			eventSummary, err := p.ExtractKardiaTxSummary(tx)
			if err != nil {
				log.Error("Error when extracting Kardia's tx summary.")
				// TODO(#140): Handle smart contract failure correctly.
				panic("Not yet implemented!")
			}
			log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value", eventSummary.TxValue, "hash", tx.Hash().Fingerprint())

			// New tx that updates smc, check input method for more filter.
			if eventSummary.TxMethod == "removeNeo" {
				// Not set flag here. If the block contains only the removeEth/removeNeo, skip look up the amount to avoid infinite loop.
				log.Info("Skip tx updating smc to remove Eth/Neo", "method", eventSummary.TxMethod)
				continue
			}

			smcUpdate = true
		}
	}

	// TODO(namdoh): Remove everything below here once Neo's code path is refactored. Currently it
	// is kept to not break the existing Neo's flow.
	if !smcUpdate {
		return
	}
	log.Info("Detect smc update, running VM call to check sending value")

	statedb, err := p.kardiaBc.StateAt(block.Root())
	if err != nil {
		log.Error("Error getting block state in dual process", "height", block.Height())
		return
	}

	// Neo dual node
	senderAddr := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
	neoSendValue := p.CallKardiaMasterGetNeoToSend(senderAddr, statedb)
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
			go p.ReleaseNeo(dev.NeoReceiverAddress, big.NewInt(convertedAmount.IntPart()))
			// Create Kardia tx removeNeo to acknowledge the neosend, otherwise getEthToSend will keep return >0
			gAccount := "0xBA30505351c17F4c818d94a990eDeD95e166474b"
			addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[gAccount])
			addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)

			tx := service.CreateKardiaRemoveAmountTx(addrKey, statedb, neoSendValue, 2)
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
func (p *DualProcessor) CallKardiaMasterGetNeoToSend(from common.Address, statedb *state.StateDB) *big.Int {
	getNeoToSend, err := p.smcABI.Pack("getNeoToSend")
	if err != nil {
		log.Error("Fail to pack Kardia smc getEthToSend", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}
	ret, err := callStaticKardiaMasterSmc(from, *p.smcAddress, p.kardiaBc, getNeoToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(ret)
}

func (p *DualProcessor) ExtractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
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

// Call Api to release Neo
func CallReleaseNeo(address string, amount *big.Int) (string, error) {
	body := []byte(`{
  "jsonrpc": "2.0",
  "method": "dual_sendeth",
  "params": ["` + address + `",` + amount.String() + `],
  "id": 1
}`)
	log.Info("Release neo", "message", string(body), "neodual", "neodual")
	var submitUrl string
	if dev.IsUsingNeoTestNet {
		submitUrl = dev.TestnetNeoSubmitUrl
	} else {
		submitUrl = dev.NeoSubmitTxUrl
	}
	rs, err := http.Post(submitUrl, "application/json", bytes.NewBuffer(body))
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
func checkTxNeo(txid string) bool {
	log.Info("Checking tx id", "txid", txid)
	var checkTxUrl string
	if dev.IsUsingNeoTestNet {
		checkTxUrl = dev.TestnetNeoCheckTxUrl
	} else {
		checkTxUrl = dev.NeoCheckTxUrl
	}
	url := checkTxUrl + txid
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
func retryTx(address string, amount *big.Int) {
	attempt := 0
	interval := 30
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount, "neodual", "neodual")
		txid, err := CallReleaseNeo(address, amount)
		if err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid, "neodual", "neodual")
			result := loopCheckingTx(txid)
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
func loopCheckingTx(txid string) bool {
	attempt := 0
	for {
		time.Sleep(10 * time.Second)
		attempt++
		success := checkTxNeo(txid)
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

func (p *DualProcessor) ReleaseNeo(address string, amount *big.Int) {
	log.Info("Release: ", "amount", amount, "address", address, "neodual", "neodual")
	txid, err := CallReleaseNeo(address, amount)
	if err != nil {
		log.Error("Error calling rpc", "err", err, "neodual", "neodual")
	}
	log.Info("Tx submitted", "txid", txid, "neodual", "neodual")
	if txid == "fail" || txid == "" {
		log.Info("Failed to release, retry tx", "txid", txid)
		retryTx(address, amount)
	} else {
		txStatus := loopCheckingTx(txid)
		if !txStatus {
			retryTx(address, amount)
		}
	}
}
