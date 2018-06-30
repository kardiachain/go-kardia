package main

import (
	"fmt"
)

func main() {
	n, err := NewNode("node1")

	if err == nil {
		fmt.Println(len(n.blockchain))
	}
}
