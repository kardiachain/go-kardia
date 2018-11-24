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
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	kardiabc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"
)

const KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"

var errNoNeoToSend = errors.New("Not enough NEO to send")

// NeoProxy provides interfaces for Neo Dual node, responsible for detecting updates
// that relates to NEO from Kardia, sending tx to NEO network and check tx status
type NeoProxy struct {
	kardiaBc   *kardiabc.BlockChain
	txPool     *kardiabc.TxPool
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    *dualbc.DualBlockChain
	eventPool *dualbc.EventPool // Event pool of DUAL service.

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain dualbc.BlockChainAdapter

	// Chain head subscription for new blocks.
	chainHeadCh  chan kardiabc.ChainHeadEvent
	chainHeadSub event.Subscription

	// Neo related URLs and address
	// TODO(sontranrad): these 3 values are stricly temporary and will be remove in future
	submitTxUrl        string
	checkTxUrl         string
	neoReceiverAddress string
}

func NewNeoProxy(kardiaBc *kardiabc.BlockChain, txPool *kardiabc.TxPool, dualBc *dualbc.DualBlockChain,
	dualEventPool *dualbc.EventPool, smcAddr *common.Address, smcABIStr string,
	submitTxUrl string, checkTxUrl string, neoReceiverAdd string) (*NeoProxy, error) {
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

		chainHeadCh: make(chan kardiabc.ChainHeadEvent, 5),

		submitTxUrl:        submitTxUrl,
		checkTxUrl:         checkTxUrl,
		neoReceiverAddress: neoReceiverAdd,
	}

	return processor, nil
}

func (n *NeoProxy) AddEvent(dualEvent *types.DualEvent) error {
	return n.eventPool.AddEvent(dualEvent)
}

func (n *NeoProxy) RegisterInternalChain(internalChain dualbc.BlockChainAdapter) {
	n.internalChain = internalChain
}

func (n *NeoProxy) SubmitTx(event *types.EventData) error {
	statedb, err := n.kardiaBc.State()
	if err != nil {
		log.Error("Fail to get Kardia state", "error", err)
		return err
	}
	senderAddr := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
	neoSendValue := n.callKardiaMasterGetNeoToSend(senderAddr, statedb)
	if neoSendValue != nil && neoSendValue.Cmp(big.NewInt(0)) != 0 {
		return errNoNeoToSend
	}
	log.Info("Kardia smc calls getNeoToSend", "neo", neoSendValue)
	amountToRelease := decimal.NewFromBigInt(neoSendValue, 0)
	log.Info("Original amount neo to release", "amount", amountToRelease, "neodual", "neodual")
	// temporarily hard code for the exchange rate, 1 ETH = 10 NEO
	convertedAmount := amountToRelease.Mul(decimal.NewFromBigInt(big.NewInt(10), 0))
	log.Info("Converted amount to release", "converted", convertedAmount, "neodual", "neodual")
	if convertedAmount.LessThan(decimal.NewFromFloat(1.0)) {
		return errors.New("Neo amount to send should be more than 1")
	}
	log.Info("Sending to neo", "amount", convertedAmount, "neodual", "neodual")
	go n.releaseNeo(n.neoReceiverAddress, big.NewInt(convertedAmount.IntPart()))

	// Create Kardia tx removeNeo to acknowledge the neosend, otherwise getEthToSend will keep return > 0
	gAccount := KardiaAccountToCallSmc
	addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[gAccount])
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)

	tx := kardia.CreateKardiaRemoveAmountTx(addrKey, statedb, neoSendValue, types.NEO)
	if err := n.txPool.AddLocal(tx); err != nil {
		log.Error("Fail to add Kardia tx to removeNeo", err, "tx", tx, "neodual", "neodual")
		return err
	}
	log.Info("Creates removeNeo tx", tx.Hash().Hex(), "neodual", "neodual")

	return nil
}

func (n *NeoProxy) ComputeTxMetadata(event *types.EventData) *types.TxMetadata {
	// TODO(#216): because we cannot pre-compute txHash for NEO tx before submitting
	// as we do with ETH, I temporarily use event.Hash instead
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.NEO,
	}
}

// TODO(sontranrad): All hardcode function names below are temporarily used for exchange use case
// these should be cleaned up or turned into dynamic logic flow
func (n *NeoProxy) callKardiaMasterGetNeoToSend(from common.Address, statedb *state.StateDB) *big.Int {
	getNeoToSend, err := n.smcABI.Pack("getNeoToSend")
	if err != nil {
		log.Error("Fail to pack Kardia smc getEthToSend", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}
	ret, err := kardia.CallStaticKardiaMasterSmc(from, *n.smcAddress, n.kardiaBc, getNeoToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err, "neodual", "neodual")
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ret)
}

func (n *NeoProxy) releaseNeo(address string, amount *big.Int) {
	log.Info("Release: ", "amount", amount, "address", address, "neodual", "neodual")
	txid, err := n.callReleaseNeo(address, amount)
	if err != nil {
		log.Error("Error calling rpc", "err", err, "neodual", "neodual")
	}
	log.Info("Tx submitted", "txid", txid, "neodual", "neodual")
	if txid == "fail" || txid == "" {
		log.Info("Failed to release, retry tx", "txid", txid)
		n.retryTx(address, amount)
	} else {
		txStatus := n.loopCheckingTx(txid)
		if !txStatus {
			n.retryTx(address, amount)
		}
	}
}

// Call Api to release Neo
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
	} else {
		log.Info("Checking tx result neo failed", "neodual", "neodual")
		return false
	}
}

// retry send and loop checking tx status until it is successful
func (n *NeoProxy) retryTx(address string, amount *big.Int) {
	attempt := 0
	interval := 30
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount, "neodual", "neodual")
		txid, err := n.callReleaseNeo(address, amount)
		if err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid, "neodual", "neodual")
			result := n.loopCheckingTx(txid)
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
func (n *NeoProxy) loopCheckingTx(txid string) bool {
	attempt := 0
	for {
		time.Sleep(10 * time.Second)
		attempt++
		success := n.checkTxNeo(txid)
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
