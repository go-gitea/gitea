// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
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

func TestGetDivergingCommits(t *testing.T) {
	// divergence of master branch to itself - should be 0 ahead and 0 behind
	divergence, err := GetDivergingCommits(".", "master", "master")
	assert.NoError(t, err)
	assert.Equal(t, 0, divergence.Behind)
	assert.Equal(t, 0, divergence.Ahead)

	// divergence of two commits
	// baseBranch commit represents the v1.8.0 tag, targetBranch commit represents the v1.7.6 tag
	// unable to directly use tags due to CI pipeline(?)
	divergence2, err := GetDivergingCommits(".", "8b3aad940e915b9db11deb0f06d9e5338cfe3fdd", "bafa9ff4323cef048412157e5f5a7bc516adf985")
	assert.NoError(t, err)
	assert.Equal(t, 360, divergence2.Behind)
	assert.Equal(t, 72, divergence2.Ahead)
}
