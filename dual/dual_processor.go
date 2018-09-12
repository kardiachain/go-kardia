package dual

import (
	"github.com/kardiachain/go-kardia/abi"
	bc "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"

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
	smcUpdate := false
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			// New tx that updates smc, check input method for more filter.
			method, err := p.smcABI.MethodById(tx.Data()[0:4])
			if err != nil {
				log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
				return
			}
			log.Info("Detect tx updating smc", "method", method.Name, "Value", tx.Value())
			smcUpdate = true
		}
	}
	if !smcUpdate {
		return
	}
	log.Info("Detect smc update, running VM call to check sending value")

	statedb, err := p.blockchain.StateAt(block.Hash())
	if err != nil {
		log.Error("Error getting block state in dual process", "height", block.Height())
		return
	}

	// Trigger the logic depend on what type of dual node
	// In the future this can be a common interface with a single method
	if p.ethKardia != nil {
		// Eth dual node
		ethSendValue := p.CallKardiaMasterGetEthToSend(p.smcCallSenderAddr, statedb)
		if ethSendValue != nil && ethSendValue.Cmp(big.NewInt(0)) != 0 {
			p.ethKardia.SendEthFromContract(ethSendValue)
		}
	} else {
		// Neo dual node
		neoSendValue := p.CallKardiaMasterGetEthToSend(p.smcCallSenderAddr, statedb)
		if neoSendValue != nil && neoSendValue.Cmp(big.NewInt(0)) != 0 {
			// TODO: create new NEO tx to send NEO
		}

	}

}

func (p *DualProcessor) CallKardiaMasterGetEthToSend(from common.Address, statedb *state.StateDB) *big.Int {
	getEthToSend, err := p.smcABI.Pack("getEthToSend")
	if err != nil {
		log.Error("Fail to pack Kardia smc getEthToSend", "error", err)
		return big.NewInt(0)
	}
	ret, err := callStaticKardiaMasterSmc(from, *p.smcAddress, p.blockchain, getEthToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err)
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ret)
}

func (p *DualProcessor) CallKardiaMasterGetNeoToSend(from common.Address, statedb *state.StateDB) *big.Int {
	getNeoToSend, err := p.smcABI.Pack("getNeoToSend")
	if err != nil {
		log.Error("Fail to pack Kardia smc getEthToSend", "error", err)
		return big.NewInt(0)
	}
	ret, err := callStaticKardiaMasterSmc(from, *p.smcAddress, p.blockchain, getNeoToSend, statedb)
	if err != nil {
		log.Error("Error calling master exchange contract", "error", err)
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ret)
}

// The following function is just call the master smc and return result in bytes format
func callStaticKardiaMasterSmc(from common.Address, to common.Address, blockchain *bc.BlockChain, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := bc.NewKVMContextFromDualNodeCall(from, blockchain.CurrentHeader(), blockchain)
	vmenv := vm.NewKVM(context, statedb, vm.Config{})
	sender := vm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

// CreateKardiaMatchAmountTx creates Kardia tx to report new matching amount from Eth/Neo network.
// type = 1: ETH
// type = 2: NEO
func CreateKardiaMatchAmountTx(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity int, matchType int) *types.Transaction {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = kABI.Pack("matchEth", big.NewInt(int64(quantity)))
	} else {
		getAmountToSend, err = kABI.Pack("matchNeo", big.NewInt(int64(quantity)))
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return tool.GenerateSmcCall(senderKey, masterSmcAddr, getAmountToSend, statedb)
}
