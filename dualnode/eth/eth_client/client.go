/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethstats"
	"github.com/ethereum/go-ethereum/les"
	log "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
	message2 "github.com/kardiachain/go-kardia/dualnode/message"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/pebbe/zmq4"
	"github.com/rs/cors"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	// headChannelSize is the size of channel listening to ChainHeadEvent.
	headChannelSize = 10
	ServiceName = "ETH"
)

// A full Ethereum node. In additional, it provides additional interface with dual's node,
// responsible for listening to Eth blockchain's new block and submiting Eth's transaction .
type Eth struct {
	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name   string
	logger log.Logger

	// Eth's blockchain stuffs.
	geth   *node.Node
	config *Config
	// TODO(@kiendn): this field must be loaded from config as well as from db to load or watched contract addresses
	smcABI        map[string]abi.ABI
	currentNonce uint64
	sender common.Address
	privateKey ecdsa.PrivateKey

	publishEndpoint string
	subscribeEndpoint string
}

// defaultEthDataDir returns default Eth root datadir.
func defaultEthDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home == "" {
		panic("Fail to get OS home directory")
	}
	return filepath.Join(home, ".ethereum")
}

// Copy from go-kardia/node
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func NewEth(config *Config) (*Eth, error) {

	log.Info("Init New ETH client")

	if len(config.ContractAddress) != len(config.ContractAbis) {
		panic(fmt.Errorf("contract Addresses and abis are mismatched"))
	}

	smcAbi := make(map[string]abi.ABI)
	if len(config.ContractAddress) > 0 {
		for i, address := range config.ContractAddress {
			abiStr := strings.Replace(config.ContractAbis[i], "'", "\"", -1)
			a, err := abi.JSON(strings.NewReader(abiStr))
			if err != nil {
				panic(err)
			}
			smcAbi[address] = a
		}
	}

	// Create a specific logger for ETH Proxy.
	bootUrls := params.RinkebyBootnodes

	datadir := defaultEthDataDir()
	// similar to cmd/eth/config.go/makeConfigNode
	ethConf := &eth.DefaultConfig
	ethConf.NetworkId = uint64(config.NetworkId)

	switch ethConf.NetworkId {
	case 1: // mainnet
		ethConf.Genesis = core.DefaultGenesisBlock()
		datadir = filepath.Join(datadir, "mainnet", config.Name)
		bootUrls = params.MainnetBootnodes
	case 3: // ropsten
		ethConf.Genesis = core.DefaultTestnetGenesisBlock()
		datadir = filepath.Join(datadir, "ropsten", config.Name)
		bootUrls = params.TestnetBootnodes
	case 4: // rinkeby
		ethConf.Genesis = core.DefaultRinkebyGenesisBlock()
		datadir = filepath.Join(datadir, "rinkeby", config.Name)
	default: // default is rinkeby
		ethConf.Genesis = core.DefaultRinkebyGenesisBlock()
		datadir = filepath.Join(datadir, "rinkeby", config.Name)
	}

	bootstrapNodes := make([]*enode.Node, 0, len(bootUrls))
	bootstrapNodesV5 := make([]*discv5.Node, 0, len(bootUrls)) // rinkeby set default bootnodes as also discv5 nodes.
	for _, url := range bootUrls {
		peer, err := enode.ParseV4(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodes = append(bootstrapNodes, peer)

		peerV5, err := discv5.ParseNode(url)
		if err != nil {
			log.Error("BootstrapV5 URL invalid", "enode", url, "err", err)
			continue
		}
		bootstrapNodesV5 = append(bootstrapNodesV5, peerV5)
	}

	// similar to utils.SetNodeConfig
	nodeConfig := &node.Config{
		DataDir:          datadir,
		IPCPath:          "geth.ipc",
		Name:             config.Name,
		HTTPHost:         config.HTTPHost,
		HTTPPort:         config.HTTPPort,
		HTTPVirtualHosts: config.HTTPVirtualHosts,
		HTTPCors:         config.HTTPCors,
	}

	// similar to utils.SetP2PConfig
	nodeConfig.P2P = p2p.Config{
		BootstrapNodes:   bootstrapNodes,
		ListenAddr:       config.ListenAddr,
		MaxPeers:         config.MaxPeers,
		DiscoveryV5:      config.LightNode, // Force using discovery if light node, as in flags.go.
		BootstrapNodesV5: bootstrapNodesV5,
	}

	ethConf.LightServ = config.LightServ
	ethConf.LightPeers = config.LightPeers

	// similar to cmd/utils/flags.go
	ethConf.DatabaseCache = config.CacheSize * 75 / 100
	// Hardcode to 50% of 2048 file descriptor limit for whole process, as in flags.go/makeDatabaseHandles()
	ethConf.DatabaseHandles = config.DBHandle

	// Creates new node.
	ethNode, err := node.New(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("protocol node: %v", err)
	}

	// register fullnode backend
	if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) { return eth.New(ctx, ethConf) }); err != nil {
		return nil, fmt.Errorf("ethereum service: %v", err)
	}

	// Registers ethstats service to report node stat to testnet system.
	if config.ReportStats {
		url := fmt.Sprintf("[Eth]%s:Respect my authoritah!@stats.rinkeby.io", config.StatName)
		if err := ethNode.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			// Retrieve both eth and les services
			var ethServ *eth.Ethereum
			ctx.Service(&ethServ)

			var lesServ *les.LightEthereum
			ctx.Service(&lesServ)

			return ethstats.New(url, ethServ, lesServ)
		}); err != nil {
			log.Error("Failed to register the Ethereum Stats service", "err", err)
		}
	}

	keyBytes, err := hex.DecodeString(config.SignedTxPrivateKey)
	if err != nil {
		panic(err)
	}
	key := crypto.ToECDSAUnsafe(keyBytes)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	return &Eth{
		name:          ServiceName,
		geth:          ethNode,
		config:        config,
		smcABI:        smcAbi,
		publishEndpoint: config.PublishedEndpoint,
		subscribeEndpoint: config.SubscribedEndpoint,
		logger:        config.Logger,
		privateKey: *key,
		sender: addr,
		currentNonce: 0,
	}, nil
}

// Returns the EthClient to acccess Eth subnode.
func (n *Eth) Client() (*ethclient.Client, *node.Node, error) {
	rpcClient, err := n.geth.Attach()
	if err != nil {
		return nil, nil, err
	}
	client := ethclient.NewClient(rpcClient)
	return client, n.geth, nil
}

// syncHead syncs with latest events from Eth network to Kardia.
func (n *Eth)syncHead() {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)

	if ethService == nil {
		log.Error("Not implement dual sync for Eth light mode yet")
		return
	}

	ethChain := ethService.BlockChain()

	chainHeadEventCh := make(chan core.ChainHeadEvent, headChannelSize)
	headSubCh := ethChain.SubscribeChainHeadEvent(chainHeadEventCh)
	defer headSubCh.Unsubscribe()

	blockCh := make(chan *types.Block, 1)

	// Follow other examples.
	// Listener to exhaust extra event while sending block to our channel.
	go func() {
	ListenerLoop:
		for {
			select {
			// Gets chain head events, drop if overload.
			case head := <-chainHeadEventCh:
				select {
				case blockCh <- head.Block:
					// Block field would be nil here.
					log.Info("receive new block", "blockNumber", head.Block.Number(), "txs", len(head.Block.Transactions()))
				default:
					// TODO(thientn): improves performance/handling here.
				}
			case <-headSubCh.Err():
				break ListenerLoop
			}
		}
	}()

	// Handler loop for new blocks.
	for {
		select {
		case block := <-blockCh:
			if !n.config.LightNode {
				go n.handleBlock(block)
			}
		}
	}
}

func (n *Eth)handleBlock(block *types.Block) {
	// TODO(thientn): block from this event is not guaranteed newly update. May already handled before.

	// Some events has nil block.
	if block == nil {
		// TODO(thientn): could call blockchain.CurrentBlock() here.
		log.Info("handleBlock with nil block")
		return
	}

	log.Info("handleBlock...", "blockNum", block.Number(), "txns size", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		if tx.To() == nil {
			log.Trace("To address is nil", "tx", tx.Hash().Hex())
			continue
		}
		// get smc abi from database, return nil if not found
		smcAbi := n.getAbi(tx.To().Hex())
		if smcAbi == nil {
			log.Trace("cannot find abi from to's address", "address", tx.To().Hex(), "tx", tx.Hash().Hex())
			continue
		}
		signer := types.NewEIP155Signer(tx.ChainId())
		sender, err := types.Sender(signer, tx)
		if err != nil {
			log.Error("error while getting sender address", "err", err, "tx", tx.Hash().Hex())
			continue
		}

		// get method and params from data and create a dualMessage message
		method, args := GetMethodAndParams(*smcAbi, tx.Data())
		message := message2.Message{
			TransactionId: tx.Hash().Hex(),
			ContractAddress: tx.To().Hex(),
			BlockNumber: block.Number().Uint64(),
			Sender: sender.Hex(),
			Amount: tx.Value().Uint64(),
			Timestamp: getCurrentTimeStamp(),
			MethodName: method,
			Params: args,
		}

		if err := n.PublishMessage(message); err != nil {
			log.Error("error while publishing tx message", "err", err, "tx", tx.Hash().Hex())
		}
	}
}

func getCurrentTimeStamp() uint64 {
	return uint64(time.Now().UnixNano() / int64(time.Millisecond))
}

// PublishMessage publishes message to 0MQ based on given endpoint, topic
func (n *Eth)PublishMessage(message interface{}) error {
	pub, _ := zmq4.NewSocket(zmq4.PUB)
	defer pub.Close()
	pub.Connect(n.publishEndpoint)

	// sleep 1 second to prevent socket closes
	time.Sleep(1 * time.Second)

	msgToSend, topic, err := GetMessageToSend(message)
	if err != nil {
		return err
	}

	// send topic
	if _, err = pub.Send(topic, zmq4.SNDMORE); err != nil {
		return err
	}

	// send message
	log.Info("Publish message", "topic", topic, "msgToSend", msgToSend)
	if _, err = pub.Send(msgToSend, zmq4.DONTWAIT); err != nil {
		return err
	}

	return nil
}

// GetMessageToSend returns topic and correspond message based on input type
func GetMessageToSend(message interface{}) (string, string, error) {
	var msgToSend, topic string
	var err error
	m := &jsonpb.Marshaler{}

	switch message.(type) {
	case message2.Message:
		msg := message.(message2.Message)
		msgToSend, err = m.MarshalToString(&msg)
		topic = utils.DUAL_MSG
	case message2.TriggerMessage:
		msg := message.(message2.TriggerMessage)
		msgToSend, err = m.MarshalToString(&msg)
		topic = utils.DUAL_CALL
	default:
		err = fmt.Errorf("invalid message type %v", reflect.TypeOf(message))
	}
	if err != nil {
		return "", "", err
	}
	return msgToSend, topic, nil
}

// StartSubscribe subscribes messages from subscribedEndpoint
func (n *Eth)StartSubscribe() {
	subscriber, _ := zmq4.NewSocket(zmq4.SUB)
	defer subscriber.Close()
	subscriber.Bind(n.subscribeEndpoint)
	subscriber.SetSubscribe("")
	time.Sleep(time.Second)
	for {
		if err := n.subscribe(subscriber); err != nil {
			log.Error("Error while subscribing", "err", err.Error())
		}
	}
}

// subscribe handles getting/handle topic and content, return error if any
func (n *Eth)subscribe(subscriber *zmq4.Socket) error {
	//  Read envelope with address
	topic, err := subscriber.Recv(0)
	if err != nil {
		return err
	}
	//  Read message contents
	contents, err := subscriber.Recv(0)
	if err != nil {
		return err
	}
	log.Info("[%s] %s\n", topic, contents)

	switch topic {
	case utils.KARDIA_CALL:
		// call release here
		triggerMessage := message2.TriggerMessage{}
		if err := jsonpb.UnmarshalString(contents, &triggerMessage); err != nil {
			return err
		}

		// from contract address, get abi from it, return error if not found
		tx, err := n.ExecuteTriggerMessage(&triggerMessage)
		if err != nil || tx == nil {
			return err
		}

		// callback here - publish a dual call message back to eth-dual
		for _, cb := range triggerMessage.CallBacks {
			if cb == nil {
				log.Warn("callback is nil", "message", triggerMessage.String())
				continue
			}
			// append tx hash returned by previous trigger tx to callback's param.
			cb.Params = append(cb.Params, *tx)
			if err := n.PublishMessage(*cb); err != nil {
				log.Error("error while publish message to dual node", "err", err)
			}
		}
	default:
		return fmt.Errorf("invalid topic %v", topic)
	}
	return nil
}

func (n *Eth) getAbi(contractAddress string) *abi.ABI {
	if a, ok := n.smcABI[contractAddress]; ok {
		return &a
	}
	return nil
}

// ExecuteTriggerMessage executes smart contract based on data in trigger message
func (n *Eth) ExecuteTriggerMessage(message *message2.TriggerMessage) (*string, error) {
	if message == nil {
		return nil, fmt.Errorf("trigger message is nil")
	}

	// generate args
	if smcAbi := n.getAbi(message.ContractAddress); smcAbi != nil {
		args, err := GenerateArguments(*smcAbi, message.MethodName, message.Params...)
		if err != nil || args == nil {
			return nil, err
		}

		// create input with method name and generated args
		input, err := smcAbi.Pack(message.MethodName, args...)
		if err != nil {
			return nil, err
		}

		// sign new transaction from contractAddress and above input
		tx := n.createEthSmartContractCallTx(common.HexToAddress(message.ContractAddress), input)
		if tx == nil {
			return nil, fmt.Errorf("cannot create new smart contract call for contract %v with method %v", message.ContractAddress, message.MethodName)
		}

		// add tx into eth's pool
		err = n.ethTxPool().AddLocal(tx)
		if err != nil {
			log.Error("Fail to add Ether tx", "error", err)
			return nil, err
		}
		log.Info("Add Eth release tx successfully", "txhash", tx.Hash().Hex())
		// increment nonce by 1
		n.currentNonce += 1
		str := tx.Hash().Hex()
		return &str, nil
	}

	return nil, fmt.Errorf("abi not found with contract %v", message.ContractAddress)
}

func (n *Eth) createEthSmartContractCallTx(contractAddr common.Address, input []byte) *types.Transaction {
	nonce, err := n.getNonce()
	if err != nil {
		return nil
	}
	gasLimit := uint64(40000)
	gasPrice := big.NewInt(5000000000) // 5gwei
	tx, err := types.SignTx(
		types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, input),
		types.HomesteadSigner{},
		&n.privateKey)
	if err != nil {
		panic(err)
	}
	return tx
}

func (n *Eth) getNonce() (uint64, error) {
	statedb, err := n.ethBlockChain().State()
	if err != nil {
		log.Error("Fail to get Ethereum state to create release tx", "err", err)
		return 0, err
	}
	// Nonce of account to sign tx
	nonce := statedb.GetNonce(n.sender)
	if n.currentNonce < nonce {
		return nonce, nil
	}
	return n.currentNonce, nil
}

func (n *Eth) ethBlockChain() *core.BlockChain {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.BlockChain()
}

func (n *Eth) chainDb() ethdb.Database {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.ChainDb()
}

func (n *Eth) ethTxPool() *core.TxPool {
	var ethService *eth.Ethereum
	n.geth.Service(&ethService)
	return ethService.TxPool()
}

// Start starts the Ethereum node.
func (n *Eth) Start() error {
	err := n.geth.Start()
	if err != nil {
		return err
	}
	go n.syncHead()
	go n.StartSubscribe()
	// start an api that receives pump configure
	go func(){
		router := mux.NewRouter()
		router.HandleFunc("/contract/abi", n.updateABI).Methods("POST")
		if err := http.ListenAndServe(n.config.APIListenAddr, cors.AllowAll().Handler(router)); err != nil {
			panic(err)
		}
	}()
	return nil
}

// updateABI adds or updates contract address with its abi to eth client
func (n *Eth) updateABI(w http.ResponseWriter, r *http.Request) {

	data, err := HandlePost(r)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("%v", err))
		return
	}

	m := data.(map[string]interface{})
	contractAddress, ok := m["contractAddress"]
	if !ok || contractAddress == "" {
		respondWithError(w, 500, fmt.Sprintf("contractAddress is required"))
		return
	}

	newAbi, ok := m["abi"]
	if !ok {
		respondWithError(w, 500, fmt.Sprintf("abi is required"))
		return
	}

	newContractAddress, ok := m["newContractAddress"]
	var key string
	if !ok || newContractAddress == "" { // update abi using contractAddress
		key = contractAddress.(string)
	} else {
		key = newContractAddress.(string)

		// if contractAddress exists, remove it
		if _, ok := n.smcABI[contractAddress.(string)]; ok {
			delete(n.smcABI, contractAddress.(string))
		}
	}

	// update abi with current contractAddress
	abiStr := strings.Replace(newAbi.(string), "'", "\"", -1)
	a, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("cannot update abi to contractAddress %v - %v", key, err))
		return
	}
	n.smcABI[key] = a
	respondWithJSON(w, 201, "OK")
}

// GenerateArguments generates args based on inputs types
func GenerateArguments(smcABI abi.ABI, method string, args ...string) ([]interface{}, error) {
	results := make([]interface{}, 0)
	for k, v := range smcABI.Methods {
		if k == method {
			if len(args) != len(v.Inputs) {
				return nil, fmt.Errorf("args and inputs are mismatched")
			}

			for i, input := range v.Inputs {
				switch input.Type.String() {
				case "address":
					results = append(results, common.HexToAddress(args[i]))
				case "uint256":
					var value int
					var err error
					if value, err = strconv.Atoi(args[i]); err != nil {
						return nil, err
					}
					results = append(results, big.NewInt(int64(value)))
				}
			}
			return results, nil
		}
	}
	return nil, fmt.Errorf("cannot find method %v", method)
}

// GetMethodAndParams returns method and list of params in string
func GetMethodAndParams(smcABI abi.ABI, input []byte) (string, []string) {
	args := make([]string, 0)
	method, str, err := GenerateInputStruct(smcABI, input)
	if err != nil || method == nil {
		return "", nil
	}

	if len(input[4:])%32 != 0 {
		return "", nil
	}

	if err := method.Inputs.Unpack(str, input[4:]); err != nil {
		log.Error("error while unpacking inputs", "method", method.Name, "err", err)
		return "", nil
	}
	obj := reflect.ValueOf(str)
	inputs := getInputs(smcABI, method.Name)
	for i, data := range *inputs {
		switch data.Type.String() {
		case "string":
			args = append(args, obj.Elem().Field(i).String())
		}
	}
	return method.Name, args
}

func getInputs(smcABI abi.ABI, method string) *abi.Arguments {
	for k, v := range smcABI.Methods {
		if k == method {
			return &v.Inputs
		}
	}
	return nil
}

// GenerateInputStructs creates structs for all methods from theirs inputs
func GenerateInputStruct(smcABI abi.ABI, input []byte) (*abi.Method, interface{}, error) {
	method, err := smcABI.MethodById(input)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range smcABI.Methods {
		if k == method.Name {
			return method, makeStruct(v.Inputs), nil
		}
	}
	return nil, nil, fmt.Errorf("method not found")
}

// makeStruct makes a struct from abi arguments
func makeStruct(args abi.Arguments) interface{} {
	var sfs []reflect.StructField
	for _, arg := range args {
		sf := reflect.StructField{
			Name: fmt.Sprintf("%v", strings.Title(arg.Name)),
			Type: arg.Type.Type,
		}
		sfs = append(sfs, sf)
	}
	st := reflect.StructOf(sfs)
	so := reflect.New(st)
	return so.Interface()
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// HandlePost handles post data from http.Request and return data as json format
func HandlePost(r *http.Request) (interface{}, error) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
