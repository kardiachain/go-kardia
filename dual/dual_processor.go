package dual

import (
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/vm"
	bc "github.com/kardiachain/go-kardia/blockchain"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"math/big"
)

func CallKardiaMasterSmc(from common.Address, to common.Address, blockchain *bc.BlockChain, input []byte, statedb *state.StateDB) {
	context := bc.NewKVMContextFromDualNodeCall(from, blockchain.CurrentHeader(), blockchain)
	vmenv := vm.NewKVM(context, statedb, vm.Config{})
	sender := vm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		log.Info("Error calling smart contract", "err", err)
	}
	log.Info("Result", "result", big.NewInt(0).SetBytes(ret))
}
