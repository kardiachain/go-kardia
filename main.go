package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	cs "github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kai"
	"github.com/kardiachain/go-kardia/log"
	"github.com/kardiachain/go-kardia/node"
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

	if *peerURL != "" {
		success, err := n.AddPeer(*peerURL)
		if !success {
			logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
		}
	}

	// TODO(namdoh): Temporarily hook up consensus state here for compiling
	// check purposes.
	consensusState := cs.NewConsensusState(
		nil,
		nil,
		nil,
	)
	consensusState.DoNothing()

	go displayPeers(n)

	blockForever()
}

func displayPeers(n *node.Node) {
	for {
		fmt.Println("Peer list: ", n.Server().PeerCount())
		time.Sleep(10 * time.Second)
	}

}

func blockForever() {
	select {}
}
