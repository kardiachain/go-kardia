package node

// PublicNodeAPI offers helper utils
type PublicNodeAPI struct {
	node *Node
}

// NewPublicNodeAPI creates a new PublicNodeAPI instance
func NewPublicNodeAPI(node *Node) *PublicNodeAPI {
	return &PublicNodeAPI{node}
}

// PeersCount returns the number of peers that current node can connect to.
func (s *PublicNodeAPI) PeersCount() int {
	return s.node.server.PeerCount()
}


// Peers returns a list of peers
func (s *PublicNodeAPI) Peers() map[string]string{
	peers := s.node.server.Peers()
	results := make(map[string]string)
	for _, peer := range peers {
		results[peer.Name()] = peer.RemoteAddr().String()
	}
	return results
}


// NodeName returns name of current node
func (s *PublicNodeAPI) NodeName() string {
	return s.node.config.Name
}
