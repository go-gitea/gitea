// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestCommitTime(t *testing.T) {
	lct, err := GetLatestCommitTime(".")
	assert.NoError(t, err)
	// Time is in the past
	now := time.Now()
	assert.True(t, lct.Unix() < now.Unix(), "%d not smaller than %d", lct, now)
	// Time is after Mon Oct 23 03:52:09 2017 +0300
	// which is the time of commit
	// d47b98c44c9a6472e44ab80efe65235e11c6da2a
	refTime, err := time.Parse("Mon Jan 02 15:04:05 2006 -0700", "Mon Oct 23 03:52:09 2017 +0300")
	assert.NoError(t, err)
	assert.True(t, lct.Unix() > refTime.Unix(), "%d not greater than %d", lct, refTime)
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
