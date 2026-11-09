// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

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

func TestReleaseTagNameIsCaseSensitive(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	exist, err := IsReleaseExist(t.Context(), 1, "V1.1")
	assert.NoError(t, err)
	assert.False(t, exist)

	rel, err := GetRelease(t.Context(), 1, "v1.1")
	assert.NoError(t, err)
	assert.NoError(t, db.Insert(t.Context(), &Release{RepoID: 1, TagName: "V1.1", Sha1: "upper"}))

	upper, err := GetRelease(t.Context(), 1, "V1.1")
	assert.NoError(t, err)
	assert.Equal(t, "upper", upper.Sha1)
	lower, err := GetRelease(t.Context(), 1, "v1.1")
	assert.NoError(t, err)
	assert.Equal(t, rel.ID, lower.ID)
}

func Test_FindTagsByCommitIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("_FindTagsByCommitIDs", func(t *testing.T) {
		sha1Rels, err := FindTagsByCommitIDs(t.Context(), 1, "65f1bf27bc3bf70f64657658635e66094edbcb4d")
		assert.NoError(t, err)
		assert.Len(t, sha1Rels, 1)
		rels := sha1Rels["65f1bf27bc3bf70f64657658635e66094edbcb4d"]
		assert.Len(t, rels, 3)
		assert.Equal(t, "v1.1", rels[0].TagName)
		assert.Equal(t, "delete-tag", rels[1].TagName)
		assert.Equal(t, "v1.0", rels[2].TagName)
	})

	t.Run("AddReleaseAttachmentsRejectsDifferentRepo", func(t *testing.T) {
		uuid := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12" // attachment 2 belongs to repo 2
		err := AddReleaseAttachments(t.Context(), 1, []string{uuid})
		assert.Error(t, err)
		assert.ErrorIs(t, err, util.ErrPermissionDenied)

		attach, err := GetAttachmentByUUID(t.Context(), uuid)
		assert.NoError(t, err)
		assert.Zero(t, attach.ReleaseID, "attachment should not be linked to release on failure")
	})

	t.Run("AddReleaseAttachmentsRejectsIssueAttachment", func(t *testing.T) {
		uuid := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
		err := AddReleaseAttachments(t.Context(), 1, []string{uuid})
		assert.ErrorIs(t, err, util.ErrPermissionDenied)

		attach, err := GetAttachmentByUUID(t.Context(), uuid)
		assert.NoError(t, err)
		assert.Zero(t, attach.ReleaseID)
	})

	t.Run("AddReleaseAttachmentsRejectsRecentZeroRepoID", func(t *testing.T) {
		recentUUID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd3800aa"
		attachment := &Attachment{
			UUID:        recentUUID,
			RepoID:      0,
			IssueID:     0,
			ReleaseID:   0,
			CommentID:   0,
			Name:        "recent-zero",
			CreatedUnix: LegacyAttachmentMissingRepoIDCutoff + 1,
		}
		assert.NoError(t, db.Insert(t.Context(), attachment))

		err := AddReleaseAttachments(t.Context(), 1, []string{recentUUID})
		assert.Error(t, err)
		assert.ErrorIs(t, err, util.ErrPermissionDenied)

		attach, err := GetAttachmentByUUID(t.Context(), recentUUID)
		assert.NoError(t, err)
		assert.Zero(t, attach.ReleaseID)
		assert.Zero(t, attach.RepoID)
	})

	t.Run("AddReleaseAttachmentsAllowsLegacyMissingRepoID", func(t *testing.T) {
		legacyUUID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a20" // attachment 10 has repo_id 0
		err := AddReleaseAttachments(t.Context(), 1, []string{legacyUUID})
		assert.NoError(t, err)

		attach, err := GetAttachmentByUUID(t.Context(), legacyUUID)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, attach.RepoID)
		assert.EqualValues(t, 1, attach.ReleaseID)
	})
}

func TestImmutableTag(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})
	isImmutable := func(r *Repository, tagName string) bool {
		immutable, err := IsTagImmutable(t.Context(), r, tagName)
		assert.NoError(t, err)
		return immutable
	}

	assert.False(t, isImmutable(repo, "V1.1"))
	assert.NoError(t, db.Insert(t.Context(), &ImmutableTag{
		RepoID: repo.ID, LowerOwnerName: strings.ToLower(repo.OwnerName), LowerRepoName: repo.LowerName, TagName: "V1.1",
	}))
	assert.True(t, isImmutable(repo, "V1.1"))
	assert.False(t, isImmutable(repo, "v1.1"))

	successor := &Repository{ID: repo.ID + 9999, OwnerName: repo.OwnerName, LowerName: repo.LowerName}
	assert.True(t, isImmutable(successor, "V1.1"))
	assert.True(t, isImmutable(&Repository{ID: repo.ID, OwnerName: "new-owner", LowerName: "renamed"}, "V1.1"))
	assert.False(t, isImmutable(&Repository{ID: successor.ID, OwnerName: "new-owner", LowerName: "renamed"}, "V1.1"))

	assert.NoError(t, db.Insert(t.Context(), &Release{RepoID: repo.ID, TagName: "V1.1", IsTag: true}))
	assert.NoError(t, CheckImmutableTagConflicts(t.Context(), repo.ID, repo.OwnerName, repo.LowerName))
	assert.NoError(t, CheckImmutableTagConflicts(t.Context(), successor.ID, repo.OwnerName, repo.LowerName))
	assert.NoError(t, db.Insert(t.Context(), &Release{RepoID: successor.ID, TagName: "V1.1", IsTag: true}))
	assert.ErrorIs(t, CheckImmutableTagConflicts(t.Context(), successor.ID, repo.OwnerName, repo.LowerName), ErrImmutableTag)
	assert.NoError(t, CheckImmutableTagConflicts(t.Context(), successor.ID, repo.OwnerName, "elsewhere"))

	rel := unittest.AssertExistsAndLoadBean(t, &Release{ID: 1})
	hasRelease := func() bool {
		has, err := HasImmutableRelease(t.Context(), rel.RepoID, rel.TagName)
		assert.NoError(t, err)
		return has
	}
	_, err := db.GetEngine(t.Context()).ID(rel.ID).Cols("is_immutable").Update(&Release{IsImmutable: true})
	assert.NoError(t, err)
	assert.True(t, hasRelease())

	rel.IsTag, rel.IsImmutable = true, false
	assert.NoError(t, UpdateRelease(t.Context(), rel))
	assert.False(t, hasRelease())
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &Release{ID: rel.ID}).IsImmutable)
}
