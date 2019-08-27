package main

import (
	"github.com/stretchr/testify/require"
	"path"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	p := path.Join(".", "kai_eth_config_1.yaml")
	config, err := LoadConfig(p)
	require.NoError(t, err)
	require.Nil(t, config.MainChain.EventPool)
	require.Nil(t, config.DualChain.TxPool)
	require.Len(t, config.MainChain.Events, 1)
	require.Len(t, config.DualChain.Events, 1)
	require.Len(t, config.MainChain.Events[0].WatcherActions, 1)
	require.Len(t, config.DualChain.Events[0].WatcherActions, 1)
}
