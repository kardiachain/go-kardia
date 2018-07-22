package main

import (
	"flag"
	"fmt"
	elog "github.com/ethereum/go-ethereum/log"
	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/crypto"
	"github.com/kardiachain/go-kardia/dual"
	"github.com/kardiachain/go-kardia/kai"
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"time"
)

func main() {
	// args
	logLevel := flag.String("loglevel", "info", "minimum log verbosity to display")
	ethLogLevel := flag.String("ethloglevel", "warn", "minimum Eth log verbosity to display")
	listenAddr := flag.String("addr", ":30301", "listen address")
	peerURL := flag.String("peer", "", "enode URL of static peer")
	name := flag.String("name", "", "Name of node")
	addTxn := flag.Bool("txn", false, "whether to add a fake txn")
	dualMode := flag.Bool("dual", false, "whether to run in dual mode")
	ethstat := flag.Bool("ethstat", false, "report eth stats to network")
	lightNode := flag.Bool("light", false, "connect to Eth as light node")

	flag.Parse()

	// Setups log to Stdout.
	level, err := log.LvlFromString(*logLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		level = log.LvlInfo
	}
	log.Root().SetHandler(log.LvlFilterHandler(level, log.StdoutHandler))
	logger := log.New()

	elevel, err := elog.LvlFromString(*ethLogLevel)
	if err != nil {
		fmt.Printf("invalid log level argument, default to INFO: %v \n", err)
		elevel = elog.LvlInfo
	}
	elog.Root().SetHandler(elog.LvlFilterHandler(elevel, elog.StdoutHandler))

	// Setups config.
	config := &node.DefaultConfig
	config.P2P.ListenAddr = *listenAddr
	config.Name = *name

	n, err := node.NewNode(config)

	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	n.RegisterService(kai.NewKardiaService)
	if err := n.Start(); err != nil {
		logger.Error("Cannot start node", "err", err)
		return
	}

	var kService *kai.Kardia
	if err := n.Service(&kService); err != nil {
		logger.Error("Cannot get Kardia Serivce", "err", err)
		return
	}
	if *addTxn {
		logger.Info("Adding local txn")
		emptyTx := types.NewTransaction(
			0,
			common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
			big.NewInt(0), 0, big.NewInt(0),
			nil,
		)
		txPool := kService.TxPool()
		key, _ := crypto.GenerateKey()
		signedTx, _ := types.SignTx(emptyTx, *txPool.PoolSigner(), key)

		txPool.AddLocal(signedTx)
	}

	if *peerURL != "" {
		success, err := n.AddPeer(*peerURL)
		if !success {
			logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
		}
	}

	// go displayPeers(n)

	if *dualMode {
		ethNode, err := dual.NewEthKardia(*lightNode, *ethstat)
		if err != nil {
			logger.Error("Fail to create Eth sub node", "err", err)
			return
		}
		if err := ethNode.Start(); err != nil {
			logger.Error("Fail to start Eth sub node", "err", err)
			return
		}
		go displayEthPeers(ethNode)

		client, err := ethNode.Client()
		if err != nil {
			logger.Error("Fail to create EthKardia client", "err", err)
			return
		}
		go displaySyncStatus(client)
	}

	go displayKardiaPeers(n)
	waitForever()
}

func displayEthPeers(n *dual.EthKardia) {
	for {
		log.Info("Ethereum peers: ", "count", n.EthNode().Server().PeerCount())
		time.Sleep(20 * time.Second)
	}

}

func displayKardiaPeers(n *node.Node) {
	for {
		log.Info("Kardia peers: ", "count", n.Server().PeerCount())
		time.Sleep(20 * time.Second)
	}

}

func displaySyncStatus(client *dual.KardiaEthClient) {
	for {
		status, err := client.NodeSyncStatus()
		if err != nil {
			log.Error("Fail to check sync status of EthKarida", "err", err)
		} else {
			log.Info("Sync status", "sync", status)
		}
		time.Sleep(20 * time.Second)
	}
}

func waitForever() {
	select {}
}
