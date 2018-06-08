package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFixturesAreConsistent assert that test fixtures are consistent
func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	CheckConsistencyForAll(t)
}

func TestMain(m *testing.M) {
	MainTest(m, "..")
}
