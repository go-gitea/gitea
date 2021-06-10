package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLFSRootURL(t *testing.T) {
	AppURL = "http://localhost:3000"
	LFS.RootURL = ""

	rootURL := GetLFSRootURL()
	assert.Equal(t, rootURL, AppURL)

	LFS.RootURL = "http://localhost:3001"
	rootURL = GetLFSRootURL()
	assert.Equal(t, rootURL, LFS.RootURL)
}
