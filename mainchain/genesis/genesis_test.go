package genesis

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestToCell(t *testing.T) {
	cell := ToCell(int64(math.Pow(10, 6)))
	assert.Equal(t, len(cell.String()),25)
}
