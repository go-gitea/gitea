// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetTags(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)

	tags, err := bareRepo1.GetTagInfos()
	assert.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.EqualValues(t, "test", tags[0].Name)
	assert.EqualValues(t, "37991dec2c8e592043f47155ce4808d4580f9123", tags[0].ID.String())
	assert.EqualValues(t, "commit", tags[0].Type)
}

func TestRepository_GetTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(bareRepo1Path, testReposDir, "repo1_TestRepository_GetTag")
	assert.NoError(t, err)
	defer os.RemoveAll(clonedPath)

	bareRepo1, err := OpenRepository(clonedPath)
	assert.NoError(t, err)

	tag, err := bareRepo1.GetTag("test")
	assert.NoError(t, err)
	assert.NotNil(t, tag)
	assert.EqualValues(t, "test", tag.Name)
	assert.EqualValues(t, "37991dec2c8e592043f47155ce4808d4580f9123", tag.ID.String())
	assert.EqualValues(t, "commit", tag.Type)
}
