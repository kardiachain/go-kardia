// Package lightnode
package lightnode

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/service"
)

type Node struct {
	service.BaseService

	cfg Config
}

// New
func New(cfg Config) (*Node, error) {
	// Create new empty node and start parsing config
	node := &Node{
		cfg: cfg,
	}

	fmt.Println("Cfg ", cfg)

	// Create if logger nil
	if node.Logger == nil {
		node.Logger = log.New()
	}
	node.Logger.AddTag("lightNode")

	// Validate data dir
	if err := node.validateConfig(); err != nil {
		return nil, fmt.Errorf(ErrWrongConfig.Error(), err)
	}
	fmt.Printf("Node info: %+v \n", node)
	return node, nil
}

//region helper and private func
func (n *Node) validateConfig() error {
	if n.cfg.DataDir != "" {
		abdDataDir, err := filepath.Abs(n.cfg.DataDir)
		if err != nil {
			n.cfg.Logger.Debug("cannot get abs file path", "dataDir", n.cfg.DataDir)
			return ErrCreateDataDirFailed
		}
		n.cfg.DataDir = abdDataDir
	}

	// Ensure that the instance name doesn't cause weird conflicts with
	// other files in the data directory.
	if strings.ContainsAny(n.cfg.Name, prohibitCharacaters) {
		return ErrConfigNameBadCharacters
	}
	if n.cfg.Name == datadirDefaultKeyStore {
		return ErrConfigNameConflicted
	}
	if strings.HasSuffix(n.cfg.Name, prohibitSuffix) {
		return ErrConfigNameBadSuffix
	}
	return nil
}

//endregion helper and private func
