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

func TestInflation(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	// Check valid node with correct type
	_, err = util.SetInflation(1000, address)
	if err != nil {
		t.Fatal(err)
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
	_, err = util.SetTotalSupply(9000, address)
	if err != nil {
		t.Fatal(err)
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
	_, err = util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 2, 5000000000000, address)

	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateValidator(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	_, err = util.CreateValidator(2, 20, 5, 5, address, 3999999999)

	if err != nil {
		t.Fatal(err)
	}

	//get validator
	token, delShare, jail, err := util.GetValidator(address)
	if err != nil {
		t.Log("Error", err)
	}

	if token.Cmp(big.NewInt(3999999999)) != 0 {
		t.Error("Expected 3999999999, got ", token)
	}

	if delShare.Cmp(big.NewInt(1000000000000000000)) != 0 {
		t.Error("Expected 1000000000000000000, got ", delShare)
	}

	if jail {
		t.Error("Expected false, got ", jail)
	}

}

func TestSetAndGetValidatorSet(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	store, err := util.CreateValidator(2, 20, 5, 5, address, 30)

	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Log("storeeeee", store)
	}

	err = util.ApplyAndReturnValidatorSets(address)
	if err != nil {
		t.Log(err)
	}
	_, _, err = util.GetValidatorSets()
	if err != nil {
		t.Log(err)
	}
}

func TestMint(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.SetMintParams(1, 1, 5, 1, 1, address)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.SetTotalSupply(10000, address)
	if err != nil {
		t.Fatal(err)
	}

	err = util.SetTotalBonded(1000, address)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.SetInflation(0, address)
	if err != nil {
		t.Fatal(err)
	}

	err = util.SetAnnualProvision(1, address)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.Mint()
	if err != nil {
		t.Fatal(err)
	}

	inflation, err := util.GetInflation(address)
	if err != nil {
		t.Fatal(err)
	}

	if inflation.Cmp(big.NewInt(1)) != 0 {
		t.Error("Expected 1, got ", inflation)
	}
}

func TestFinalizeCommit(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	valAddress1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	_, err = util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 3, 5000000000000, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.CreateValidator(2, 20, 5, 5, valAddress1, 30)
	if err != nil {
		t.Fatal(err)
	}

	addrs := []common.Address{valAddress1}
	powers := []*big.Int{big.NewInt(1)}
	signed := []bool{false}

	err = util.FinalizeCommit(addrs, powers, signed, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	missedBlock, err := util.GetMissedBlock(valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	if !missedBlock[0] {
		t.Error("Expected true, got ", missedBlock)
	}

}

func TestDoubleSign(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	valAddress1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	_, err = util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 3, 5000000000000, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.CreateValidator(2, 20, 5, 5, valAddress1, 30)
	if err != nil {
		t.Fatal(err)
	}

	//get validator
	token, _, _, err := util.GetValidator(valAddress1)
	if err != nil {
		t.Log("Error", err)
	}

	if token.Cmp(big.NewInt(30)) != 0 {
		t.Error("Expected 30, got ", token)
	}

	err = util.DoubleSign(valAddress1, 1000000, 10, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	//slash 100% token
	token, _, _, err = util.GetValidator(valAddress1)
	if err != nil {
		t.Log("Error", err)
	}

	if token.Cmp(big.NewInt(0)) != 0 {
		t.Error("Expected 0, got ", token)
	}
}

func TestUndelegate(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	valAddress1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	_, err = util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 3, 5000000000000, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.CreateValidator(2, 20, 5, 5, valAddress1, 30)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.SetTotalSupply(1000, valAddress1)

	addrs := []common.Address{}
	powers := []*big.Int{}
	signed := []bool{}

	err = util.FinalizeCommit(addrs, powers, signed, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	err = util.Undelegate(valAddress1, 30, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	token, _, _, err := util.GetValidator(valAddress1)
	if err != nil {
		t.Log("Error", err)
	}
	t.Log(token)
}

//should jail when self delegation too low
func TestJail(t *testing.T) {
	util, err := GetSmcStakingUtil()
	if err != nil {
		t.Fatal(err)
	}

	valAddress1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")

	_, err = util.SetParams(1000000000000, 100000000000, 1000000000000, 5000000000000,
		1, 3, 5000000000000, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.CreateValidator(2, 20, 5, 5, valAddress1, 30)
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.SetTotalSupply(1000, valAddress1)

	err = util.Undelegate(valAddress1, 29, valAddress1)
	if err != nil {
		t.Fatal(err)
	}

	token, _, jail, err := util.GetValidator(valAddress1)
	if err != nil {
		t.Log("Error", err)
	}

	if token.Cmp(big.NewInt(2)) != 0 {
		t.Error("Expected 0, got ", token)
	}

	if !jail {
		t.Error("Expected true, got ", false)
	}
}
