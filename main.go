package main

import (
	"conceptchain/node"
	"fmt"
)

func main() {
	n, err := node.NewNode("node1")

	if err != nil {
		fmt.Println(err)
		return
	}
	n.Start()
	fmt.Println(n.server.PeerCount())
	fmt.Println(n.Server.Peers())
	n.Stop()
}
