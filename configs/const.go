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

package configs

import "errors"

const (
	// constants related to account to call smc
	KardiaPrivKeyToCallSmc = "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7"

	// constants related to rate & addOrder function from smc
	KAISymbol = "KAI"
	ETHSymbol = "ETH"
	NEOSymbol = "NEO"

	// constants related to candidate exchange, Kardia part
	KardiaCandidateExchangeSmcAddress  = "0x00000000000000000000000000000000736D6338"
	KardiaForwardRequestFunction       = "forwardRequest"
	KardiaForwardResponseFunction      = "forwardResponse"
	KardiaForwardResponseFields        = 4
	KardiaForwardResponseEmailIndex    = 0
	KardiaForwardResponseResponseIndex = 1
	KardiaForwardResponseFromOrgIndex  = 2
	KardiaForwardResponseToOrgIndex    = 3
	KardiaForwardRequestFields         = 3
	KardiaForwardRequestEmailIndex     = 0
	KardiaForwardRequestFromOrgIndex   = 1
	KardiaForwardRequestToOrgIndex     = 2

	// default value for 0mq
	DefaultSubscribedEndpoint = "tcp://127.0.0.1:5555"
	DefaultPublishedEndpoint  = "tcp://127.0.0.1:5554"

	// default params for blockchain APIs
	DefaultTimeOutForStaticCall = 60

	// default params for configs
	DefaultBcReactorServiceName = "BCR"

	// default networkID
	MainnetNetworkID = 0
	TestnetNetworkID = 69
)

var (
	ErrUnsupportedMethod = errors.New("method is not supported by dual logic")
)
