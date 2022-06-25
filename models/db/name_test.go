package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidUsername(t *testing.T) {
	assert.True(t, IsValidUsername("abc"))
	assert.True(t, IsValidUsername("0.b-c"))
	assert.False(t, IsValidUsername(".abc"))
	assert.False(t, IsValidUsername("a/bc"))
}
