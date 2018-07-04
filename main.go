package main

import (
	"conceptchain/node"
	"fmt"
	"conceptchain/log"
	"os"
	"flag"
)

func main() {
	// setup log to stdout.
	handler := log.StreamHandler(os.Stdout, log.TerminalFormat(false))
	log.Root().SetHandler(handler)
	logger := log.New()

	// args
	listenAddr := flag.String("addr", ":30301", "listen address")
	peerUrl := flag.String("peer", "", "enode URL of static peer")

	flag.Parse()

	config := &node.DefaultConfig
	config.P2P.ListenAddr = *listenAddr

	n, err := node.NewNode(config)

	if err != nil {
		logger.Error("Cannot create node", "err", err)
		return
	}

	n.Start()

	if *peerUrl != "" {
		success, err := n.AddPeer(*peerUrl)
		if !success {
			logger.Error("Fail to add peer", "err", err, "peerUrl", peerUrl)
		}
	}

	fmt.Println(n.Server().Peers())
	blockForever()
}

func blockForever() {
	select{ }
}