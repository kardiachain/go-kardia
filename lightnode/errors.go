// Package lightnode
package lightnode

import (
	"errors"
)

var (
	ErrCreateDataDirFailed     = errors.New("cannot create data dir")
	ErrConfigNameBadCharacters = errors.New(`Config.Name must not contain '/' or '\'`)
	ErrConfigNameConflicted    = errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	ErrConfigNameBadSuffix     = errors.New(`Config.Name cannot end in ".ipc"`)

	ErrWrongConfig = errors.New("wrong config for light node")

	ErrWrongPeer            = errors.New("could not add peers from persistent_peers")
	ErrCreateAddrBookFailed = errors.New("could not create address book")
	ErrLoadStateDBFailed    = errors.New("load stateDB failed")
)
