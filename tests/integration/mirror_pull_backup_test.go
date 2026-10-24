// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	mirror_service "gitea.dev/services/mirror"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

type mirrorBackupFixture struct {
	remote string
	work   string
	repo   *repo_model.Repository
}

func gitForMirrorBackupTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	return strings.TrimSpace(string(output))
}

func newMirrorBackupFixture(t *testing.T, name string, backupLimit int, prune bool) *mirrorBackupFixture {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.Mirror.DefaultForcePushBackupLimit, backupLimit))
	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	work := filepath.Join(base, "work")
	gitForMirrorBackupTest(t, base, "init", "--bare", remote)
	gitForMirrorBackupTest(t, base, "init", work)
	gitForMirrorBackupTest(t, work, "config", "user.name", "Mirror Test")
	gitForMirrorBackupTest(t, work, "config", "user.email", "mirror@example.com")
	gitForMirrorBackupTest(t, work, "symbolic-ref", "HEAD", "refs/heads/main")
	gitForMirrorBackupTest(t, work, "commit", "--allow-empty", "-m", "initial")
	gitForMirrorBackupTest(t, work, "remote", "add", "origin", remote)
	gitForMirrorBackupTest(t, work, "push", "origin", "main")
	gitForMirrorBackupTest(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")

	opts := migration.MigrateOptions{
		RepoName:  name,
		Mirror:    true,
		CloneAddr: remote,
	}
	repo, err := repo_service.CreateRepositoryDirectly(ctx, user, user, repo_service.CreateRepoOptions{
		Name:     name,
		IsMirror: true,
		Status:   repo_model.RepositoryBeingMigrated,
	}, false)
	require.NoError(t, err)
	repo, err = repo_service.MigrateRepositoryGitData(ctx, user, repo, opts, nil)
	require.NoError(t, err)
	mirror, err := repo_model.GetMirrorByRepoID(ctx, repo.ID)
	require.NoError(t, err)
	mirror.EnablePrune = prune
	require.NoError(t, repo_model.UpdateMirror(ctx, mirror))
	require.True(t, mirror_service.SyncPullMirror(ctx, repo.ID))
	return &mirrorBackupFixture{remote: remote, work: work, repo: repo}
}

func (f *mirrorBackupFixture) commit(t *testing.T, message string) string {
	t.Helper()
	gitForMirrorBackupTest(t, f.work, "commit", "--allow-empty", "-m", message)
	gitForMirrorBackupTest(t, f.remote, "fetch", "--no-tags", f.work, "HEAD")
	return gitForMirrorBackupTest(t, f.work, "rev-parse", "HEAD")
}

func (f *mirrorBackupFixture) setRemoteRef(t *testing.T, ref, objectID string) {
	t.Helper()
	gitForMirrorBackupTest(t, f.remote, "update-ref", ref, objectID)
}

func (f *mirrorBackupFixture) deleteRemoteRef(t *testing.T, ref string) {
	t.Helper()
	gitForMirrorBackupTest(t, f.remote, "update-ref", "-d", ref)
}

func (f *mirrorBackupFixture) mirrorRef(t *testing.T, ref string) string {
	t.Helper()
	return gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "rev-parse", "--verify", ref)
}

func (f *mirrorBackupFixture) hasMirrorRef(t *testing.T, ref string) bool {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "-C", gitrepo.RepoLocalPath(f.repo), "show-ref", "--verify", "--quiet", ref)
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
	return false
}

func (f *mirrorBackupFixture) backupRefs(t *testing.T, prefix string) []string {
	t.Helper()
	listPrefix := prefix
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		listPrefix = prefix[:idx+1]
	}
	output := gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "for-each-ref", "--format=%(refname) %(objectname)", listPrefix)
	if output == "" {
		return nil
	}
	var refs []string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			refs = append(refs, line)
		}
	}
	return refs
}

func TestMirrorPullBackupSyncsAllRefs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_all_refs", 5, true)
	mainOID := f.mirrorRef(t, "refs/heads/main")
	f.setRemoteRef(t, "refs/meta/config", mainOID)
	gitForMirrorBackupTest(t, f.work, "notes", "add", "-m", "mirror note", "HEAD")
	gitForMirrorBackupTest(t, f.work, "push", "origin", "refs/notes/commits:refs/notes/commits")
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, mainOID, f.mirrorRef(t, "refs/meta/config"))
	require.Equal(t, gitForMirrorBackupTest(t, f.remote, "rev-parse", "refs/notes/commits"), f.mirrorRef(t, "refs/notes/commits"))
}

func TestMirrorPullZeroBackupLimitSyncsWithoutBackups(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_zero_backup_limit", 0, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	f.setRemoteRef(t, "refs/heads/deleted", firstOID)
	f.setRemoteRef(t, "refs/tags/replaced", firstOID)
	f.setRemoteRef(t, "refs/tags/deleted", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	f.setRemoteRef(t, "refs/tags/replaced", secondOID)
	f.setRemoteRef(t, "refs/notes/commits", secondOID)
	f.deleteRemoteRef(t, "refs/heads/deleted")
	f.deleteRemoteRef(t, "refs/tags/deleted")
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/heads/main"))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/tags/replaced"))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/notes/commits"))
	require.False(t, f.hasMirrorRef(t, "refs/heads/deleted"))
	require.False(t, f.hasMirrorRef(t, "refs/tags/deleted"))
	require.Empty(t, f.backupRefs(t, "refs/heads/mirror-backup/"))
	require.Empty(t, f.backupRefs(t, "refs/tags/mirror-backup/"))
}

func TestMirrorPullBackupSurvivesDisabledBackupPrune(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_survives_disabled", 5, true)
	oldOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", oldOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	backupPrefix := "refs/heads/mirror-backup/branch/main-"
	backups := f.backupRefs(t, backupPrefix)
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], firstOID)

	setting.Mirror.DefaultForcePushBackupLimit = 0
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", oldOID)
	thirdOID := f.commit(t, "disabled backup history")
	f.setRemoteRef(t, "refs/heads/main", thirdOID)
	f.setRemoteRef(t, "refs/heads/deleted", secondOID)
	f.setRemoteRef(t, "refs/tags/deleted", secondOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	f.deleteRemoteRef(t, "refs/heads/deleted")
	f.deleteRemoteRef(t, "refs/tags/deleted")
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))

	require.Equal(t, thirdOID, f.mirrorRef(t, "refs/heads/main"))
	require.False(t, f.hasMirrorRef(t, "refs/heads/deleted"))
	require.False(t, f.hasMirrorRef(t, "refs/tags/deleted"))
	require.Equal(t, backups, f.backupRefs(t, backupPrefix))
	require.Empty(t, f.backupRefs(t, "refs/heads/mirror-backup/branch/deleted-"))
	require.Empty(t, f.backupRefs(t, "refs/tags/mirror-backup/tag/deleted-"))
}

func TestMirrorPullBackupFetchFailurePreservesRefs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_fetch_failure", 5, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	backups := f.backupRefs(t, "refs/heads/mirror-backup/branch/main-")
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], firstOID)
	last, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.NoError(t, os.Rename(f.remote, f.remote+"-unavailable"))
	require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/heads/main"))
	require.Equal(t, backups, f.backupRefs(t, "refs/heads/mirror-backup/branch/main-"))
	current, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.Equal(t, last.LastSyncUnix, current.LastSyncUnix)
}

func TestMirrorPullBackupLockFailurePreservesRefsAndRetries(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_lock_failure", 5, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	backups := f.backupRefs(t, "refs/heads/mirror-backup/branch/main-")
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], firstOID)
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	thirdOID := f.commit(t, "another history")
	f.setRemoteRef(t, "refs/heads/main", thirdOID)

	lockPath := filepath.Join(gitrepo.RepoLocalPath(f.repo), "refs", "heads", "main.lock")
	require.NoError(t, os.WriteFile(lockPath, nil, 0o600))
	last, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/heads/main"))
	require.Equal(t, backups, f.backupRefs(t, "refs/heads/mirror-backup/branch/main-"))
	current, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.Equal(t, last.LastSyncUnix, current.LastSyncUnix)
	require.NoError(t, os.Remove(lockPath))
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, thirdOID, f.mirrorRef(t, "refs/heads/main"))
	require.Contains(t, f.backupRefs(t, "refs/heads/mirror-backup/branch/main-"), backups[0])
}

func TestMirrorPullBackupDirectoryFileTransitions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	for _, tc := range []struct {
		name    string
		refType string
		old     string
		new     string
	}{
		{"branch_file_to_directory", "heads", "topic", "topic/nested"},
		{"branch_directory_to_file", "heads", "topic/nested", "topic"},
		{"tag_file_to_directory", "tags", "release", "release/nested"},
		{"tag_directory_to_file", "tags", "release/nested", "release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMirrorBackupFixture(t, "test_mirror_backup_"+tc.name, 5, true)
			objectID := f.mirrorRef(t, "refs/heads/main")
			oldRef := "refs/" + tc.refType + "/" + tc.old
			newRef := "refs/" + tc.refType + "/" + tc.new
			f.setRemoteRef(t, oldRef, objectID)
			require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
			require.Equal(t, objectID, f.mirrorRef(t, oldRef))
			f.deleteRemoteRef(t, oldRef)
			f.setRemoteRef(t, newRef, objectID)
			require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
			require.False(t, f.hasMirrorRef(t, oldRef))
			require.Equal(t, objectID, f.mirrorRef(t, newRef))
			backupType := "branch"
			if tc.refType == "tags" {
				backupType = "tag"
			}
			backups := f.backupRefs(t, "refs/"+tc.refType+"/mirror-backup/"+backupType+"/"+tc.old+"-")
			require.Len(t, backups, 1)
			require.Contains(t, backups[0], objectID)
		})
	}
}

func TestMirrorPullBackupPruneDisabledDirectoryFileConflict(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_prune_disabled_conflict", 5, false)
	objectID := f.mirrorRef(t, "refs/heads/main")
	f.setRemoteRef(t, "refs/heads/topic", objectID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	f.deleteRemoteRef(t, "refs/heads/topic")
	f.setRemoteRef(t, "refs/heads/topic/nested", objectID)
	require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, objectID, f.mirrorRef(t, "refs/heads/topic"))
	require.False(t, f.hasMirrorRef(t, "refs/heads/topic/nested"))
	require.Empty(t, f.backupRefs(t, "refs/heads/mirror-backup/branch/topic-"))
}

func TestMirrorPullBackupRecordsOldObjectIDs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_old_oids", 5, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Contains(t, f.backupRefs(t, "refs/heads/mirror-backup/branch/main-")[0], firstOID)

	f.setRemoteRef(t, "refs/heads/deleted", firstOID)
	f.setRemoteRef(t, "refs/tags/replaced", firstOID)
	f.setRemoteRef(t, "refs/tags/deleted", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	f.deleteRemoteRef(t, "refs/heads/deleted")
	f.setRemoteRef(t, "refs/tags/replaced", secondOID)
	f.deleteRemoteRef(t, "refs/tags/deleted")
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	for _, prefix := range []string{
		"refs/heads/mirror-backup/branch/deleted-",
		"refs/tags/mirror-backup/tag/replaced-",
		"refs/tags/mirror-backup/tag/deleted-",
	} {
		backups := f.backupRefs(t, prefix)
		require.Len(t, backups, 1, prefix)
		require.Contains(t, backups[0], firstOID, prefix)
	}
}

func TestMirrorPullBackupIgnoresReplacementRefsForAncestry(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	f := newMirrorBackupFixture(t, "test_mirror_backup_replace_ancestry", 5, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	oldOID := f.commit(t, "upstream old history")
	f.setRemoteRef(t, "refs/heads/main", oldOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	newOID := f.commit(t, "alternate upstream history")
	require.NotEqual(t, oldOID, newOID)
	gitForMirrorBackupTest(t, f.work, "replace", "--graft", newOID, oldOID)
	replaceRef := "refs/replace/" + newOID
	gitForMirrorBackupTest(t, f.work, "push", "origin", replaceRef+":"+replaceRef)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, oldOID, f.mirrorRef(t, "refs/heads/main"))
	require.True(t, f.hasMirrorRef(t, replaceRef))
	gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "fetch", "--no-tags", f.work, "HEAD")
	withReplace := exec.CommandContext(t.Context(), "git", "-C", gitrepo.RepoLocalPath(f.repo), "merge-base", "--is-ancestor", oldOID, newOID)
	require.NoError(t, withReplace.Run())
	withoutReplace := exec.CommandContext(t.Context(), "git", "-C", gitrepo.RepoLocalPath(f.repo), "merge-base", "--is-ancestor", oldOID, newOID)
	withoutReplace.Env = append(os.Environ(), "GIT_NO_REPLACE_OBJECTS=1")
	var exitErr *exec.ExitError
	require.ErrorAs(t, withoutReplace.Run(), &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())

	f.setRemoteRef(t, "refs/heads/main", newOID)
	f.deleteRemoteRef(t, replaceRef)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, newOID, f.mirrorRef(t, "refs/heads/main"))
	backups := f.backupRefs(t, "refs/heads/mirror-backup/branch/main-")
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], oldOID)
}

func TestMirrorPullBackupRetentionKeepsNewestAcrossSequences(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	for _, limit := range []int{-1, 1} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			f := newMirrorBackupFixture(t, "test_mirror_backup_retention_"+strconv.Itoa(limit), limit, true)
			initialOID := f.mirrorRef(t, "refs/heads/main")
			firstOID := f.commit(t, "first branch tip")
			f.setRemoteRef(t, "refs/heads/main", firstOID)
			f.setRemoteRef(t, "refs/tags/release", firstOID)
			require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
			gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
			secondOID := f.commit(t, "alternate branch tip")
			futureTimestamp := time.Now().UTC().Add(time.Hour).Format("20060102-150405")
			prefixes := []string{"refs/heads/mirror-backup/branch/main-", "refs/tags/mirror-backup/tag/release-"}
			for _, prefix := range prefixes {
				gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "update-ref", prefix+futureTimestamp, initialOID)
			}

			for i := range 12 {
				oldOID, newOID := firstOID, secondOID
				if i%2 == 1 {
					oldOID, newOID = secondOID, firstOID
				}
				f.setRemoteRef(t, "refs/heads/main", newOID)
				f.setRemoteRef(t, "refs/tags/release", newOID)
				require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID), "sync %d", i)
				require.Equal(t, newOID, f.mirrorRef(t, "refs/heads/main"))
				for _, prefix := range prefixes {
					backups := f.backupRefs(t, prefix)
					newest := prefix + futureTimestamp + "-" + strconv.Itoa(i+2) + " " + oldOID
					if limit == -1 {
						require.Len(t, backups, i+2, "sync %d", i)
						require.Contains(t, backups, newest)
					} else {
						require.Equal(t, []string{newest}, backups, "sync %d", i)
					}
				}
			}
		})
	}
}

func TestMirrorPullBackupRejectsReservedUpstreamRefs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	for _, tc := range []struct {
		name string
		ref  string
	}{
		{"branch", "refs/heads/mirror-backup/branch/upstream"},
		{"tag", "refs/tags/mirror-backup/tag/upstream"},
		{"branch_ancestor", "refs/heads/mirror-backup"},
		{"tag_ancestor", "refs/tags/mirror-backup"},
		{"fetch_namespace", "refs/mirror-fetch/upstream"},
		{"fetch_namespace_root", "refs/mirror-fetch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMirrorBackupFixture(t, "test_mirror_backup_reserved_"+tc.name, 5, true)
			oldOID := f.mirrorRef(t, "refs/heads/main")
			newOID := f.commit(t, "upstream new main")
			f.setRemoteRef(t, "refs/heads/main", newOID)
			f.setRemoteRef(t, tc.ref, newOID)
			last, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
			require.NoError(t, err)
			require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
			require.Equal(t, oldOID, f.mirrorRef(t, "refs/heads/main"))
			require.False(t, f.hasMirrorRef(t, tc.ref))
			require.Empty(t, f.backupRefs(t, "refs/heads/mirror-backup/"))
			current, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
			require.NoError(t, err)
			require.Equal(t, last.LastSyncUnix, current.LastSyncUnix)
		})
	}
}

func TestMirrorPullBackupPhaseFailureRetainsRefsAndRetries(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	requireMirrorBackupReferenceTransactionHook(t)
	f := newMirrorBackupFixture(t, "test_mirror_backup_phase_failure", 1, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)

	prefix := "refs/heads/mirror-backup/branch/main-"
	previousBackup := prefix + time.Now().UTC().Add(-time.Hour).Format("20060102-150405")
	gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "update-ref", previousBackup, initialOID)
	hook := installMirrorBackupRejectingHook(t, f, "refs/heads/main", secondOID)
	last, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, firstOID, f.mirrorRef(t, "refs/heads/main"))
	backups := f.backupRefs(t, prefix)
	require.Len(t, backups, 2)
	require.Contains(t, backups, previousBackup+" "+initialOID)
	var newBackup string
	for _, backup := range backups {
		if strings.Fields(backup)[1] == firstOID {
			newBackup = backup
		}
	}
	require.NotEmpty(t, newBackup)
	backupName := strings.TrimPrefix(strings.Fields(newBackup)[0], "refs/heads/")
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: f.repo.ID, Name: backupName})
	require.False(t, branch.IsDeleted)
	current, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.Equal(t, last.LastSyncUnix, current.LastSyncUnix)
	require.NoError(t, os.Remove(hook))
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/heads/main"))
	remaining := f.backupRefs(t, prefix)
	require.Len(t, remaining, 1)
	require.Contains(t, remaining[0], firstOID)
}

func TestMirrorPullBackupDirectoryFilePhaseFailure(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	requireMirrorBackupReferenceTransactionHook(t)
	f := newMirrorBackupFixture(t, "test_mirror_backup_df_phase_failure", 5, true)
	objectID := f.mirrorRef(t, "refs/heads/main")
	oldRef := "refs/heads/topic"
	newRef := "refs/heads/topic/nested"
	f.setRemoteRef(t, oldRef, objectID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	f.deleteRemoteRef(t, oldRef)
	f.setRemoteRef(t, newRef, objectID)
	hook := installMirrorBackupRejectingHook(t, f, newRef, objectID)
	require.False(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.False(t, f.hasMirrorRef(t, oldRef))
	require.False(t, f.hasMirrorRef(t, newRef))
	backups := f.backupRefs(t, "refs/heads/mirror-backup/branch/topic-")
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], objectID)
	backupName := strings.TrimPrefix(strings.Fields(backups[0])[0], "refs/heads/")
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: f.repo.ID, Name: backupName})
	require.False(t, branch.IsDeleted)
	require.NoError(t, os.Remove(hook))
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.False(t, f.hasMirrorRef(t, oldRef))
	require.Equal(t, objectID, f.mirrorRef(t, newRef))
}

func TestMirrorPullBackupRejectsStaleLiveRef(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	requireMirrorBackupReferenceTransactionHook(t)
	f := newMirrorBackupFixture(t, "test_mirror_backup_stale_live", 5, true)
	initialOID := f.mirrorRef(t, "refs/heads/main")
	firstOID := f.commit(t, "first history")
	f.setRemoteRef(t, "refs/heads/main", firstOID)
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	gitForMirrorBackupTest(t, f.work, "reset", "--hard", initialOID)
	secondOID := f.commit(t, "alternate history")
	f.setRemoteRef(t, "refs/heads/main", secondOID)
	thirdOID := f.commit(t, "concurrent history")
	gitForMirrorBackupTest(t, gitrepo.RepoLocalPath(f.repo), "fetch", "--no-tags", f.work, "HEAD")

	hook := filepath.Join(gitrepo.RepoLocalPath(f.repo), "hooks", "reference-transaction")
	hookTrace := filepath.Join(t.TempDir(), "hook-trace")
	_, err := os.Stat(hook)
	require.ErrorIs(t, err, os.ErrNotExist)
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = committed ] || exit 0\nwhile read old new ref; do\n  printf '%%s %%s\\n' \"$1\" \"$ref\" >> %q\n  case \"$ref\" in refs/heads/mirror-backup/branch/main-*)\n    git -C %q -c core.hooksPath=/dev/null update-ref refs/heads/main %q %q >> %q 2>&1\n    exit $? ;;\n  esac\ndone\nexit 0\n", hookTrace, gitrepo.RepoLocalPath(f.repo), thirdOID, firstOID, hookTrace)
	require.NoError(t, os.WriteFile(hook, []byte(script), 0o755))
	last, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	synced := mirror_service.SyncPullMirror(t.Context(), f.repo.ID)
	trace, _ := os.ReadFile(hookTrace)
	require.False(t, synced, "%s", trace)
	require.Equal(t, thirdOID, f.mirrorRef(t, "refs/heads/main"))
	backups := f.backupRefs(t, "refs/heads/mirror-backup/branch/main-")
	require.Len(t, backups, 1)
	require.Contains(t, backups[0], firstOID)
	current, err := repo_model.GetMirrorByRepoID(t.Context(), f.repo.ID)
	require.NoError(t, err)
	require.Equal(t, last.LastSyncUnix, current.LastSyncUnix)
	require.NoError(t, os.Remove(hook))
	require.True(t, mirror_service.SyncPullMirror(t.Context(), f.repo.ID))
	require.Equal(t, secondOID, f.mirrorRef(t, "refs/heads/main"))
}

func requireMirrorBackupReferenceTransactionHook(t *testing.T) {
	t.Helper()
	version := gitForMirrorBackupTest(t, ".", "--version")
	var major, minor int
	_, err := fmt.Sscanf(version, "git version %d.%d", &major, &minor)
	require.NoError(t, err)
	if major < 2 || (major == 2 && minor < 29) {
		t.Skip("reference-transaction hook requires Git 2.29")
	}
}

func installMirrorBackupRejectingHook(t *testing.T, f *mirrorBackupFixture, ref, newID string) string {
	t.Helper()
	hook := filepath.Join(gitrepo.RepoLocalPath(f.repo), "hooks", "reference-transaction")
	_, err := os.Stat(hook)
	require.ErrorIs(t, err, os.ErrNotExist)
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = prepared ] || exit 0\nwhile read old new ref; do\n  [ \"$ref\" = %q ] && [ \"$new\" = %q ] && exit 1\ndone\nexit 0\n", ref, newID)
	require.NoError(t, os.WriteFile(hook, []byte(script), 0o755))
	return hook
}
