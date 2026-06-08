// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	mirror_service "gitea.dev/services/mirror"
	release_service "gitea.dev/services/release"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMirrorPull(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

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

	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
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

	assert.NoError(t, release_service.CreateRelease(gitRepo, &repo_model.Release{
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
	assert.NoError(t, mirror_service.UpdateAddress(ctx, mirror, repoPath+"-missing"))

	ok = mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.False(t, ok)

	mirror = unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	assert.Equal(t, lastMirrorSync, mirror.LastSyncUnix)
}

func TestMirrorPullForcePushBackupRefIsNotRestored(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	sourceRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	sourceRepoPath := repo_model.RepoPath(user.Name, sourceRepo.Name)
	forcePushBackup := true
	oldForcePushBackupLimit := setting.Mirror.DefaultForcePushBackupLimit
	setting.Mirror.DefaultForcePushBackupLimit = 1
	defer func() {
		setting.Mirror.DefaultForcePushBackupLimit = oldForcePushBackupLimit
	}()

	opts := migration.MigrateOptions{
		RepoName:        "test_mirror_force_push_backup",
		Description:     "Test mirror force push backup",
		Private:         false,
		Mirror:          true,
		CloneAddr:       sourceRepoPath,
		ForcePushBackup: &forcePushBackup,
	}

	mirrorRepo, err := repo_service.CreateRepositoryDirectly(ctx, user, user, repo_service.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	}, false)
	require.NoError(t, err)

	mirrorRepo, err = repo_service.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	require.NoError(t, err)

	workDir := t.TempDir()
	require.NoError(t, git.Clone(ctx, sourceRepoPath, workDir, git.CloneRepoOptions{}))

	writeCommitAndUpdateSource := func(fileName, content, refspec string) {
		require.NoError(t, os.WriteFile(filepath.Join(workDir, fileName), []byte(content), 0o644))
		require.NoError(t, gitAddChangesDeprecated(ctx, workDir, true))
		signature := git.Signature{
			Email: "test@test.test",
			Name:  "test",
		}
		require.NoError(t, gitCommitChangesDeprecated(ctx, workDir, gitCommitChangesOptions{
			Committer: &signature,
			Author:    &signature,
			Message:   "update " + fileName,
		}))
		_, _, err := gitcmd.NewCommand("fetch").
			AddDynamicArguments(workDir, refspec).
			WithDir(sourceRepoPath).
			RunStdString(ctx)
		require.NoError(t, err)
	}

	writeCommitAndUpdateSource("mirror-force-push-a.txt", "first", "master:refs/heads/master")
	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))

	_, _, err = gitcmd.NewCommand("reset", "--hard", "HEAD~1").
		WithDir(workDir).
		RunStdString(ctx)
	require.NoError(t, err)
	actionCount := countMirrorBackupActions(t, mirrorRepo.ID)
	writeCommitAndUpdateSource("mirror-force-push-b.txt", "second", "+master:refs/heads/master")
	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))
	assert.Equal(t, actionCount+1, countMirrorBackupActions(t, mirrorRepo.ID))

	_ = assertSingleActiveBackupBranch(t, mirrorRepo.ID)
	_, _, err = gitcmd.NewCommand("reset", "--hard", "HEAD~1").
		WithDir(workDir).
		RunStdString(ctx)
	require.NoError(t, err)
	writeCommitAndUpdateSource("mirror-force-push-c.txt", "third", "+master:refs/heads/master")
	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))
	assert.Equal(t, actionCount+2, countMirrorBackupActions(t, mirrorRepo.ID))
	backupBranch := assertSingleActiveBackupBranch(t, mirrorRepo.ID)

	mirrorGitRepo, err := gitrepo.OpenRepository(ctx, mirrorRepo)
	require.NoError(t, err)
	defer mirrorGitRepo.Close()
	assert.True(t, gitrepo.IsBranchExist(ctx, mirrorRepo, backupBranch.Name))

	require.NoError(t, repo_service.DeleteBranch(ctx, user, mirrorRepo, mirrorGitRepo, backupBranch.Name))
	assert.False(t, gitrepo.IsBranchExist(ctx, mirrorRepo, backupBranch.Name))

	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))
	assert.False(t, gitrepo.IsBranchExist(ctx, mirrorRepo, backupBranch.Name))

	deletedBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{
		RepoID: mirrorRepo.ID,
		Name:   backupBranch.Name,
	})
	assert.True(t, deletedBranch.IsDeleted)

	tagName := "mirror-backup-tag"
	_, _, err = gitcmd.NewCommand("tag", "-f").AddDashesAndList(tagName, "HEAD").
		WithDir(workDir).
		RunStdString(ctx)
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("fetch").
		AddDynamicArguments(workDir, "refs/tags/"+tagName+":refs/tags/"+tagName).
		WithDir(sourceRepoPath).
		RunStdString(ctx)
	require.NoError(t, err)
	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))

	writeCommitAndUpdateSource("mirror-force-push-tag.txt", "tag", "master:refs/heads/master")
	_, _, err = gitcmd.NewCommand("tag", "-f").AddDashesAndList(tagName, "HEAD").
		WithDir(workDir).
		RunStdString(ctx)
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("fetch").
		AddDynamicArguments(workDir, "+refs/tags/"+tagName+":refs/tags/"+tagName).
		WithDir(sourceRepoPath).
		RunStdString(ctx)
	require.NoError(t, err)
	require.True(t, mirror_service.SyncPullMirror(ctx, mirrorRepo.ID))
	assertSingleRefWithPrefix(t, mirrorRepo, "refs/tags/mirror-backup/tag/"+tagName+"-")
}

func assertSingleActiveBackupBranch(t *testing.T, repoID int64) *git_model.Branch {
	t.Helper()

	branches, err := db.Find[git_model.Branch](t.Context(), git_model.FindBranchOptions{
		ListOptions: db.ListOptionsAll,
		RepoID:      repoID,
	})
	require.NoError(t, err)

	var backupBranches []*git_model.Branch
	for _, branch := range branches {
		if !branch.IsDeleted && strings.HasPrefix(branch.Name, "mirror-backup/branch/") {
			backupBranches = append(backupBranches, branch)
		}
	}
	require.Len(t, backupBranches, 1)
	return backupBranches[0]
}

func countMirrorBackupActions(t *testing.T, repoID int64) int64 {
	t.Helper()

	count, err := db.GetEngine(t.Context()).Count(&activities_model.Action{
		RepoID: repoID,
		OpType: activities_model.ActionMirrorSyncBackup,
	})
	require.NoError(t, err)
	return count
}

func assertSingleRefWithPrefix(t *testing.T, repo *repo_model.Repository, prefix string) {
	t.Helper()

	listPrefix := prefix
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		listPrefix = prefix[:idx+1]
	}
	stdout, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname)").
		AddDynamicArguments(listPrefix).
		WithDir(repo.RepoPath()).
		RunStdString(t.Context())
	require.NoError(t, err)

	var refs []string
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			refs = append(refs, line)
		}
	}
	require.Len(t, refs, 1)
}
