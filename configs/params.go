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

const (
	GenesisGasLimit uint64 = 50000000  // Gas limit of the Genesis block.
	BlockGasLimit   uint64 = 200000000 // Gas limit of one block.
	BlockMaxBytes   int64  = 104857600 // Block max size bytes: 10mbs

	CallValueTransferGas  uint64 = 9000  // Paid for CALL when the value transfer is non-zero.
	CallNewAccountGas     uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.
	TxGas                 uint64 = 29000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxGasContractCreation uint64 = 53000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.

	MaximumExtraDataSize uint64 = 32   // Maximum size extra data may be after Genesis.
	ExpByteGas           uint64 = 10   // Times ceil(log256(exponent)) for the EXP instruction.
	SloadGas             uint64 = 50   // Multiplied by the number of 32-byte words that are copied (round up) for any *COPY operation and added.
	TxDataZeroGas        uint64 = 4    // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions.
	QuadCoeffDiv         uint64 = 512  // Divisor for the quadratic particle of the memory cost equation.
	LogDataGas           uint64 = 8    // Per byte in a LOG* operation's data.
	CallStipend          uint64 = 2300 // Free gas given at beginning of call.

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

	CallGas                 uint64 = 40    // Once per CALL operation & message call transaction.
	CallGasStatic           uint64 = 700   // Once per CALL operation & message call transaction.
	BalanceGas              uint64 = 400   // The cost of a BALANCE operation
	ExtcodeSizeGas          uint64 = 700   // Cost of EXTCODESIZE before EIP 150 (Tangerine)
	ExpByte                 uint64 = 50    // EXP has a dynamic portion depending on the size of the exponent
	ExtcodeCopyBase         uint64 = 700   // Extcodecopy has a dynamic AND a static cost. This represents only the static portion of the gas
	CreateBySelfdestructGas uint64 = 25000 // CreateBySelfdestructGas is used when the refunded account is one that does not exist. This logic is similar to call.
	ExtcodeHashGas          uint64 = 400   // Cost of EXTCODEHASH

	MaxCodeSize = 39231 // Maximum bytecode to permit for a contract

	// Precompiled contract gas prices
	EcrecoverGas        uint64 = 3000 // Elliptic curve sender recovery gas price
	Sha256BaseGas       uint64 = 60   // Base price for a SHA256 operation
	Sha256PerWordGas    uint64 = 12   // Per-word price for a SHA256 operation
	Ripemd160BaseGas    uint64 = 600  // Base price for a RIPEMD160 operation
	Ripemd160PerWordGas uint64 = 120  // Per-word price for a RIPEMD160 operation
	IdentityBaseGas     uint64 = 15   // Base price for a data copy operation
	IdentityPerWordGas  uint64 = 3    // Per-work price for a data copy operation
	ModExpQuadCoeffDiv  uint64 = 20   // Divisor for the quadratic particle of the big int modular exponentiation

	Bn256AddGas             uint64 = 500    // Byzantium gas needed for an elliptic curve addition
	Bn256ScalarMulGas       uint64 = 40000  // Byzantium gas needed for an elliptic curve scalar multiplication
	Bn256PairingBaseGas     uint64 = 100000 // Byzantium base price for an elliptic curve pairing check
	Bn256PairingPerPointGas uint64 = 80000  // Byzantium per-point price for an elliptic curve pairing check

	// Call Gas cost
	GasQuickStep   uint64 = 2
	GasFastestStep uint64 = 3
	GasFastStep    uint64 = 5
	GasMidStep     uint64 = 8
	GasSlowStep    uint64 = 10
	GasExtStep     uint64 = 20

	// Gas cap per transaction
	GasCap uint64 = 20000000

	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains on the server side.
	BloomBitsBlocks uint64 = 4096

	// BloomBitsBlocksClient is the number of blocks a single bloom bit section vector
	// contains on the light client side
	BloomBitsBlocksClient uint64 = 32768

	// HelperTrieConfirmations is the number of confirmations before a client is expected
	// to have the given HelperTrie available.
	HelperTrieConfirmations = 2048
)
