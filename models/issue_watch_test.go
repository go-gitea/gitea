// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueWatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.NoError(t, CreateOrUpdateIssueWatch(3, 1, true))
	iw := AssertExistsAndLoadBean(t, &IssueWatch{UserID: 3, IssueID: 1}).(*IssueWatch)
	assert.True(t, iw.IsWatching)

	assert.NoError(t, CreateOrUpdateIssueWatch(1, 1, false))
	iw = AssertExistsAndLoadBean(t, &IssueWatch{UserID: 1, IssueID: 1}).(*IssueWatch)
	assert.False(t, iw.IsWatching)
}

func TestGetIssueWatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	_, exists, err := GetIssueWatch(9, 1)
	assert.True(t, exists)
	assert.NoError(t, err)

	iw, exists, err := GetIssueWatch(2, 2)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.EqualValues(t, false, iw.IsWatching)

	_, exists, err = GetIssueWatch(3, 1)
	assert.False(t, exists)
	assert.NoError(t, err)
}

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	iws, err := GetIssueWatchers(1)
	assert.NoError(t, err)
	// Watcher is inactive, thus 0
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(2)
	assert.NoError(t, err)
	// Watcher is explicit not watching
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(5)
	assert.NoError(t, err)
	// Issue has no Watchers
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(7)
	assert.NoError(t, err)
	// Issue has one watcher
	assert.Len(t, iws, 1)
}
