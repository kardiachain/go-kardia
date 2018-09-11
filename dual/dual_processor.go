package dual

import (
	abi2 "github.com/kardiachain/go-kardia/abi"

	bc "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"

	"crypto/ecdsa"
	"math/big"
	"strings"
)

// The following function is just call the master smc and display result,
// which is mainly used for testing. Should be removed soon
func CallKardiaMasterSmcWithoutReturn(from common.Address, to common.Address, blockchain *bc.BlockChain, input []byte, statedb *state.StateDB) {
	context := bc.NewKVMContextFromDualNodeCall(from, blockchain.CurrentHeader(), blockchain)
	vmenv := vm.NewKVM(context, statedb, vm.Config{})
	sender := vm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		log.Info("Error calling smart contract", "err", err)
	}
	log.Info("Result", "result", big.NewInt(0).SetBytes(ret))
}

// The following function is just call the master smc and return result in bytes format
func CallStaticKardiaMasterSmc(from common.Address, to common.Address, blockchain *bc.BlockChain, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := bc.NewKVMContextFromDualNodeCall(from, blockchain.CurrentHeader(), blockchain)
	vmenv := vm.NewKVM(context, statedb, vm.Config{})
	sender := vm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}
// senderKey *ecdsa.PrivateKey, address common.Address, input []byte, stateDb *state.StateDB
func CallUpdateKardiaMasterSmc(senderKey *ecdsa.PrivateKey, address common.Address, input []byte, statedb *state.StateDB)  *types.Transaction {
	return tool.GenerateSmcCall(senderKey, address, input, statedb)
}

func CallKardiaMasterGetEthToSend(from common.Address, blockchain *bc.BlockChain, statedb *state.StateDB) *big.Int {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi2.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
		return big.NewInt(0)
	}

	getEthToSend, err := abi.Pack("getEthToSend")
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return big.NewInt(0)
	}
	ret, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, blockchain, getEthToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err)
		return big.NewInt(0)
	}
	return big.NewInt(0).SetBytes(ret)
}

func CallKardiaMasterGetNeoToSend(from common.Address, blockchain *bc.BlockChain, statedb *state.StateDB) *big.Int {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi2.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
		return big.NewInt(0)
	}

	getEthToSend, err := abi.Pack("getNeoToSend")
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return big.NewInt(0)
	}
	ret, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, blockchain, getEthToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err)
		return big.NewInt(0)
	}
	return big.NewInt(0).SetBytes(ret)
}

// Call to update matching amount
// type = 1: ETH
// type = 2: NEO
func CallKardiaMasterMatchAmount(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity int, matchType int) *types.Transaction{
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi2.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = abi.Pack("matchEth", big.NewInt(int64(quantity)))
	} else {
		getAmountToSend, err = abi.Pack("matchNeo", big.NewInt(int64(quantity)))
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return CallUpdateKardiaMasterSmc(senderKey, masterSmcAddr, getAmountToSend, statedb)
}
