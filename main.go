package main

import (
	"conceptchain/node"
	"fmt"
)

func main() {
	n, err := node.NewNode(&node.DefaultConfig)

	if err != nil {
		fmt.Println(err)
		return
	}
	n.Start()
	fmt.Println(n.Server.Peers())
	n.Stop()
}
