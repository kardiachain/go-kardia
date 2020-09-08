package channel

import (
	"log"
	"math"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/types"
)

// GenesisAccounts are used to initialized accounts in genesis block
var initValue = genesis.ToCell(int64(math.Pow10(6)))
var genesisAccounts = map[string]*big.Int{
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
}

func TestStateTransition_TransitionDb_noFee(t *testing.T) {
	// Start setting up blockchain
	blockDB := memorydb.New()
	storeDB := kvstore.NewStoreDB(blockDB)
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(log.New(), storeDB, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
	if genesisErr != nil {
		t.Fatal(genesisErr)
	}

	bc, err := blockchain.NewBlockChain(log.New(), storeDB, chainConfig, false)
	if err != nil {
		t.Fatal(err)
	}

}
