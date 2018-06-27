package main

import (
	"fmt"
)

func main() {
	n, err := NewNode()

	if err == nil {
		fmt.Println(len(n.Blockchain))
	}
}
