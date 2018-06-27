package main

// TODO: move to a blockstore.
type Block struct {
	Index        int
	Hash         string
	PreviousHash string
	Content      string
}

// Wrapper for a running node.
type Node struct {
	Blockchain []Block
}

func NewNode() (*Node, error) {
	node := new(Node)
	return node, nil
}
