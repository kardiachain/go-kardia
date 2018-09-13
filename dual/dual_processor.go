package dual

import (
	"encoding/hex"
	"github.com/kardiachain/go-kardia/abi"
	bc "github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"time"
	"net/http"
	"bytes"
	"io/ioutil"
	"encoding/json"

	"github.com/shopspring/decimal"
)

type DualProcessor struct {
	blockchain        *bc.BlockChain
	txPool            *bc.TxPool
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

func NewDualProcessor(chain *bc.BlockChain, txPool *bc.TxPool, smcAddr *common.Address, smcABIStr string) (*DualProcessor, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &DualProcessor{
		blockchain:        chain,
		txPool:            txPool,
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
			if method.Name == "removeEth" || method.Name == "removeNeo" {
				// Not set flag here. If the block contains only the removeEth/removeNeo, skip look up the amount to avoid infinite loop.
				log.Info("Skip tx updating smc to remove Eth/Neo", "method", method.Name)
				continue
			}
			smcUpdate = true
		}
	}
	if !smcUpdate {
		return
	}
	log.Info("Detect smc update, running VM call to check sending value")

	statedb, err := p.blockchain.StateAt(block.Root())
	if err != nil {
		log.Error("Error getting block state in dual process", "height", block.Height())
		return
	}

	// Trigger the logic depend on what type of dual node
	// In the future this can be a common interface with a single method
	if p.ethKardia != nil {
		// Eth dual node
		ethSendValue := p.CallKardiaMasterGetEthToSend(p.smcCallSenderAddr, statedb)
		log.Info("Kardia smc calls getEthToSend", "eth", ethSendValue)
		if ethSendValue != nil && ethSendValue.Cmp(big.NewInt(0)) != 0 {
			p.ethKardia.SendEthFromContract(ethSendValue)

			// Create Kardia tx removeEth right away to acknowledge the ethsend
			gAccount := "0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547"
			addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[gAccount])
			addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)

			tx := CreateKardiaRemoveAmountTx(addrKey, statedb, ethSendValue, 1)
			if err := p.txPool.AddLocal(tx); err != nil {
				log.Error("Fail to add Kardia tx to removeEth", err, "tx", tx)
			} else {
				log.Info("Creates removeEth tx", tx.Hash().Hex())
			}
		}
	} else {
		// Neo dual node
		neoSendValue := p.CallKardiaMasterGetNeoToSend(p.smcCallSenderAddr, statedb)
		log.Info("Kardia smc calls getNeoToSend", "neo", neoSendValue)
		if neoSendValue != nil && neoSendValue.Cmp(big.NewInt(0)) != 0 {
			// TODO: create new NEO tx to send NEO
			// Temporarily hard code the recipient
			amountToRelease := decimal.NewFromBigInt(neoSendValue, 10).Div(decimal.NewFromBigInt(common.BigPow(10, 18), 10))
			if amountToRelease.IntPart() < 1 {
				log.Info("Too little amount to send")
			} else {
				// temporarily hard code for the exchange rate
				go p.ReleaseNeo("APCarJ7aYRqfakPmRHsNGByWR3MMemUyBn", big.NewInt(amountToRelease.IntPart() * 10))
				// Create Kardia tx removeNeo to acknowledge the neosend, otherwise getEthToSend will keep return >0
				gAccount := "0xBA30505351c17F4c818d94a990eDeD95e166474b"
				addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[gAccount])
				addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)

				tx := CreateKardiaRemoveAmountTx(addrKey, statedb, neoSendValue, 2)
				if err := p.txPool.AddLocal(tx); err != nil {
					log.Error("Fail to add Kardia tx to removeNeo", err, "tx", tx)
				} else {
					log.Info("Creates removeNeo tx", tx.Hash().Hex())
				}
			}

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
func CreateKardiaMatchAmountTx(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = kABI.Pack("matchEth", quantity)
	} else {
		getAmountToSend, err = kABI.Pack("matchNeo", quantity)
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return tool.GenerateSmcCall(senderKey, masterSmcAddr, getAmountToSend, statedb)
}

// Call to remove amount of ETH / NEO on master smc
// type = 1: ETH
// type = 2: NEO

func CreateKardiaRemoveAmountTx(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var amountToRemove []byte
	if matchType == 1 {
		amountToRemove, err = abi.Pack("removeEth", quantity)
	} else {
		amountToRemove, err = abi.Pack("removeNeo", quantity)
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)

	}
	return tool.GenerateSmcCall(senderKey, masterSmcAddr, amountToRemove, statedb)
}

// Call Api to release Neo
func CallReleaseNeo(address string, amount *big.Int) (string, error) {
	body := []byte(`{
  "jsonrpc": "2.0",
  "method": "dual_sendeth",
  "params": ["` + address +`",` + amount.String() + `],
  "id": 1
}`)
	log.Info("Release neo", "message", string(body))
	rs, err := http.Post(dev.NeoSubmitTxUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	bytesRs, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return "", err
	}
	var f interface{}
	json.Unmarshal(bytesRs, &f)
	m := f.(map[string]interface{})
	var txid string
	txid = m["result"].(string)
	return txid, nil
}

// Call Neo api to check status
// txid is "not found" : pending tx or tx is failed, need to loop checking
// to cover both case
func checkTxNeo(txid string) bool {
	log.Info("Checking tx id", "txid", txid)
	url := dev.NeoCheckTxUrl + txid
	rs, err := http.Get(url)
	if err != nil {
		return false
	}
	bytesRs, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return false
	}
	var f interface{}
	json.Unmarshal(bytesRs, &f)
	m := f.(map[string]interface{})
	txid = m["txid"].(string)
	if txid != "not found" {
		return true
	} else {
		return false
	}
}

// retry send and loop checking tx status until it is successful
func retryTx(address string, amount *big.Int) {
	attempt := 0
	interval := 30
	for {
		log.Info("retrying tx ...", "addr", address, "amount", amount)
		txid, err := CallReleaseNeo(address, amount)
		if err == nil && txid != "fail" {
			log.Info("Send successfully", "txid", txid)
			result := loopCheckingTx(txid)
			if result {
				log.Info("tx is successful")
				return
			} else {
				log.Info("tx is not successful, retry in 5 sconds", "txid", txid)
			}
		} else {
			log.Info("Posting tx failed, retry in 5 seconds", "txid", txid)
		}
		attempt ++
		if attempt > 1 {
			log.Info("Trying 2 time but still fail, give up now", "txid", txid)
			return
		}
		sleepDuration := time.Duration(interval) * time.Second
		time.Sleep(sleepDuration)
		interval += 30
	}
}

// Continually check tx status for 10 times, interval is 10 seconds
func loopCheckingTx(txid string) bool {
	attempt := 0
	for {
		time.Sleep(10 * time.Second)
		attempt ++
		success := checkTxNeo(txid)
		if !success && attempt > 10 {
			log.Info("Tx fail, need to retry", "attempt", attempt)
			return false
		}

		if success {
			log.Info("Tx is successful", "txid", txid)
			return true
		}
	}
}

func(p *DualProcessor) ReleaseNeo(address string, amount *big.Int) {
	log.Info("Release: ", "amount", amount, "address", address)
	txid, err := CallReleaseNeo(address, amount)
	if err != nil {
		log.Error("Error calling rpc", "err", err)
	}
	log.Info("Tx submitted", "txid", txid)
	if txid == "fail" {
		retryTx(address, amount)
	} else {
		txStatus := loopCheckingTx(txid)
		if !txStatus {
			retryTx(address, amount)
		}
	}
}