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