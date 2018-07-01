package main

import (
	"fmt"
)

func main() {
	n, err := NewNode("node1")

	if err != nil {
		fmt.Println(err)
		return
	}
	n.start()
	//fmt.Println(n.server.PeerCount())
	// FIXME: need to initialize server config for this, currently null pointer
	// fmt.Println(n.server.PeerCount())
	n.stop()
}
