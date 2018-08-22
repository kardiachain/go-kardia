package blockchain

import (
	"errors"
	"fmt"
	"github.com/kardiachain/go-kardia/blockchain/rawdb"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	kaidb "github.com/kardiachain/go-kardia/storage"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

// Genesis specifies the header fields, state of a genesis block.
type Genesis struct {
	Config    *configs.ChainConfig `json:"config"`
	Timestamp uint64               `json:"timestamp"`
	GasLimit  uint64               `json:"gasLimit"   gencodec:"required"`
	Alloc     GenesisAlloc         `json:"alloc"      gencodec:"required"`

	// TODO(huny@): Add default validators?
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code    []byte                      `json:"code,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance *big.Int                    `json:"balance" gencodec:"required"`
	Nonce   uint64                      `json:"nonce,omitempty"`
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The returned chain configuration is never nil.
func SetupGenesisBlock(db kaidb.Database, genesis *Genesis) (*configs.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		// TODO(huny@): should we return another default config?
		return configs.TestnetChainConfig, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		log.Info("Create new genesis block")
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != configs.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	rawdb.WriteChainConfig(db, stored, newcfg)
	return newcfg, stored, nil
}

func (g *Genesis) configOrDefault(ghash common.Hash) *configs.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	case ghash == configs.MainnetGenesisHash:
		return configs.MainnetChainConfig
	case ghash == configs.TestnetGenesisHash:
		return configs.TestnetChainConfig
	default:
		return configs.TestnetChainConfig
	}
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (g *Genesis) ToBlock(db kaidb.Database) *types.Block {
	if db == nil {
		db = kaidb.NewMemStore()
	}
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))

	accountStates := make(types.AccountStates, 0)
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
		copyAddr := common.BytesToAddress(addr.Bytes())
		accountStates = append(accountStates, &types.BlockAccount{Addr: &copyAddr, Balance: account.Balance})
	}
	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		//@huny: convert timestamp here
		// Time:           g.Timestamp,
		GasLimit: g.GasLimit,
		Root:     root,
	}
	if g.GasLimit == 0 {
		head.GasLimit = configs.GenesisGasLimit
	}
	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	return types.NewBlock(head, nil, nil, &types.Commit{}, accountStates)
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db kaidb.Database) (*types.Block, error) {
	block := g.ToBlock(db)
	if block.Height() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with height > 0")
	}
	rawdb.WriteBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.Height(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.Height())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())

	config := g.Config
	if config == nil {
		config = configs.TestnetChainConfig
	}
	rawdb.WriteChainConfig(db, block.Hash(), config)

	return block, nil
}

// DefaultGenesisBlock returns the main net genesis block.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:   configs.MainnetChainConfig,
		GasLimit: 5000,
		//@huny Alloc:    decodePrealloc(mainnetAllocData),
	}
}

// DefaultTestnetGenesisBlock returns the test network genesis block from configs.
func DefaultTestnetGenesisBlock(allocData map[string]int64) *Genesis {

	ga, err := GenesisAllocFromData(allocData)
	if err != nil {
		return nil
	}

	return &Genesis{
		Config:   configs.TestnetChainConfig,
		GasLimit: 16777216,
		Alloc:    ga,
	}
}

func GenesisAllocFromData(data map[string]int64) (GenesisAlloc, error) {
	ga := make(GenesisAlloc, len(data))

	for address, balance := range data {
		ga[common.HexToAddress(address)] = GenesisAccount{Balance: big.NewInt(balance)}
	}

	return ga, nil
}
