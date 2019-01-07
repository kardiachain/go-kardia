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
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
)

const GenerateTxInterval = 60 * time.Second
const CheckTxInterval = 10 * time.Second
const MaximumCheckTxAttempts = 10
const MaximumSubmitTxAttempts = 2
const CheckTxIntervalDelta = 15
const InitialCheckTxInterval = 30
var (
	OneNeo                = big.NewInt(0).Exp(big.NewInt(10), big.NewInt(18), nil) // 10**18 is 1 neo in exchange smc
	errNoNeoToSend        = errors.New("not enough NEO to send")
	errEnoughAvailableNeo = errors.New("enough available NEO, no need to add more")
	errNilReturnedFromNeo = errors.New("Neo API return nil")
	errRetryFailed		  = errors.New("exceeding maximum retry attempts but still failed")
	MaximumAvailableNeo   = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(19), nil)
)

// NeoProxy provides interfaces for Neo Dual node, responsible for detecting updates
// that relates to NEO from Kardia, sending tx to NEO network and check tx status
type NeoProxy struct {
	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool // Event pool of DUAL service.

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for new blocks.
	chainHeadCh  chan base.ChainHeadEvent
	chainHeadSub event.Subscription

	// Neo related URLs and address
	// TODO(sontranrad): these 3 values are stricly temporary and will be remove in future
	submitTxUrl        string
	checkTxUrl         string
	neoReceiverAddress string
	generateTx         bool
}

func NewNeoProxy(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain,
	dualEventPool *event_pool.EventPool, smcAddr *common.Address, smcABIStr string,
	submitTxUrl string, checkTxUrl string, neoReceiverAdd string, generateTx bool) (*NeoProxy, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &NeoProxy{
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		smcAddress: smcAddr,
		smcABI:     &smcABI,

		chainHeadCh: make(chan base.ChainHeadEvent, 5),

		submitTxUrl:        submitTxUrl,
		checkTxUrl:         checkTxUrl,
		neoReceiverAddress: neoReceiverAdd,
		generateTx:         generateTx,
	}
	if generateTx {
		// go GenerateMatchingTx(processor)
		go LoopFulfillRequest(processor)
	}

	return processor, nil
}

// LoopFulfillRequest loops to fulfill NEO-ETH order to make sure always matching for order from ETH with value 0.1 -> 1 ETH
func LoopFulfillRequest(n *NeoProxy) {
	errorCount := 0
	for {
		time.Sleep(GenerateTxInterval)
		statedb, err := n.kardiaBc.State()
		if err != nil {
			continue
		}

		availableAmountNeo, err := kardia.CallAvailableAmount(kardia.NEO2ETH, n.kardiaBc, statedb)
		if err != nil {
			continue
		}
		// If there is enough available matchable NEO amount, stop filling and return
		if availableAmountNeo.Cmp(MaximumAvailableNeo) > 0 {
			log.Info("NEO amount is enough")
			continue
		}
		for i := 1; i < 10; i++ {
			err := n.fillMissingOrder(i)
			if err != nil {
				errorCount++
				log.Error("Error fulfill available NEO", "err", err, "count", errorCount)
				break
			}
			// We try to add many 1-neo orders for testing and monitoring purpose
			err = n.fillMissingOrder(1)
			if err != nil {
				errorCount++
				log.Error("Error fulfill available NEO", "err", err, "count", errorCount)
				break
			}
		}
	}
}

func (n *NeoProxy) AddEvent(dualEvent *types.DualEvent) error {
	return n.eventPool.AddEvent(dualEvent)
}

func (n *NeoProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	n.internalChain = internalChain
}

// SubmitTx submit corresponding tx to NEO or Kardia basing on Data in EventData, include release NEO
// and upgrade Kardia smart contract. In case of matching event, we find the matched request here to release NEO to.
// In case of request from NEO -> ETH is completed, we check if its matched one is completed yet, if not then release
// Neo to it. Return error in case of invalid EventData
func (n *NeoProxy) SubmitTx(event *types.EventData) error {
	if len(event.Data.ExtData) > 3 {
		log.Info("submitting neo tx", "pair", string(event.Data.ExtData[3]))
	}
	statedb, err := n.kardiaBc.State()
	if err != nil {
		log.Error("Cannot get kardia statedb", "err", err)
		return kardia.ErrFailedGetState
	}
	switch event.Data.TxMethod {
	case kardia.MatchFunction:
		if len(event.Data.ExtData) != kardia.NumOfExchangeDataField {
			return kardia.ErrInsufficientExchangeData
		}
		// get matched request if any and submit it. we're only interested with those have dest pair NEO-ETH
		log.Info("detect matching request", "destPair", string(event.Data.ExtData[kardia.ExchangeDataDestPairIndex]),
			"srcPair", string(event.Data.ExtData[kardia.ExchangeDataSourcePairIndex]),
			"from", string(event.Data.ExtData[kardia.ExchangeDataSourceAddressIndex]), "to",
			string(event.Data.ExtData[kardia.ExchangeDataDestAddressIndex]))
		senderAddr  := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
		amount      := big.NewInt(0).SetBytes(event.Data.ExtData[kardia.ExchangeDataAmountIndex])
		srcAddress  := string(event.Data.ExtData[kardia.ExchangeDataSourceAddressIndex])
		destAddress := string(event.Data.ExtData[kardia.ExchangeDataDestAddressIndex])
		sourcePair  := string(event.Data.ExtData[kardia.ExchangeDataSourcePairIndex])
		request, err := kardia.CallKardiaGetMatchedRequest(senderAddr, n.kardiaBc, statedb, amount, srcAddress, destAddress,
			sourcePair, kardia.ETH2NEO)
		if err != nil {
			return err
		}
		// contract returns no matched request
		if request.DestAddress == "" {
			return kardia.ErrNoMatchedRequest
		}
		// divide by 10 ^ 18 here
		neoSendAmount := request.SendAmount.Div(request.SendAmount, OneNeo)
		// don't release  NEO if quantity < 1
		if neoSendAmount.Cmp(big.NewInt(1)) < 0 {
			return errNoNeoToSend
		}
		log.Info("there is a matching request, release neo")
		go n.releaseNeo(request.DestAddress, neoSendAmount, request.MatchedRequestID)
		err = n.completeRequest(request.MatchedRequestID)
		if err != nil {
			return err
		}
		return nil
	case kardia.CompleteFunction:
		if string(event.Data.ExtData[kardia.ExchangeDataCompletePairIndex]) != kardia.NEO2ETH {
			// The pair of completed request is not NEO -> ETH, so we just skip it and return nil
			log.Error("Invalid pair", "pair", string(event.Data.ExtData[kardia.ExchangeDataCompletePairIndex]))
			return nil
		}
		// there is a request from NEO -> ETH completed, we check whether its matched request (ETH->NEO) is complete yet
		// if no, release then complete it
		request, err := kardia.CallKardiaGetUncompletedRequest(big.NewInt(0).SetBytes(
			event.Data.ExtData[kardia.ExchangeDataCompleteRequestIDIndex]), statedb,
			n.smcABI, n.kardiaBc)
		if err != nil {
			log.Error("Error getting uncompleted request", "err", err)
			return err
		}
		// divide by 10 ^ 18 here
		neoSendAmount := request.SendAmount.Div(request.SendAmount, OneNeo)
		if request.DestAddress != "" && neoSendAmount.Cmp(big.NewInt(0)) == 1 {
			log.Info("there is an uncompleted matching request, release neo")
			go n.releaseNeo(request.DestAddress, neoSendAmount, request.MatchedRequestID)
			err = n.completeRequest(request.MatchedRequestID)
			if err != nil {
				return err
			}
			return nil
		}
		log.Info("No uncompleted request", "ID", event.Data.ExtData[0])
		return nil
	default:
		log.Warn("Unexpected method in NEO SubmitTx", "method", event.Data.TxMethod)
		return kardia.ErrUnsupportedMethod
	}
}

// completeRequest create tx call to Kardia smart contract to complete a request
func (n *NeoProxy) completeRequest(requestID *big.Int) error {
	tx, err := kardia.CreateKardiaCompleteRequestTx(n.txPool.State(), requestID, kardia.ETH2NEO)
	if err != nil {
		log.Error("Failed to create complete request tx", "ID", requestID, "direction",
			kardia.ETH2NEO)
		return err
	}
	err = n.txPool.AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Kardia tx to complete request", "err", err, "tx", tx)
		return err
	}
	log.Info("Submitted tx to Kardia to complete request successully", "txHash", tx.Hash().String())
	return nil
}

// In case it's an exchange event (matchOrder), we will calculate matching order later
// when we submitTx to externalChain, so I simply return a basic metadata here basing on target and event hash,
// to differentiate TxMetadata inferred from events
func (n *NeoProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}

func (n *NeoProxy) releaseNeo(address string, amount *big.Int, requestID *big.Int) {
	log.Info("Release: ", "amount", amount, "address", address, "requestID", requestID, "neodual", "neodual")
	txid, err := n.callReleaseNeo(address, amount, requestID)
	if err != nil {
		log.Error("Error calling rpc", "err", err, "neodual", "neodual")
		return
	}
	if txid == "fail" || txid == "" {
		log.Info("Failed to release, retry tx", "txid", txid)
		txid, err = n.retryTx(address, amount, requestID)
		if err != nil {
			log.Error("Error retrying release NEO", "err", err)
			return
		}
		log.Info("Submitted release NEO tx successfully", "txid", txid)
	}
	txStatus := n.loopCheckingTx(txid, requestID)
	if !txStatus {
		log.Error("Checking tx result neo failed", "txid", txid)
		return
	}
	log.Info("Tx release NEO is successful", "txid", txid)
}

// callReleaseNeo calls Api to release Neo
func (n *NeoProxy) callReleaseNeo(address string, amount *big.Int, id *big.Int) (string, error) {
	body := []byte(`{
  		"jsonrpc": "2.0",
  		"method": "dual_sendeth",
  		"params": ["` + address + `",` + amount.String() + `],
  		"id": 1
	}`)
	log.Info("Release neo", "message", string(body), "neodual", "neodual")

	rs, err := http.Post(n.submitTxUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	bytesRs, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return "", err
	}
	var f interface{}
	// Json decode result bytes then cast to map[string]interface
	json.Unmarshal(bytesRs, &f)
	if f == nil {
		return "", errNilReturnedFromNeo
	}
	m := f.(map[string]interface{})
	if m["result"] == nil {
		return "", errNilReturnedFromNeo
	}
	txid := m["result"].(string)
	log.Info("tx result neo", "txid", txid, "neodual", "neodual")
	return txid, nil
}

// Call Neo api to check status
// txid is "not found" : pending tx or tx is failed, need to loop checking
// to cover both case
func (n *NeoProxy) checkTxNeo(txid string) bool {
	log.Info("Checking tx id", "txid", txid)
	url := n.checkTxUrl + txid
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
	}
	log.Info("Checking tx result neo failed", "neodual", "neodual")
	return false
}

// retry send and loop checking tx status until it is successful
func (n *NeoProxy) retryTx(address string, amount *big.Int, requestID *big.Int) (string, error) {
	attempt := 0
	interval := InitialCheckTxInterval
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount, "neodual", "neodual")
		txid, err := n.callReleaseNeo(address, amount, requestID)
		if  err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid, "method", "retryTx", "neodual", "neodual")
			return txid, nil
		}
		attempt++
		// FIXME: (sontranrad) Currently we retry maximum 2 times then give up. Proper retry logic should be added later
		if attempt == MaximumSubmitTxAttempts {
			log.Info("Exceeded maximum submit attempts, give up now", "method", "retryTx", "neodual", "neodual",
				"maximum", MaximumSubmitTxAttempts)
			return "", errRetryFailed
		}
		sleepDuration := time.Duration(interval) * time.Second
		time.Sleep(sleepDuration)
		interval += CheckTxIntervalDelta
	}
}

func (n *NeoProxy) fillMissingOrder(i int) error {
	amount := big.NewInt(int64(0)).Mul(big.NewInt(int64(i)), OneNeo)
	tx, err := kardia.CreateKardiaMatchAmountTx(n.txPool.State(), amount,
		dev.NeoReceiverAddressList[0], dev.EthReceiverAddressList[1], types.NEO)
	if err != nil {
		return err
	}
	err = n.txPool.AddLocal(tx)
	if err != nil {
		return err
	}
	return nil
}

// Continually check tx status for 10 times, interval is 10 seconds
func (n *NeoProxy) loopCheckingTx(txid string, requestID *big.Int) bool {
	attempt := 0
	for {
		time.Sleep(CheckTxInterval)
		attempt++
		if n.checkTxNeo(txid) {
			log.Info("Tx is successful", "txid", txid, "neodual", "neodual")
			return true
		}
		if attempt == MaximumCheckTxAttempts {
			log.Info("Maximum check attempts reached, return false", "attempt", attempt, "neodual", "neodual")
			return false
		}
	}
}
