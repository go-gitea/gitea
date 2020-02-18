// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueWatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.NoError(t, CreateOrUpdateIssueWatchMode(3, 1, IssueWatchModeNormal))
	iw := AssertExistsAndLoadBean(t, &IssueWatch{UserID: 3, IssueID: 1}).(*IssueWatch)
	assert.EqualValues(t, IssueWatchModeNormal, iw.Mode)

	assert.NoError(t, DeleteIssueWatch(3, 1))
	AssertNotExistsBean(t, &IssueWatch{UserID: 3, IssueID: 1})

	assert.NoError(t, CreateOrUpdateIssueWatchMode(3, 1, IssueWatchModeAuto))
	iw = AssertExistsAndLoadBean(t, &IssueWatch{UserID: 3, IssueID: 1}).(*IssueWatch)
	assert.EqualValues(t, IssueWatchModeAuto, iw.Mode)

	assert.NoError(t, CreateOrUpdateIssueWatchMode(1, 1, IssueWatchModeAuto))
	iw = AssertExistsAndLoadBean(t, &IssueWatch{UserID: 1, IssueID: 1}).(*IssueWatch)
	assert.EqualValues(t, IssueWatchModeAuto, iw.Mode)

	time.Sleep(1 * time.Second)
	assert.NoError(t, CreateOrUpdateIssueWatchMode(1, 1, IssueWatchModeDont))
	iw = AssertExistsAndLoadBean(t, &IssueWatch{UserID: 1, IssueID: 1}).(*IssueWatch)
	assert.EqualValues(t, IssueWatchModeDont, iw.Mode)
}

func TestGetIssueWatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	_, exists, err := GetIssueWatch(9, 1)
	assert.True(t, exists)
	assert.NoError(t, err)

	iw, exists, err := GetIssueWatch(2, 2)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.False(t, iw.IsWatching())

	_, exists, err = GetIssueWatch(3, 1)
	assert.False(t, exists)
	assert.NoError(t, err)

	assert.False(t, IsWatching(1, 10))
	iw, exists, err = GetIssueWatch(1, 8)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.True(t, iw.IsWatching())
}

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	iws, err := GetIssueWatchers(1, ListOptions{})
	assert.NoError(t, err)
	// Watcher is inactive, thus 0
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(2, ListOptions{})
	assert.NoError(t, err)
	// Watcher is explicit not watching
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(5, ListOptions{})
	assert.NoError(t, err)
	// Issue has no Watchers
	assert.Len(t, iws, 0)

	iws, err = GetIssueWatchers(7, ListOptions{})
	assert.NoError(t, err)
	// Issue has one watcher
	assert.Len(t, iws, 1)
}
