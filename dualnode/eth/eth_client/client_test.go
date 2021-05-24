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

package main

import (
	"fmt"
	"strings"
	"testing"

	abi2 "github.com/ethereum/go-ethereum/accounts/abi"
	message2 "github.com/kardiachain/go-kardia/dualnode/message"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/stretchr/testify/require"
)

const (
	data           = `0x7a9b486d000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000224159664b3478684a69616f7a546a616359546b72444439684a6770627571616a796300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034e454f0000000000000000000000000000000000000000000000000000000000`
	expectedMethod = "deposit"
	expectedArgs1  = "AYfK4xhJiaozTjacYTkrDD9hJgpbuqajyc"
	expectedArgs2  = "NEO"
	EthExchangeAbi = `[{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"release","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"constant":false,"inputs":[{"name":"receiver","type":"string"},{"name":"destination","type":"string"}],"name":"deposit","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},
{"constant":true,"inputs":[{"name":"destination","type":"string"}],"name":"isValidType","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"},
{"constant":false,"inputs":[{"name":"_type","type":"string"},{"name":"status","type":"bool"}],"name":"updateAvailableType","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"inputs":[{"name":"_owner","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`
)

func TestGetMethodAndParams(t *testing.T) {
	t.SkipNow()
	abi, err := abi2.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		t.Fatal(err)
	}
	contractData, err := common.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	method, params := GetMethodAndParams(abi, contractData)

	if method != expectedMethod {
		t.Fatal("mismatch method name")
	}

	if len(params) != 2 {
		t.Fatal("incorrect params")
	}

	if params[0] != expectedArgs1 || params[1] != expectedArgs2 {
		t.Fatal("mismatch params")
	}

	println(fmt.Sprintf("method %v and params %v", method, params))
}

func TestGetMessageToSendDualMessage(t *testing.T) {
	message := message2.Message{
		TransactionId:   "0x00",
		ContractAddress: "0x00",
		BlockNumber:     123,
		Sender:          "0x00",
		Amount:          1000,
		Timestamp:       getCurrentTimeStamp(),
		MethodName:      "testMethod",
		Params:          make([]string, 0),
	}
	msg, topic, err := GetMessageToSend(message)
	require.NoError(t, err)
	require.EqualValues(t, utils.DUAL_MSG, topic)
	println(msg)
}

func TestGetMessageToSendWithCallBack(t *testing.T) {
	message := message2.TriggerMessage{
		ContractAddress: "0x00",
		Params:          []string{},
		MethodName:      "just_test",
		CallBacks: []*message2.TriggerMessage{
			{
				MethodName: "callback1",
				Params:     []string{},
			},
		},
	}
	cb := message.CallBacks[0]
	msg, topic, err := GetMessageToSend(*cb)
	require.NoError(t, err)
	require.EqualValues(t, utils.DUAL_CALL, topic)
	println(msg)
}
