package types

import "fmt"

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	// Various consensus engines
	Kaicon *KaiconConfig `json:"kaicon,omitempty"`
}

// KaiconConfig is the consensus engine configs for Kardia BFT DPoS.
type KaiconConfig struct {
	// TODO(huny): implement this
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *KaiconConfig) String() string {
	return "kaicon"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Kaicon != nil:
		engine = c.Kaicon
	default:
		engine = "unknown"
	}
	return fmt.Sprintf("{Engine: %v}",
		engine,
	)
}