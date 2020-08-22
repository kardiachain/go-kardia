package genesis

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToCell(t *testing.T) {
	cell := ToCell(int64(math.Pow(10, 6)))
	assert.Equal(t, len(cell.String()), 25)
}
