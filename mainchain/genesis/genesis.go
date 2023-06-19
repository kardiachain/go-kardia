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

package genesis

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	kmath "github.com/kardiachain/go-kardia/lib/math"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	kaiproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

//------------------------------------------------------------
// core types for a genesis definition

// GenesisValidator is an initial validator.
type GenesisValidator struct {
	Name             string `json:"name" yaml:"Name"`
	Address          string `json:"address" yaml:"Address"`
	CommissionRate   string `json:"commissionRate" yaml:"CommissionRate"`
	MaxRate          string `json:"maxRate" yaml:"MaxRate"`
	MaxChangeRate    string `json:"maxChangeRate" yaml:"MaxChangeRate"`
	SelfDelegate     string `json:"selfDelegate" yaml:"SelfDelegate"`
	StartWithGenesis bool   `json:"startWithGenesis" yaml:"StartWithGenesis"`
	Delegators       []*struct {
		Address string `json:"address" yaml:"Address"`
		Amount  string `json:"amount" yaml:"Amount"`
	} `json:"delegators" yaml:"Delegators"`
}

// Genesis specifies the header fields, state of a genesis block.
type Genesis struct {
	ChainID       string               `json:"chain_id"`
	InitialHeight uint64               `json:"initial_height"`
	Config        *configs.ChainConfig `json:"config"`
	Timestamp     time.Time            `json:"timestamp"`
	GasLimit      uint64               `json:"gasLimit"   gencodec:"required"`
	Alloc         GenesisAlloc         `json:"alloc"      gencodec:"required"`

	Validators           []*GenesisValidator          `json:"validators"`
	ConsensusParams      *kaiproto.ConsensusParams    `json:"consensus_params,omitempty"`
	Consensus            *configs.ConsensusConfig     `json:"consensusConfig"`
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

// field type overrides for gencodec
type genesisSpecMarshaling struct {
	Timestamp     time.Time
	GasLimit      uint64
	InitialHeight uint64
	Alloc         map[common.UnprefixedAddress]GenesisAccount
}

type genesisAccountMarshaling struct {
	Code    common.Bytes
	Balance *kmath.HexOrDecimal256
	Nonce   uint64
	Storage map[storageJSON]storageJSON
}

// storageJSON represents a 256 bit byte array, but allows less than 256 bits when
// unmarshaling from hex.
type storageJSON common.Hash

func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 // pad on the left
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}

func (h storageJSON) MarshalText() ([]byte, error) {
	return common.Bytes(h[:]).MarshalText()
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
//	                     genesis == nil       genesis != nil
//	                  +------------------------------------------
//	db has no genesis |  main-net default  |  genesis
//	db has genesis    |  from DB           |  genesis (if compatible)
//
// The returned chain configuration is never nil.
func SetupGenesisBlock(db kaidb.Database, genesis *Genesis) (*configs.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
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
		if err != nil {
			return nil, common.NewZeroHash(), err
		}
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		log.Info("Create new genesis block")
		block, _ := genesis.ToBlock(db)
		hash := block.Hash()
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
func (g *Genesis) ToBlock(db kaidb.Database) (*types.Block, common.Hash) {
	if db == nil {
		db = memorydb.New()
	}
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db), nil)

	// Generate genesis deployer address
	g.Alloc[configs.GenesisDeployerAddr] = GenesisAccount{
		Balance: big.NewInt(1000000000000000000), // 1 KAI
		Nonce:   0,
	}

	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}

	head := &types.Header{
		Time:     g.Timestamp,
		Height:   0,
		GasLimit: g.GasLimit,
		AppHash:  common.Hash{},
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: uint32(0),
			},
		},
	}
	if head.GasLimit == 0 {
		head.GasLimit = configs.GenesisGasLimit
	}
	if err := setupGenesisStaking(statedb, kvm.Config{}, g.Timestamp, g.GasLimit, g.Validators); err != nil {
		panic(err)
	}

	root := statedb.IntermediateRoot(false)
	_, _ = statedb.Commit(false)
	_ = statedb.Database().TrieDB().Commit(root, true)

	head.AppHash = root
	block := types.NewBlock(head, nil, &types.Commit{}, nil, trie.NewStackTrie(nil))

	return block, block.AppHash()
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db kaidb.Database) (*types.Block, error) {
	block, root := g.ToBlock(db)
	if block.Height() > 0 {
		return nil, fmt.Errorf("can't commit genesis block with height > 0")
	}
	config := g.Config

	partsSet := block.MakePartSet(types.BlockPartSizeBytes)
	rawdb.WriteBlock(db, block, partsSet, &types.Commit{})
	rawdb.WriteBlockInfo(db, block.Hash(), block.Height(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.Height())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteAppHash(db, block.Height(), root)
	rawdb.WriteChainConfig(db, block.Hash(), config)

	return block, nil
}

// DefaultGenesisBlock returns the main net genesis block.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:   configs.MainnetChainConfig,
		GasLimit: configs.BlockGasLimit,
		//@huny Alloc:    decodePrealloc(mainnetAllocData),
	}
}

// DefaultTestnetGenesisBlock returns the test network genesis block from configs.
func DefaultTestnetGenesisBlock(allocData map[string]*big.Int) *Genesis {

	ga, err := GenesisAllocFromData(allocData)
	if err != nil {
		return nil
	}

	return &Genesis{
		Config:   configs.TestnetChainConfig,
		GasLimit: configs.BlockGasLimit,
		Alloc:    ga,
	}
}

// DefaultTestnetFullGenesisBlock return turn the test network genesis block with both account and smc from configs
func DefaulTestnetFullGenesisBlock(accountData map[string]*big.Int, contractData map[string]string) *Genesis {
	ga, err := GenesisAllocFromAccountAndContract(accountData, contractData)
	if err != nil {
		return nil
	}
	return &Genesis{
		Config:          configs.TestnetChainConfig,
		GasLimit:        configs.BlockGasLimit,
		Alloc:           ga,
		ConsensusParams: configs.DefaultConsensusParams(),
		Consensus:       configs.DefaultConsensusConfig(),
	}
}

func GenesisAllocFromData(data map[string]*big.Int) (GenesisAlloc, error) {
	ga := make(GenesisAlloc, len(data))

	for address, balance := range data {
		ga[common.HexToAddress(address)] = GenesisAccount{Balance: balance}
	}

	return ga, nil
}

// same as DefaultTestnetGenesisBlock, but with smart contract data
func DefaultTestnetGenesisBlockWithContract(allocData map[string]string) *Genesis {
	ga, err := GenesisAllocFromContractData(allocData)
	if err != nil {
		return nil
	}

	return &Genesis{
		Config:   configs.TestnetChainConfig,
		GasLimit: configs.BlockGasLimit,
		Alloc:    ga,
	}
}

func GenesisAllocFromContractData(data map[string]string) (GenesisAlloc, error) {
	ga := make(GenesisAlloc, len(data))

	for address, code := range data {
		ga[common.HexToAddress(address)] = GenesisAccount{Code: common.Hex2Bytes(code), Balance: ToCell(100)}
	}
	return ga, nil
}

func GenesisAllocFromAccountAndContract(accountData map[string]*big.Int, contractData map[string]string) (GenesisAlloc, error) {
	ga := make(GenesisAlloc, len(accountData)+len(contractData))

	for address, balance := range accountData {
		ga[common.HexToAddress(address)] = GenesisAccount{Balance: balance}
	}
	for address, code := range contractData {
		ga[common.HexToAddress(address)] = GenesisAccount{Code: common.Hex2Bytes(code), Balance: ToCell(100)}
	}
	return ga, nil
}

// ToCell converts KAI to CELL. eg: amount * 10^18
func ToCell(amount int64) *big.Int {
	cell := big.NewInt(amount)
	cell.Mul(cell, big.NewInt(int64(math.Pow10(18))))
	return cell
}

func setupGenesisStaking(statedb *state.StateDB, cfg kvm.Config, timestamp time.Time, gaslimit uint64, validators []*GenesisValidator) error {
	// ephemeral header
	header := &types.Header{
		Time:     timestamp,
		Height:   0,
		GasLimit: gaslimit,
		AppHash:  common.Hash{},
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: uint32(0),
			},
		},
	}
	if header.GasLimit == 0 {
		header.GasLimit = configs.GenesisGasLimit
	}

	stakingUtil, stakingUtilErr := staking.NewSmcStakingUtil()
	if stakingUtilErr != nil {
		return stakingUtilErr
	}
	if err := stakingUtil.CreateStakingContract(statedb, header, cfg); err != nil {
		return err
	}
	if err := stakingUtil.SetRoot(statedb, header, nil, cfg); err != nil {
		return err
	}
	// init a validator SMC util to delegate genesis amounts to corresponding validators
	validatorUtil, err := staking.NewSmcValidatorUtil()
	if err != nil {
		return err
	}
	for _, val := range validators {
		if err := stakingUtil.CreateGenesisValidator(statedb, header, nil, cfg,
			common.HexToAddress(val.Address),
			val.Name,
			val.CommissionRate,
			val.MaxRate,
			val.MaxChangeRate,
			val.SelfDelegate); err != nil {
			return fmt.Errorf("apply create validator err: %s", err)
		}
		// delegate genesis amount, if any
		valSmcAddr, err := stakingUtil.GetValFromOwner(statedb, header, nil, cfg, common.HexToAddress(val.Address))
		if err != nil {
			return err
		}
		for _, del := range val.Delegators {
			amount, ok := new(big.Int).SetString(del.Amount, 10)
			if !ok {
				return err
			}
			if err := validatorUtil.Delegate(statedb, header, nil, cfg, valSmcAddr, common.HexToAddress(del.Address), amount); err != nil {
				return err
			}
		}
		if !val.StartWithGenesis {
			continue
		}
		if err := stakingUtil.StartGenesisValidator(statedb, header, nil, cfg, validatorUtil, valSmcAddr,
			common.HexToAddress(val.Address)); err != nil {
			return fmt.Errorf("apply start validator err: %s  Validator info: %+v", err, val)
		}
	}
	return nil
}
