package main

import (
	"flag"
	"fmt"
	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/crypto"
	"github.com/kardiachain/go-kardia/dual"
	"github.com/kardiachain/go-kardia/kai"
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"os"
	"time"
)

func main() {
	// setup log to stdout.
	handler := log.StreamHandler(os.Stdout, log.TerminalFormat(false))
	log.Root().SetHandler(handler)
	logger := log.New()

	// args
	listenAddr := flag.String("addr", ":30301", "listen address")
	peerURL := flag.String("peer", "", "enode URL of static peer")
	name := flag.String("name", "", "Name of node")
	addTxn := flag.Bool("txn", false, "whether to add a fake txn")
	dualMode := flag.Bool("dual", false, "whether to run in dual mode")

	flag.Parse()

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
		ethNode, err := dual.NewEthKardia()
		if err != nil {
			logger.Error("Fail to create Eth sub node", "err", err)
			return
		}
		if err := ethNode.Start(); err != nil {
			logger.Error("Fail to start Eth sub node", "err", err)
		}
		go displayPeers(ethNode)

	}

	waitForever()
}

func displayPeers(n *dual.EthKardia) {
	for {
		fmt.Println("Peer list: ", n.EthNode().Server().PeerCount())
		time.Sleep(10 * time.Second)
	}

}

func waitForever() {
	select {}
}
