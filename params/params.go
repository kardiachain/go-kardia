package params

const (
	GenesisGasLimit uint64 = 4712388 // Gas limit of the Genesis block.

	CallValueTransferGas uint64 = 9000  // Paid for CALL when the value transfer is non-zero.
	CallNewAccountGas    uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.

	QuadCoeffDiv    uint64 = 512   // Divisor for the quadratic particle of the memory cost equation.
	SstoreSetGas    uint64 = 20000 // Once per SLOAD operation.
	LogDataGas      uint64 = 8     // Per byte in a LOG* operation's data.
	Sha3Gas         uint64 = 30    // Once per SHA3 operation.
	Sha3WordGas     uint64 = 6     // Once per word of the SHA3 operation's data.
	SstoreResetGas  uint64 = 5000  // Once per SSTORE operation if the zeroness changes from zero.
	SstoreClearGas  uint64 = 5000  // Once per SSTORE operation if the zeroness doesn't change.
	SstoreRefundGas uint64 = 15000 // Once per SSTORE operation if the zeroness changes to zero.
	MemoryGas       uint64 = 3     // Times the address of the (highest referenced byte in memory + 1). NOTE: referencing happens on read, write and in instructions such as RETURN and CALL.
	CopyGas         uint64 = 3     //
	StackLimit      uint64 = 1024  // Maximum size of VM stack allowed.
	LogGas          uint64 = 375   // Per LOG* operation.
	LogTopicGas     uint64 = 375   // Multiplied by the * of the LOG*, per LOG transaction. e.g. LOG0 incurs 0 * c_txLogTopicGas, LOG4 incurs 4 * c_txLogTopicGas.
	CreateGas       uint64 = 32000 // Once per CREATE operation & contract-creation transaction.

	// Precompiled contract gas prices
	EcrecoverGas        uint64 = 3000 // Elliptic curve sender recovery gas price
	Sha256BaseGas       uint64 = 60   // Base price for a SHA256 operation
	Sha256PerWordGas    uint64 = 12   // Per-word price for a SHA256 operation
	Ripemd160BaseGas    uint64 = 600  // Base price for a RIPEMD160 operation
	Ripemd160PerWordGas uint64 = 120  // Per-word price for a RIPEMD160 operation
	IdentityBaseGas     uint64 = 15   // Base price for a data copy operation
	IdentityPerWordGas  uint64 = 3    // Per-work price for a data copy operation
)
