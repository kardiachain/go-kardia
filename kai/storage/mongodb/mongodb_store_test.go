package mongodb

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewDB(t *testing.T) {
	uri := "mongodb://127.0.0.1:27017"
	databaseName := "kardiachain"

	_, err := NewDB(uri, databaseName, false)
	require.NoError(t, err)
}
