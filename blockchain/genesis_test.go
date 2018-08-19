package blockchain

import (
	"github.com/kardiachain/go-kardia/account"
	"github.com/kardiachain/go-kardia/blockchain/rawdb"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/storage"
	"github.com/pborman/uuid"
	"os"
	"path/filepath"
	"testing"
)

const (
	password = "KardiaChain"
)

var (
	addresses = []string{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5",
		"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd",
		"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28",
		"0x94FD535AAB6C01302147Be7819D07817647f7B63",
		"0xa8073C95521a6Db54f4b5ca31a04773B093e9274",
		"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547",
		"0xBA30505351c17F4c818d94a990eDeD95e166474b",
		"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0",
		"0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
	}
	privKeys = []string{
		"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
		"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
		"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
		"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
		"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
		"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
		"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
		"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
		"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
		"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d",
	}
	balance = int64(100000000)
	folder  = uuid.New()
)

func TestGenesisAllocFromData(t *testing.T) {

	var data = make(map[string]int64, len(privKeys))
	for _, pk := range privKeys {
		keystore := account.KeyStore{Path: ""}
		keystoreJson, err := keystore.NewKeyStoreJSON(password, &pk)

		if err != nil {
			t.Error("Cannot create new keystore")
		}

		data[keystoreJson.Address] = balance
	}

	ga, err := GenesisAllocFromData(data)
	if err != nil {
		t.Error(err)
	}

	for _, el := range addresses {
		if _, ok := ga[common.StringToAddress(el)]; ok == false {
			t.Error("address ", el, " is not found")
		}
	}
}

func TestCreateGenesisBlock(t *testing.T) {
	// Test generate genesis block
	// allocData is get from genesisAccounts in default_node_config

	// Init kai database
	db, err := storage.NewLDBStore(folder, 16, 16)
	if err != nil {
		t.Error(err)
	}

	// Create genesis block with dev.genesisAccounts
	genesis := DefaultTestnetGenesisBlock(dev.GenesisAccounts)
	_, hash, err := SetupGenesisBlock(db, genesis)

	// There are 2 ways of getting current blockHash
	// ReadHeadBlockHash or ReadCanonicalHash
	headBlockHash := rawdb.ReadHeadBlockHash(db)
	canonicalHash := rawdb.ReadCanonicalHash(db, 0)

	if !hash.Equal(headBlockHash) || !hash.Equal(canonicalHash) {
		t.Error("Current BlockHash does not match")
	}

	// Get block by hash and height
	block := rawdb.ReadBlock(db, hash, 0)

	// Init new State with current BlockHash
	s, err := state.New(block.Root(), state.NewDatabase(db))
	if err != nil {
		t.Error(err)
	} else {
		// Get balance from addresses
		for _, address := range addresses {
			b := s.GetBalance(common.StringToAddress(address)).Int64()
			if b != balance {
				t.Error("Balance does not match")
			}
		}

	}
}

func TestMain(m *testing.M) {

	retCode := m.Run()
	// Remove folder
	err := RemoveDir(folder)
	if err != nil {
		println(err)
	}
	os.Exit(retCode)
}

func RemoveDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}

	return os.Remove(dir)
}
