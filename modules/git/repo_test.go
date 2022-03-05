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
	// Time is Sun Jul 21 22:43:13 2019 +0200
	// which is the time of commit
	// feaf4ba6bc635fec442f46ddd4512416ec43c2c2 (refs/heads/master)
	assert.EqualValues(t, 1563741793, lct.Unix())
}

func TestRepoIsEmpty(t *testing.T) {
	emptyRepo2Path := filepath.Join(testReposDir, "repo2_empty")
	repo, err := OpenRepository(emptyRepo2Path)
	assert.NoError(t, err)
	defer repo.Close()
	isEmpty, err := repo.IsEmpty()
	assert.NoError(t, err)
	assert.True(t, isEmpty)
}
