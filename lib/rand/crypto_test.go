// Package rand
package rand

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomBytes(t *testing.T) {
	r64_1, err := GenerateRandomBytes(64)
	assert.Nil(t, err)
	r64_2, err := GenerateRandomBytes(64)
	assert.Nil(t, err)
	assert.NotEqual(t, r64_1, r64_2)

	r32_1, err := GenerateRandomBytes(32)
	assert.Nil(t, err)
	r32_2, err := GenerateRandomBytes(32)
	assert.Nil(t, err)
	assert.NotEqual(t, r32_1, r32_2)
}

func TestGenerateRandomString(t *testing.T) {
	r64_1, err := GenerateRandomString(64)
	assert.Nil(t, err)
	r64_2, err := GenerateRandomString(64)
	assert.Nil(t, err)
	assert.NotEqual(t, r64_1, r64_2)

	r32_1, err := GenerateRandomString(32)
	assert.Nil(t, err)
	r32_2, err := GenerateRandomString(32)
	assert.Nil(t, err)
	assert.NotEqual(t, r32_1, r32_2)
}
