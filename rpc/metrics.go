/*
 *  Copyright 2021 KardiaChain
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

package rpc

import (
	"fmt"

	"github.com/kardiachain/go-kardia/lib/metrics"
)

func init() {
	for _, method := range kaiRPCMethods {
		metrics.NewRegisteredGauge(method, metrics.RPCRegistry)
	}
	for _, method := range ethRPCMethods {
		metrics.NewRegisteredGauge(method, metrics.RPCRegistry)
	}
}

var (
	rpcRequestGauge        = metrics.NewRegisteredGauge("rpc/requests", nil)
	successfulRequestGauge = metrics.NewRegisteredGauge("rpc/success", nil)
	failedReqeustGauge     = metrics.NewRegisteredGauge("rpc/failure", nil)
	rpcServingTimer        = metrics.NewRegisteredTimer("rpc/duration/all", nil)
)

func newRPCServingTimer(method string, valid bool) metrics.Timer {
	flag := "success"
	if !valid {
		flag = "failure"
	}
	m := fmt.Sprintf("rpc/duration/%s/%s", method, flag)
	return metrics.GetOrRegisterTimer(m, nil)
}

// pre-register types of RPC API call we want to track
var (
	kaiRPCMethods = []string{
		"kai_getBlockByHash",
		"kai_getBlockByNumber",
		"kai_blockNumber",
		"kai_kardiaCall",
		"account_nonce",
		"account_balance",
		"account_getCode",
		"account_getStorageAt",
		"kai_estimateGas",
		"tx_sendRawTransaction",
		"tx_getTransaction",
		"tx_getTransactionReceipt",
		"kai_gasPrice",

		// KAI only
		"kai_validator",
		"kai_validators",
		"kai_getValidatorSet",
		"tx_pendingTransactions",
		"debug_traceTransaction",
		"debug_traceCall",
	}
	ethRPCMethods = []string{
		"eth_getBlockByHash",
		"eth_getBlockByNumber",
		"eth_blockNumber",
		"eth_call",
		"eth_getTransactionCount",
		"eth_getBalance",
		"eth_getCode",
		"eth_getStorageAt",
		"eth_estimateGas",
		"eth_sendRawTransaction",
		"eth_getTransactionByHash",
		"eth_getTransactionReceipt",
		"eth_gasPrice",

		// ETH only
		"eth_chainId",
		"eth_accounts",
		"eth_getTransactionByBlockHashAndIndex",
		"eth_getTransactionByBlockNumberAndIndex",
	}
)
