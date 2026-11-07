// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	pages_content "gitea.dev/modules/pages"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestTeam_HasRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.Equal(t, expected, repo_service.HasRepository(t.Context(), team, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, repo_service.RemoveRepositoryFromTeam(t.Context(), team, repoID))
		unittest.AssertNotExistsBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)
	testSuccess(1, unittest.NonexistentID)
}

func TestDeleteOwnerRepositoriesDirectly(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("RejectedInTransaction", func(t *testing.T) {
		err := db.WithTx(t.Context(), func(ctx context.Context) error {
			return repo_service.DeleteRepositoryDirectly(ctx, 1)
		})
		assert.ErrorContains(t, err, "must not be called within a transaction")
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	})

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.NoError(t, repo_service.DeleteOwnerRepositoriesDirectly(t.Context(), user))
}

func TestDeleteRepositoryDirectlyPurgesRepoScopedRows(t *testing.T) {
	unittest.PrepareTestEnv(t)
	contents := [][]byte{[]byte("current Pages content"), []byte("historical Pages content")}
	digests := make([]string, 0, len(contents))
	for _, content := range contents {
		digest := sha256.Sum256(content)
		digests = append(digests, hex.EncodeToString(digest[:]))
		assert.NoError(t, pages_content.NewContentStore().Save(1, digests[len(digests)-1], bytes.NewReader(content), int64(len(content))))
	}
	currentVersion := &pages_model.Version{RepoID: 1, TarGzSHA256: digests[0], OriginType: pages_model.VersionOriginUpload}
	historicalVersion := &pages_model.Version{RepoID: 1, TarGzSHA256: digests[1], OriginType: pages_model.VersionOriginUpload}
	assert.NoError(t, db.Insert(t.Context(), currentVersion, historicalVersion))

	// One row per table that repository deletion used to leave behind (#38494).
	assert.NoError(t, db.Insert(t.Context(),
		&pages_model.Site{RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceArtifact, CurrentVersionID: currentVersion.ID, PagesUpdatedUnixNano: 1},
		&actions_model.ActionVariable{RepoID: 1, Name: "to_purge", Data: "value"},
		&actions_model.ActionRunAttempt{RepoID: 1, RunID: unittest.NonexistentID, Attempt: 1},
		&actions_model.ActionTasksVersion{RepoID: 1, Version: 1},
		&git_model.RenamedBranch{RepoID: 1, From: "old-name", To: "new-name"},
		&git_model.CommitStatusSummary{RepoID: 1, SHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", State: "success"},
		&repo_model.RepoTransfer{RepoID: 1, DoerID: 2, RecipientID: 3},
		&issues_model.Reaction{Type: "heart", UserID: 2, ReleaseID: 1},
	))
	unittest.AssertExistsAndLoadBean(t, &git_model.CommitStatusIndex{RepoID: 1})

	assert.NoError(t, repo_service.DeleteRepositoryDirectly(t.Context(), 1))

	unittest.AssertNotExistsBean(t, &actions_model.ActionVariable{RepoID: 1})
	unittest.AssertNotExistsBean(t, &actions_model.ActionRunAttempt{RepoID: 1})
	unittest.AssertNotExistsBean(t, &actions_model.ActionTasksVersion{RepoID: 1})
	unittest.AssertNotExistsBean(t, &git_model.RenamedBranch{RepoID: 1})
	unittest.AssertNotExistsBean(t, &git_model.CommitStatusSummary{RepoID: 1})
	unittest.AssertNotExistsBean(t, &git_model.CommitStatusIndex{RepoID: 1})
	unittest.AssertNotExistsBean(t, &repo_model.RepoTransfer{RepoID: 1})
	unittest.AssertNotExistsBean(t, &issues_model.Reaction{ReleaseID: 1})
	pagesSite, hasPagesSite, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.True(t, hasPagesSite)
	if hasPagesSite {
		assert.True(t, pagesSite.Deleted)
		assert.False(t, pagesSite.Enabled)
		assert.Empty(t, pagesSite.SourceType)
		assert.Zero(t, pagesSite.CurrentVersionID)
	}
	unittest.AssertNotExistsBean(t, &pages_model.Version{RepoID: 1})
	for _, digest := range digests {
		_, err := pages_content.NewContentStore().Stat(1, digest)
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}
