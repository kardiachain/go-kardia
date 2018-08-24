package node

// PublicNodeAPI offers helper utils
type PublicNodeAPI struct {
	node *Node
}

// NewPublicNodeAPI creates a new PublicNodeAPI instance
func NewPublicNodeAPI(node *Node) *PublicNodeAPI {
	return &PublicNodeAPI{node}
}

// PeersList returns the number of peers that current node can connect to.
func (s *PublicNodeAPI) PeersCount() int {
	return s.node.server.PeerCount()
}


// NodeName returns name of current node
func (s *PublicNodeAPI) NodeName() string {
	return s.node.config.Name
}
