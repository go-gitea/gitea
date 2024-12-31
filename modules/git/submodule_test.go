package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetSubmoduleCommits(t *testing.T) {
	testRepoPath := filepath.Join(testReposDir, "repo4_submodules")
	submodules, err := GetSubmoduleCommits(DefaultContext, testRepoPath)
	require.NoError(t, err)

	assert.EqualValues(t, len(submodules), 2)

	assert.EqualValues(t, submodules[0].Path, "<°)))><")
	assert.EqualValues(t, submodules[0].Commit, "d2932de67963f23d43e1c7ecf20173e92ee6c43c")

	assert.EqualValues(t, submodules[1].Path, "libtest")
	assert.EqualValues(t, submodules[1].Commit, "1234567890123456789012345678901234567890")
}
