package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoad(t *testing.T) {
	path := "."
	name := "config"
	
	config, err := Load(path, name)
	require.NoError(t, err)
	require.Len(t, config.ContractAddress, 1)
	require.Len(t, config.ContractAbis, 1)
}