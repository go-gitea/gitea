package secret

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	result, err := New()
	assert.NoError(t, err)
	assert.True(t, len(result) > 32)

	result2, err := New()
	assert.NoError(t, err)
	// check if secrets
	assert.NotEqual(t, result, result2)
}
