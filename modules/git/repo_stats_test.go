// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetCodeActivityStats(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	timeFrom, err := time.Parse(time.RFC3339, "2016-01-01T00:00:00+00:00")
	assert.NoError(t, err)

	code, err := bareRepo1.GetCodeActivityStats(timeFrom, "")
	assert.NoError(t, err)
	assert.NotNil(t, code)

	assert.EqualValues(t, 9, code.CommitCount)
	assert.EqualValues(t, 3, code.AuthorCount)
	assert.EqualValues(t, 9, code.CommitCountInAllBranches)
	assert.EqualValues(t, 10, code.Additions)
	assert.EqualValues(t, 1, code.Deletions)
	assert.Len(t, code.Authors, 3)
	assert.Contains(t, code.Authors, "tris.git@shoddynet.org")
	assert.EqualValues(t, 3, code.Authors["tris.git@shoddynet.org"])
	assert.EqualValues(t, 5, code.Authors[""])
}
