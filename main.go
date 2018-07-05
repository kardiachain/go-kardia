package main

import (
	"conceptchain/log"
	"conceptchain/node"
	"flag"
	"fmt"
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

	flag.Parse()

	config := &node.DefaultConfig
	config.P2P.ListenAddr = *listenAddr
	config.Name = *name

	n, err := node.NewNode(config)

	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	n.Start()

	if *peerURL != "" {
		success, err := n.AddPeer(*peerURL)
		if !success {
			logger.Error("Fail to add peer", "err", err, "peerUrl", peerURL)
		}
	}

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
