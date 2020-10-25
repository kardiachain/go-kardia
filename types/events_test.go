package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryForEvent(t *testing.T) {
	assert.Equal(t,
		"kai.event='NewBlock'",
		QueryForEvent(EventNewBlock).String(),
	)

}
