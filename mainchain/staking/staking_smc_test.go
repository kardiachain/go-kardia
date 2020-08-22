package staking

import (
	"math"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/types"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"
)

var Pubkey = "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860"

const MaximumInitialNodes = 16

func setupGenesis(g *genesis.Genesis, db types.StoreDB) (*types.ChainConfig, common.Hash, error) {
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	return genesis.SetupGenesisBlock(log.New(), db, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
}

func GetBlockchain() (*blockchain.BlockChain, error) {
	// Start setting up blockchain
	initValue := g.ToCell(int64(math.Pow10(6)))
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}
	stakingSmcAddress := configs.GetContractAddressAt(KardiaSatkingSmcIndex).String()
	// add, abi := configs.GetContractDetailsByIndex(KardiaSatkingSmcIndex)
	// fmt.Printf("addd %s", add)
	// fmt.Printf("abi %s", abi)
	// fmt.Printf("sssssss %s", configs.GenesisContracts[stakingSmcAddress])
	var genesisContracts = map[string]string{
		stakingSmcAddress: configs.GenesisContracts[stakingSmcAddress],
	}
	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, kaiDb)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, false)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, err
	}
	return bc, nil
}

func GetSmcStakingUtil() (*StakingSmcUtil, error) {
	bc, err := GetBlockchain()
	if err != nil {
		return nil, err
	}
	util, err := NewSmcStakingnUtil(bc)
	if err != nil {
		return nil, err
	}
	return util, nil
}

func TestGetSmcStakingUtil(t *testing.T) {
	bc, err := GetBlockchain()
	if err != nil {
		t.Log(err)
	}
	util, err := NewSmcStakingnUtil(bc)
	if err != nil {
		t.Log(err)
	}
	t.Log(util)
}

func TestInflation(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	// Check valid node with correct type
	store, err := util.SetInflation(1000, address)
	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Log("storeeeee", store)
	}

	get, err := util.GetInflation(address)

	if err != nil {
		t.Log("err", err)
	}

	if get.Cmp(big.NewInt(1000)) != 0 {
		t.Error("Expected 1000, got ", get)
	}

}

func TestTotalSupply(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	// Check valid node with correct type
	store, err := util.SetTotalSupply(9000, address)
	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Log("storeeeee", store)
	}

	get, err := util.GetTotalSupply(address)
	if err != nil {
		t.Log("err", err)
	}
	if get.Cmp(big.NewInt(9000)) != 0 {
		t.Error("Expected 9000, got ", get)
	}
}

func TestSetParams(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")

	// baseProposerReward := uint64(1)
	// bonusProposerReward := 2
	// slashFractionDowntime := 3
	// slashFractionDoubleSign := 2
	// unBondingTime := 1
	// signedBlockWindow := 2
	// minSignedBlockPerWindow := 1
	store, err := util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 2, 5000000000000, address)

	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Log("storeeeee", store)
	}

}

func TestCreateValidator(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	store, err := util.CreateValidator(20, 5, 5, address)

	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Log("storeeeee", store)
	}

}
