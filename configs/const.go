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

// All const related to cross-chain demos including coin exchange and candidate exchange
// this will be dynamic and removed when run on production
const (
	// constants related to account to call smc
	KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"
	KardiaPrivKeyToCallSmc = "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7"

	// constants related to currency exchange
	KardiaNewExchangeSmcIndex          = 3
	ExternalDepositFunction            = "deposit"
	GetAddressFromType                 = "getAddressFromType"
	ExternalReleaseFunction            = "release"
	GetRate                            = "getRate"
	UpdateTargetTx                     = "updateTargetTx"

	// constants related to rate & addOrder function from smc
	KAI                                = "KAI"
	ETH                                = "ETH"
	NEO                                = "NEO"
	TRON                               = "TRX"
	AddOrderFunction                   = "addOrder"
	RateETH                            = 100000000
	RateNEO                            = 6482133
	RateTRON                           = 16571

	ExchangeDataSourceAddressIndex     = 0
	ExchangeDataDestAddressIndex       = 1
	ExchangeDataSourcePairIndex        = 2
	ExchangeDataDestPairIndex          = 3
	ExchangeDataAmountIndex            = 4
	ExchangeDataCompleteRequestIDIndex = 0
	ExchangeDataCompletePairIndex      = 1
	NumOfExchangeDataField             = 5
	NumOfCompleteRequestDataField      = 2

	// Constants related to exchange v2 which support original tx id
	ExchangeV2SourcePairIndex        = 0
	ExchangeV2DestPairIndex          = 1
	ExchangeV2SourceAddressIndex     = 2
	ExchangeV2DestAddressIndex       = 3
	ExchangeV2OriginalTxIdIndex      = 4
	ExchangeV2AmountIndex            = 5
	ExchangeV2TimestampIndex         = 6
	ExchangeV2NumOfExchangeDataField = 7
	ExchangeV2ReleaseFieldsSeparator = "|"
	ExchangeV2ReleaseToTypeIndex     = 0
	ExchangeV2ReleaseAddressesIndex  = 1
	ExchangeV2ReleaseAmountsIndex    = 2
	ExchangeV2ReleaseTxIdsIndex      = 3
	ExchangeV2ReleaseValuesSepatator = ";"

	// constants related to candidate exchange, Kardia part
	KardiaCandidateExchangeSmcIndex    = 6
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
	DefaultPublishedEndpoint = "tcp://127.0.0.1:5554"
)

var (
	ErrInsufficientExchangeData         = errors.New("insufficient exchange external data")
	ErrUnsupportedMethod                = errors.New("method is not supported by dual logic")
	ErrCreateKardiaTx                   = errors.New("fail to create Kardia's Tx from DualEvent")
	ErrAddKardiaTx                      = errors.New("fail to add Tx to Kardia's TxPool")
	ErrTypeConversionFailed             = errors.New("fail type conversion")
	ErrFailedGetState                   = errors.New("fail to get Kardia state")
	ErrInsufficientCandidateRequestData = errors.New("insufficient candidate request data")
	ErrFailedGetEventData               = errors.New("fail to get event external data")
	ErrNoMatchedRequest                 = errors.New("request has no matched opponent")
	ErrNotImplemented                   = errors.New("this function is not implemented yet")
)