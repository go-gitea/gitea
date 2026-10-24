// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	mirror_service "gitea.dev/services/mirror"
	release_service "gitea.dev/services/release"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMirrorPull(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Metrics.Enabled, true)()
	defer test.MockVariableValue(&setting.Metrics.EnabledMirrorSyncDuration, true)()

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoPath := gitrepo.RepoLocalPath(repo)

	opts := migration.MigrateOptions{
		RepoName:    "test_mirror",
		Description: "Test mirror",
		Private:     false,
		Mirror:      true,
		CloneAddr:   repoPath,
		Wiki:        true,
		Releases:    true,
	}

	mirrorRepo, err := repo_service.CreateRepositoryDirectly(ctx, user, user, repo_service.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	}, false)
	assert.NoError(t, err)
	assert.True(t, mirrorRepo.IsMirror, "expected pull-mirror repo to be marked as a mirror immediately after its creation")

	mirrorRepo, err = repo_service.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	assert.NoError(t, err)

	// these units should have been enabled
	mirrorRepo.Units = nil
	require.NoError(t, mirrorRepo.LoadUnits(ctx))
	assert.True(t, slices.ContainsFunc(mirrorRepo.Units, func(u *repo_model.RepoUnit) bool { return u.Type == unit.TypeReleases }))
	assert.True(t, slices.ContainsFunc(mirrorRepo.Units, func(u *repo_model.RepoUnit) bool { return u.Type == unit.TypeWiki }))

	gitRepo, err := git.OpenRepository(ctx, repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	findOptions := repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		RepoID:        mirrorRepo.ID,
	}
	initCount, err := db.Count[repo_model.Release](t.Context(), findOptions)
	assert.NoError(t, err)
	assert.Zero(t, initCount) // no sync yet, so even though there is a tag in source repo, the mirror's release table is still empty

	assert.NoError(t, release_service.CreateRelease(ctx, gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.2",
		Target:       "master",
		Title:        "v0.2 is released",
		Note:         "v0.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}, nil, ""))

	_, err = repo_model.GetMirrorByRepoID(ctx, mirrorRepo.ID)
	assert.NoError(t, err)

	syncStepCounts := func() map[string]uint64 {
		families, err := prometheus.DefaultGatherer.Gather()
		require.NoError(t, err)
		counts := map[string]uint64{}
		for _, family := range families {
			if family.GetName() != "gitea_mirror_sync_duration_ms" {
				continue
			}
			for _, metric := range family.GetMetric() {
				labels := map[string]string{}
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				if labels["owner"] == mirrorRepo.OwnerName && labels["repo"] == mirrorRepo.Name {
					counts[labels["step"]] = metric.GetHistogram().GetSampleCount()
				}
			}
		}
		return counts
	}
	before := syncStepCounts()

	ok := mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.True(t, ok)

	mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	assert.Equal(t, mirror.UpdatedUnix, mirror.LastSyncUnix)

	// actually there is a tag in the source repo, so after "sync", that tag will also come into the mirror
	initCount++

	count, err := db.Count[repo_model.Release](t.Context(), findOptions)
	assert.NoError(t, err)
	assert.Equal(t, initCount+1, count)

	release, err := repo_model.GetRelease(t.Context(), repo.ID, "v0.2")
	assert.NoError(t, err)
	assert.NoError(t, release_service.DeleteReleaseByID(ctx, repo, release, user, true))

	ok = mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.True(t, ok)

	count, err = db.Count[repo_model.Release](t.Context(), findOptions)
	assert.NoError(t, err)
	assert.Equal(t, initCount, count)

	mirror = unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	lastMirrorSync := mirror.LastSyncUnix
	after := syncStepCounts()
	for _, step := range []string{"fetch", "wiki", "total"} {
		assert.Equal(t, before[step]+2, after[step], step)
	}

	require.True(t, repo_service.HasWiki(ctx, mirrorRepo))
	require.NoError(t, git.ManagedRemoteRemove(ctx, mirrorRepo.WikiStorageRepo(), mirror.GetRemoteName()))
	require.NoError(t, git.ManagedRemoteAdd(ctx, mirrorRepo.WikiStorageRepo(), mirror.GetRemoteName(), repoPath+"-missing", git.RemoteOptionMirrorFetch))
	assert.False(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))
	after = syncStepCounts()
	for _, step := range []string{"fetch", "wiki", "total"} {
		assert.Equal(t, before[step]+3, after[step], step)
	}

	assert.NoError(t, mirror_service.UpdateAddress(ctx, mirror, repoPath+"-missing"))

	ok = mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.False(t, ok)

	mirror = unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	assert.Equal(t, lastMirrorSync, mirror.LastSyncUnix)
	after = syncStepCounts()
	assert.Equal(t, before["fetch"]+4, after["fetch"])
	assert.Equal(t, before["wiki"]+3, after["wiki"])
	assert.Equal(t, before["total"]+4, after["total"])
}

// TestMirrorPullSSRFRevalidation ensures a pull mirror re-validates its remote URL against
// the migration allow/block list on every sync, so a mirror whose (network) remote now
// points at a disallowed internal host is never fetched.
func TestMirrorPullSSRFRevalidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "http://"+strings.Replace(r.Host, "127.0.0.1", "0.0.0.0", 1)+r.URL.RequestURI(), http.StatusFound)
	}))
	defer server.Close()

	mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: 5})
	require.NoError(t, mirror_service.UpdateAddress(ctx, mirror, server.URL+"/repo.git"))
	assert.False(t, mirror_service.SyncPullMirror(ctx, mirror.RepoID))
	assert.EqualValues(t, 1, requests.Load(), "the git proxy must deny the redirect to a reserved address")

	defer test.MockVariableValue(&setting.Migrations.AllowedHostList, "")()
	assert.False(t, mirror_service.SyncPullMirror(ctx, mirror.RepoID))
	assert.EqualValues(t, 1, requests.Load(), "the disallowed internal remote must not be reached")
}
