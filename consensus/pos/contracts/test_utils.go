package contracts

import (
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/types"
	"math"
	"math/big"
)

const maximumGasUsed = uint64(7000000)
var (
	masterAddress = common.HexToAddress("0x0000000000000000000000000000000000000009")
	genesisNodes = []map[string]interface{}{
		{
			"address": "0x0000000000000000000000000000000000000010",
			"id": "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860",
			"name": "node1",
			"host": "127.0.0.1",
			"port": "3000",
			"percentageReward": uint16(500),
			"owner": "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
			"staker": "0x0000000000000000000000000000000000000020",
		},
		{
			"address": "0x0000000000000000000000000000000000000011",
			"id": "660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0",
			"name": "node2",
			"host": "127.0.0.1",
			"port": "3001",
			"percentageReward": uint16(500),
			"owner": "0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5",
			"staker": "0x0000000000000000000000000000000000000021",
		},
		{
			"address": "0x0000000000000000000000000000000000000012",
			"id": "2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da",
			"name": "node3",
			"host": "127.0.0.1",
			"port": "3002",
			"percentageReward": uint16(500),
			"owner": "0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd",
			"staker": "0x0000000000000000000000000000000000000022",
		},
	}
	minimumStakes, _ = big.NewInt(0).SetString("1000000000000000000", 10)
)

// staticCall calls smc and return result in bytes format
func staticCall(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, statedb *state.StateDB) (result []byte, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, maximumGasUsed)
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

func call(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, value *big.Int, statedb *state.StateDB) (result []byte, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.Call(sender, to, input, maximumGasUsed, value)
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

func create(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, statedb *state.StateDB) (result []byte, address *common.Address, leftOverGas uint64, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, contractAddr, leftOver, err := vmenv.CreateGenesisContract(sender, &to, input, maximumGasUsed, big.NewInt(0))
	if err != nil {
		return make([]byte, 0), nil, leftOver, err
	}
	address = &contractAddr
	return ret, address, leftOver, nil
}

func setupGenesis(g *genesis.Genesis, db *types.MemStore) (*types.ChainConfig, common.Hash, error) {
	address := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	privateKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	return genesis.SetupGenesisBlock(log.New(), db, g, &types.BaseAccount{
		Address:    address,
		PrivateKey: *privateKey,
	})
}

func setupBlockchain() (*blockchain.BlockChain, error) {
	initValue := genesis.ToCell(int64(math.Pow10(6)))

	var genesisAccounts = map[string]*big.Int{
		"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": initValue,
	}
	kaiDb := types.NewMemStore()
	g := genesis.DefaulTestnetFullGenesisBlock(genesisAccounts, map[string]string{})
	chainConfig, _, genesisErr := setupGenesis(g, kaiDb)
	if genesisErr != nil {
		return nil, genesisErr
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, true)
	return bc, err
}
