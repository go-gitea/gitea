// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestCommitTime(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	lct, err := GetLatestCommitTime(DefaultContext, bareRepo1Path)
	assert.NoError(t, err)
	// Time is Sun Nov 13 16:40:14 2022 +0100
	// which is the time of commit
	// ce064814f4a0d337b333e646ece456cd39fab612 (refs/heads/master)
	assert.EqualValues(t, 1668354014, lct.Unix())
}

func TestRepoIsEmpty(t *testing.T) {
	emptyRepo2Path := filepath.Join(testReposDir, "repo2_empty")
	repo, err := openRepositoryWithDefaultContext(emptyRepo2Path)
	assert.NoError(t, err)
	defer repo.Close()
	isEmpty, err := repo.IsEmpty()
	assert.NoError(t, err)
	assert.True(t, isEmpty)
}
