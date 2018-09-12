package dual

import (
	"github.com/kardiachain/go-kardia/abi"
	bc "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/tool"

	"crypto/ecdsa"
	"math/big"
	"strings"
)



type DualProcessor struct {
	blockchain        *bc.BlockChain
	smcAddress        *common.Address
	smcABI            *abi.ABI
	smcCallSenderAddr common.Address

	// For when running dual node to Eth network
	ethKardia *EthKardia
	// TODO: add struct when running dual node to Neo

	// Chain head subscription for new blocks.
	chainHeadCh  chan bc.ChainHeadEvent
	chainHeadSub event.Subscription
}

func NewDualProcessor(chain *bc.BlockChain, smcAddr *common.Address, smcABIStr string) (*DualProcessor, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &DualProcessor{
		blockchain:        chain,
		smcAddress:        smcAddr,
		smcABI:            &smcABI,
		smcCallSenderAddr: common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5"),

		chainHeadCh: make(chan bc.ChainHeadEvent, 5),
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = chain.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *DualProcessor) Start() {
	// Start event loop
	go p.loop()
}

func (p *DualProcessor) RegisterEthDualNode(ethKardia *EthKardia) {
	p.ethKardia = ethKardia
}

func (p *DualProcessor) loop() {
	for {
		select {
		case ev := <-p.chainHeadCh:
			if ev.Block != nil {
				// New block
				// TODO(thietn): concurrency improvement. Consider call new go routine, or have height atomic counter.
				p.checkNewBlock(ev.Block)
			}
		case err := <-p.chainHeadSub.Err():
			log.Error("Error while listening to new blocks", "error", err)
			return
		}
	}
}

func (p *DualProcessor) checkNewBlock(block *types.Block) {
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			// New tx that updates smc, check input method for more filter.
			method, err := p.smcABI.MethodById(tx.Data()[0:4])
			if err != nil {
				log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
				return
			}
			log.Info("Detect tx updating smc", "method", method.Name, "Value", tx.Value())

			statedb, err := p.blockchain.StateAt(block.Hash())
			if err != nil {
				log.Error("Error getting block state in dual process", "height", block.Height())
				return
			}

			// Trigger the logic depend on what type of dual node
			// In the future this can be a common interface with a single method
			if p.ethKardia != nil {
				getEthToSend, err := p.smcABI.Pack("getEthToSend")
				if err != nil {
					log.Error("Error packing ABI func getEthToSend", "error", err)
					return
				}
				ethSendValue := callKardiaMasterSmc(p.smcCallSenderAddr, *p.smcAddress, p.blockchain, getEthToSend, statedb)
				p.ethKardia.SendEthFromContract(ethSendValue)
			}
		}
	}

}

func callKardiaMasterSmc(from common.Address, to common.Address, blockchain *bc.BlockChain, input []byte, statedb *state.StateDB) *big.Int {
	context := bc.NewKVMContextFromDualNodeCall(from, blockchain.CurrentHeader(), blockchain)
	vmenv := vm.NewKVM(context, statedb, vm.Config{})
	sender := vm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		log.Info("Error calling smart contract", "err", err)
	}
	result := big.NewInt(0).SetBytes(ret)
	log.Info("Kardia SMC call result", "result", result)
	return result
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
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

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
	return new(big.Int).SetBytes(ret)
}

func CallKardiaMasterGetNeoToSend(from common.Address, blockchain *bc.BlockChain, statedb *state.StateDB) *big.Int {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

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
	return new(big.Int).SetBytes(ret)
}

// Call to update matching amount
// type = 1: ETH
// type = 2: NEO
func CallKardiaMasterMatchAmount(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction{
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = abi.Pack("matchEth", quantity)
	} else {
		getAmountToSend, err = abi.Pack("matchNeo", quantity)
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return CallUpdateKardiaMasterSmc(senderKey, masterSmcAddr, getAmountToSend, statedb)
}

// Call to remove amount of ETH / NEO on master smc
// type = 1: ETH
// type = 2: NEO

func CallKardiaMasterRemoveAmount(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction{
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = abi.Pack("removeEth", quantity)
	} else {
		getAmountToSend, err = abi.Pack("removeNeo", quantity)
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return CallUpdateKardiaMasterSmc(senderKey, masterSmcAddr, getAmountToSend, statedb)
}