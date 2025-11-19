// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestMigrate_InsertReleases(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	a := &Attachment{
		UUID: "a0eebc91-9c0c-4ef7-bb6e-6bb9bd380a12",
	}
	r := &Release{
		Attachments: []*Attachment{a},
	}

	err := InsertReleases(t.Context(), r)
	assert.NoError(t, err)
}

func Test_FindTagsByCommitIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	sha1Rels, err := FindTagsByCommitIDs(t.Context(), 1, "65f1bf27bc3bf70f64657658635e66094edbcb4d")
	assert.NoError(t, err)
	assert.Len(t, sha1Rels, 1)
	rels := sha1Rels["65f1bf27bc3bf70f64657658635e66094edbcb4d"]
	assert.Len(t, rels, 3)
	assert.Equal(t, "v1.1", rels[0].TagName)
	assert.Equal(t, "delete-tag", rels[1].TagName)
	assert.Equal(t, "v1.0", rels[2].TagName)
}

func TestGetPreviousPublishedRelease(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	current := unittest.AssertExistsAndLoadBean(t, &Release{ID: 8})
	prev, err := GetPreviousPublishedRelease(t.Context(), current.RepoID, current)
	assert.NoError(t, err)
	assert.EqualValues(t, 7, prev.ID)
}

func TestGetPreviousPublishedRelease_NoPublishedCandidate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(1)
	draft := &Release{
		RepoID:       repoID,
		PublisherID:  1,
		TagName:      "draft-prev",
		LowerTagName: "draft-prev",
		IsDraft:      true,
		CreatedUnix:  timeutil.TimeStamp(2),
	}
	current := &Release{
		RepoID:       repoID,
		PublisherID:  1,
		TagName:      "published-current",
		LowerTagName: "published-current",
		CreatedUnix:  timeutil.TimeStamp(3),
	}

	err := InsertReleases(t.Context(), draft, current)
	assert.NoError(t, err)

	_, err = GetPreviousPublishedRelease(t.Context(), repoID, current)
	assert.Error(t, err)
	assert.True(t, IsErrReleaseNotExist(err))
}
