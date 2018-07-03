package main

import (
	"conceptchain/node"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"os"
	"flag"
)

func main() {
	// setup max log verbosity.
	logger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	logger.Verbosity(log.Lvl(6))
	logger.Vmodule("")
	log.Root().SetHandler(logger)

	// args
	listenAddr := flag.String("addr", ":30301", "listen address")
	flag.Parse()

	config := &node.DefaultConfig
	config.P2P.ListenAddr = *listenAddr

	n, err := node.NewNode(config)

	if err != nil {
		fmt.Println(err)
		return
	}
	n.Start()
	fmt.Println(n.Server.Peers())
	blockForever()
}

func blockForever() {
	select{ }
}