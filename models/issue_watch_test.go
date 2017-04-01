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
	assert.Equal(t, true, iw.IsWatching)

	assert.NoError(t, CreateOrUpdateIssueWatch(1, 1, false))
	iw = AssertExistsAndLoadBean(t, &IssueWatch{UserID: 1, IssueID: 1}).(*IssueWatch)
	assert.Equal(t, false, iw.IsWatching)
}

func TestGetIssueWatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	_, exists, err := GetIssueWatch(1, 1)
	assert.Equal(t, true, exists)
	assert.NoError(t, err)

	_, exists, err = GetIssueWatch(2, 2)
	assert.Equal(t, true, exists)
	assert.NoError(t, err)

	_, exists, err = GetIssueWatch(3, 1)
	assert.Equal(t, false, exists)
	assert.NoError(t, err)
}

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	iws, err := GetIssueWatchers(1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(iws))

	iws, err = GetIssueWatchers(5)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(iws))
}
