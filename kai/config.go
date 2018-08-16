package kai

import (
	"github.com/kardiachain/go-kardia/blockchain"
)

// DefaultConfig contains default settings for use on the Kardia main net.
var DefaultConfig = Config{

	NetworkId: 1,

	TxPool: blockchain.DefaultTxPoolConfig,
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// Protocol options
	NetworkId uint64 // Network

	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ethereum main net block is used.
	Genesis *blockchain.Genesis `toml:",omitempty"`

	// Transaction pool options
	TxPool blockchain.TxPoolConfig

	// chaindata
	ChainData string

	// DB caches
	DbCaches int

	// DB handles
	DbHandles int
}
