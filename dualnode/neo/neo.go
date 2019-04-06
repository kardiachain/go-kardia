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
	"strconv"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

const GenerateTxInterval = 60 * time.Second
const CheckTxInterval = 10 * time.Second
const MaximumCheckTxAttempts = 10
const MaximumSubmitTxAttempts = 2
const CheckTxIntervalDelta = 15
const InitialCheckTxInterval = 30

var (
	errNoNeoToSend        = errors.New("not enough NEO to send")
	errNilReturnedFromNeo = errors.New("Neo API return nil")
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
		// go LoopFulfillRequest(processor)
	}

	return processor, nil
}

func (n *NeoProxy) AddEvent(dualEvent *types.DualEvent) error {
	return n.eventPool.AddEvent(dualEvent)
}

func (n *NeoProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	n.internalChain = internalChain
}

// SubmitTx submit corresponding tx to NEO or Kardia basing on Data in EventData, include release NEO
// and upgrade Kardia smart contract. In case of matching event, we find the matched request here to release NEO to.
func (n *NeoProxy) SubmitTx(event *types.EventData) error {
	if len(event.Data.ExtData) > 3 {
		log.Info("Submitting neo tx", "pair", string(event.Data.ExtData[3]))
	}
	statedb, err := n.kardiaBc.State()
	if err != nil {
		log.Error("Cannot get kardia statedb", "err", err)
		return configs.ErrFailedGetState
	}
	switch event.Data.TxMethod {
	case configs.AddOrderFunction:
		if len(event.Data.ExtData) != configs.ExchangeV2NumOfExchangeDataField {
			return configs.ErrInsufficientExchangeData
		}
		senderAddr := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
		originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
		// Get all matched orders of newly added order
		releases, err := utils.CallKardiGetMatchingResultByTxId(senderAddr, n.kardiaBc, statedb, originalTx)
		if err != nil {
			return err
		}
		log.Info("Release info", "release", releases)
		if releases != "" {
			fields := strings.Split(releases, configs.ExchangeV2ReleaseFieldsSeparator)
			if len(fields) != 4 {
				log.Error("Invalid numn of field", "release", releases)
				return errors.New("Invalid num of field for release")
			}
			// Parse release info into arrays of types, addresses, amounts and txids
			arrTypes := strings.Split(fields[configs.ExchangeV2ReleaseToTypeIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAddresses := strings.Split(fields[configs.ExchangeV2ReleaseAddressesIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAmounts := strings.Split(fields[configs.ExchangeV2ReleaseAmountsIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrTxIds := strings.Split(fields[configs.ExchangeV2ReleaseTxIdsIndex], configs.ExchangeV2ReleaseValuesSepatator)

			for i, t := range arrTypes {
				if t == configs.NEO { // If the receiving end if the release is NEO, we release NEO
					if arrAmounts[i] == "" || arrAddresses[i] == "" || arrTxIds[i] == "" {
						log.Error("Missing release info", "matchedTxId", arrTxIds[i], "field", i, "releases", releases)
						continue
					}
					address := arrAddresses[i]
					amount, err1 := strconv.ParseInt(arrAmounts[i], 10, 64) //big.NewInt(0).SetString(arrAmounts[i], 10)
					log.Info("Amount", "amount", amount, "in string", arrAmounts[i])
					if err1 != nil {
						log.Error("Error parse amount", "amount", arrAmounts[i])
						continue
					}
					// Divide amount from smart contract by 10^8 to get base NEO amount to release
					amountNeoRelease := big.NewInt(amount).Div(big.NewInt(amount), utils.TenPoweredByEight)
					// don't release  NEO if quantity < 1
					if amountNeoRelease.Cmp(big.NewInt(1)) < 0 {
						log.Error("Too little neo to send", "originalTxId", originalTx, "err", errNoNeoToSend, "amount", amountNeoRelease)
						return nil
					}
					go n.releaseNeo(address, amountNeoRelease, arrTxIds[i])
				}
			}

			return nil
		}
		return nil
	default:
		log.Warn("Unexpected method in NEO SubmitTx", "method", event.Data.TxMethod)
		return configs.ErrUnsupportedMethod
	}
}

// completeRequest create tx call to Kardia smart contract to complete a request
func (n *NeoProxy) completeRequest(originalTxId string, releaseTxId string) error {
	// Update targetTx to mark that the originalTx is matched and released successfully
	tx, err := utils.UpdateKardiaTargetTx(n.txPool.State(), originalTxId, releaseTxId, "target")
	if err != nil {
		log.Error("Failed to update target tx", "originalTx", originalTxId, "tx", releaseTxId, "err", err)
		return err
	}
	err = n.txPool.AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Kardia tx to update target tx", "originalTx", originalTxId, "tx", releaseTxId, "err", err)
		return err
	}
	log.Info("Submitted tx to Kardia to update target tx successully", "txHash", tx.Hash().String(),
		"releaseTxId", releaseTxId, "originalTxId", originalTxId)
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

func (n *NeoProxy) releaseNeo(address string, amount *big.Int, matchedTxId string) {
	log.Info("Release: ", "amount", amount, "address", address, "matchedTxId", matchedTxId, "neodual", "neodual")
	releaseTxId, err := n.callReleaseNeo(address, amount)
	if err != nil {
		log.Error("Error calling rpc", "err", err, "neodual", "neodual")
		return
	}
	if releaseTxId == "fail" || releaseTxId == "" {
		log.Info("Failed to release, retry tx", "releaseTxId", releaseTxId)
		releaseTxId, err = n.retryTx(address, amount)
		if err != nil {
			log.Error("Error retrying release NEO", "err", err)
			return
		}
		log.Info("Submitted release NEO tx successfully", "releaseTxId", releaseTxId)
	}
	txStatus := n.loopCheckingTx(releaseTxId)
	if !txStatus {
		log.Error("Checking tx result neo failed", "releaseTxId", releaseTxId)
		return
	}
	log.Info("Tx release NEO is successful", "releaseTxId", releaseTxId)
	err = n.completeRequest(matchedTxId, releaseTxId)
	if err != nil {
		log.Error("Error complete request", "matchedTxId", matchedTxId, "releaseTxId", releaseTxId, "err", err)
	}
}

// callReleaseNeo calls Api to release Neo
func (n *NeoProxy) callReleaseNeo(address string, amount *big.Int) (string, error) {
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

// retryTx sends and loops checking tx status until it is successful
func (n *NeoProxy) retryTx(address string, amount *big.Int) (string, error) {
	attempt := 0
	interval := InitialCheckTxInterval
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount, "attempt", attempt, "neodual", "neodual")
		txid, err := n.callReleaseNeo(address, amount)
		if err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid, "method", "retryTx", "neodual", "neodual")
			return txid, nil
		}
		attempt++
		sleepDuration := time.Duration(interval) * time.Second
		time.Sleep(sleepDuration)
		interval += CheckTxIntervalDelta
	}
}

// loopCheckingTx periodically checks tx status for 10 times, interval is 10 seconds
func (n *NeoProxy) loopCheckingTx(txid string) bool {
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
