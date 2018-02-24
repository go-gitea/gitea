package generate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	retVal := m.Run()

	os.Exit(retVal)
}

func TestGetRandomString(t *testing.T) {
	randomString, err := GetRandomString(4)
	assert.NoError(t, err)
	assert.Len(t, randomString, 4)
}
