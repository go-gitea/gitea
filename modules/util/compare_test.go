package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStringMatchesPattern(t *testing.T) {
	// Make sure non-wildcard matching works
	assert.True(t, StringMatchesPattern("fubar", "fubar"))

	// Make sure wildcard matching accepts
	assert.True(t, StringMatchesPattern("A is not B", "A*B"))
	assert.True(t, StringMatchesPattern("A is not B", "A*"))

	// Make sure wildcard matching rejects
	assert.False(t, StringMatchesPattern("fubar", "A*B"))
	assert.False(t, StringMatchesPattern("A is not b", "A*B"))

	// Make sure regexp specials are escaped
	assert.False(t, StringMatchesPattern("A is not B", "[aA]*"))
	assert.True(t, StringMatchesPattern("[aA] is not B", "[aA]*"))
}
