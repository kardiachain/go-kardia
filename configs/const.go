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

const (
	GenesisGasLimit uint64 = 50000000 // Gas limit of the Genesis block.
	//BlockGasLimit   uint64 = 200000000 // Gas limit of one block.
	//BlockMaxBytes   int64  = 104857600 // Block max size bytes: 10mbs

	CallValueTransferGas  uint64 = 9000  // Paid for CALL when the value transfer is non-zero.
	CallNewAccountGas     uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.
	TxGas                 uint64 = 21000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxGasContractCreation uint64 = 53000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.

	//MaximumExtraDataSize uint64 = 32   // Maximum size extra data may be after Genesis.
	//ExpByteGas           uint64 = 10   // Times ceil(log256(exponent)) for the EXP instruction.
	SloadGas      uint64 = 50   // Multiplied by the number of 32-byte words that are copied (round up) for any *COPY operation and added.
	TxDataZeroGas uint64 = 4    // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions.
	QuadCoeffDiv  uint64 = 512  // Divisor for the quadratic particle of the memory cost equation.
	LogDataGas    uint64 = 8    // Per byte in a LOG* operation's data.
	CallStipend   uint64 = 2300 // Free gas given at beginning of call.

	Sha3Gas     uint64 = 30 // Once per SHA3 operation.
	Sha3WordGas uint64 = 6  // Once per word of the SHA3 operation's data.

	SstoreSetGas    uint64 = 20000 // Once per SLOAD operation.
	SstoreResetGas  uint64 = 5000  // Once per SSTORE operation if the zeroness changes from zero.
	SstoreClearGas  uint64 = 5000  // Once per SSTORE operation if the zeroness doesn't change.
	SstoreRefundGas uint64 = 15000 // Once per SSTORE operation if the zeroness changes to zero.

	JumpdestGas uint64 = 1 // Once per JUMPDEST operation.

	CreateDataGas         uint64 = 200   // Gas for creatding data
	CallCreateDepth       uint64 = 1024  // Maximum depth of call/create stack.
	ExpGas                uint64 = 10    // Once per EXP instruction
	LogGas                uint64 = 375   // Per LOG* operation.
	CopyGas               uint64 = 3     //
	StackLimit            uint64 = 1024  // Maximum size of VM stack allowed.
	TierStepGas           uint64 = 0     // Once per operation, for a selection of them.
	LogTopicGas           uint64 = 375   // Multiplied by the * of the LOG*, per LOG transaction. e.g. LOG0 incurs 0 * c_txLogTopicGas, LOG4 incurs 4 * c_txLogTopicGas.
	CreateGas             uint64 = 32000 // Once per CREATE operation & contract-creation transaction.      uint64 = 32000 // Once per CREATE2 operation
	CreateGas2            uint64 = 32000 // Once per CREATE2 operation
	SelfdestructRefundGas uint64 = 24000 // Refunded following a selfdestruct operation.
	MemoryGas             uint64 = 3     // Times the address of the (highest referenced byte in memory + 1). NOTE: referencing happens on read, write and in instructions such as RETURN and CALL.
	TxDataNonZeroGas      uint64 = 68    // Per byte of data attached to a transaction that is not equal to zero. NOTE: Not payable on data of calls between transactions.

	CallGas                 uint64 = 700 // Once per CALL operation & message call transaction.
	BalanceGas              uint64 = 400 // The cost of a BALANCE operation
	ExtcodeSizeGas          uint64 = 700 // Cost of EXTCODESIZE before EIP 150 (Tangerine)
	ExpByte                 uint64 = 50
	ExtcodeCopyBase         uint64 = 700
	CreateBySelfdestructGas uint64 = 5000
	ExtcodeHashGas          uint64 = 400 // Cost of EXTCODEHASH

	MaxCodeSize = 39231 // Maximum bytecode to permit for a contract

	// Precompiled contract gas prices
	EcrecoverGas        uint64 = 3000 // Elliptic curve sender recovery gas price
	Sha256BaseGas       uint64 = 60   // Base price for a SHA256 operation
	Sha256PerWordGas    uint64 = 12   // Per-word price for a SHA256 operation
	Ripemd160BaseGas    uint64 = 600  // Base price for a RIPEMD160 operation
	Ripemd160PerWordGas uint64 = 120  // Per-word price for a RIPEMD160 operation
	IdentityBaseGas     uint64 = 15   // Base price for a data copy operation
	IdentityPerWordGas  uint64 = 3    // Per-work price for a data copy operation
)
