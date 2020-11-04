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

/*
Genesis confg {Node:{P2P:{ListenAddress:tcp://0.0.0.0:3000 PrivateKey:8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06} LogLevel:info Name:node1 DataDir:/tmp/.kardia HTTPHost:0.0.0.0 HTTPPort:8545 HTTPModules:[node kai tx account dual neo] HTTPVirtualHosts:[0.0.0.0 localhost] HTTPCors:[*] Metrics:1 Genesis:0xc000220fc0} MainChain:0xc000190fc0 DualChain:<nil>}
Mainchian confg &{ServiceName:KARDIA Protocol:<nil> ChainID:0 NetworkID:0 AcceptTxs:1 ZeroFee:0 IsDual:0 Genesis:0xc000220fc0 EventPool:<nil> Database:0xc000613f20 Seeds:[c1fe56e3f58d3244f606306611a5d10c8333f1f6@127.0.0.1:3000 7cefc13b6e2aedeedfb7cb6c32457240746baee5@127.0.0.1:3001 ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd@127.0.0.1:3002] Events:[] PublishedEndpoint:<nil> SubscribedEndpoint:<nil> BaseAccount:{Address: PrivateKey:} Consensus:<nil>}
*/

package configs

import "errors"

// All const related to cross-chain demos including coin exchange and candidate exchange
// this will be dynamic and removed when run on production
const (
	// constants related to account to call smc
	KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"
	KardiaPrivKeyToCallSmc = "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7"

	// constants related to rate & addOrder function from smc
	KAI = "KAI"
	ETH = "ETH"
	NEO = "NEO"

	// constants related to candidate exchange, Kardia part
	KardiaCandidateExchangeSmcAddress     = "0x00000000000000000000000000000000736D6338"
	KardiaPrivateChainCandidateSmcAddress = "0x00000000000000000000000000000000736D6337"
	KardiaPermissionSmcAddress            = "0x00000000000000000000000000000000736D6336"
	KardiaForwardRequestFunction          = "forwardRequest"
	KardiaForwardResponseFunction         = "forwardResponse"
	KardiaForwardResponseFields           = 4
	KardiaForwardResponseEmailIndex       = 0
	KardiaForwardResponseResponseIndex    = 1
	KardiaForwardResponseFromOrgIndex     = 2
	KardiaForwardResponseToOrgIndex       = 3
	KardiaForwardRequestFields            = 3
	KardiaForwardRequestEmailIndex        = 0
	KardiaForwardRequestFromOrgIndex      = 1
	KardiaForwardRequestToOrgIndex        = 2

	// constants related to candidate exchange, private chain part
	PrivateChainCandidateDBSmcIndex                     = 5
	PrivateChainCandidateRequestCompletedFields         = 4
	PrivateChainCandidateRequestCompletedFromOrgIDIndex = 0
	PrivateChainCandidateRequestCompletedToOrgIDIndex   = 1
	PrivateChainCandidateRequestCompletedEmailIndex     = 2
	PrivateChainCandidateRequestCompletedContentIndex   = 3
	PrivateChainRequestInfoFunction                     = "requestCandidateInfo"
	PrivateChainCompleteRequestFunction                 = "completeRequest"
	PrivateChainCandidateRequestFields                  = 3
	PrivateChainCandidateRequestEmailIndex              = 0
	PrivateChainCandidateRequestFromOrgIndex            = 1
	PrivateChainCandidateRequestToOrgIndex              = 2

	// default value for 0mq
	DefaultSubscribedEndpoint = "tcp://127.0.0.1:5555"
	DefaultPublishedEndpoint  = "tcp://127.0.0.1:5554"
)

var (
	ErrUnsupportedMethod = errors.New("method is not supported by dual logic")
)
