// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
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
	assert.EqualValues(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", tags[0].ID.String())
	assert.EqualValues(t, "commit", tags[0].Type)
}
