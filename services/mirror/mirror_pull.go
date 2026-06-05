// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	system_model "gitea.dev/models/system"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/proxy"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	feed_service "gitea.dev/services/feed"
	"gitea.dev/services/migrations"
	notify_service "gitea.dev/services/notify"
	repo_service "gitea.dev/services/repository"
)

// UpdateAddress writes new address to Git repository and database
func UpdateAddress(ctx context.Context, m *repo_model.Mirror, addr string) error {
	u, err := giturl.ParseGitURL(addr)
	if err != nil {
		return fmt.Errorf("invalid addr: %v", err)
	}

	remoteName := m.GetRemoteName()
	repo := m.GetRepository(ctx)
	// Remove old remote
	err = gitrepo.GitRemoteRemove(ctx, repo, remoteName)
	if err != nil && !git.IsRemoteNotExistError(err) {
		return err
	}

	err = gitrepo.GitRemoteAdd(ctx, repo, remoteName, addr, gitrepo.RemoteOptionMirrorFetch)
	if err != nil && !git.IsRemoteNotExistError(err) {
		return err
	}

	if repo_service.HasWiki(ctx, m.Repo) {
		wikiRemotePath := repo_module.WikiRemoteURL(ctx, addr)
		// Remove old remote of wiki
		err = gitrepo.GitRemoteRemove(ctx, repo.WikiStorageRepo(), remoteName)
		if err != nil && !git.IsRemoteNotExistError(err) {
			return err
		}

		err = gitrepo.GitRemoteAdd(ctx, repo.WikiStorageRepo(), remoteName, wikiRemotePath, gitrepo.RemoteOptionMirrorFetch)
		if err != nil && !git.IsRemoteNotExistError(err) {
			return err
		}
	}

	// erase authentication before storing in database
	u.User = nil
	m.Repo.OriginalURL = u.String()
	return repo_model.UpdateRepositoryColsNoAutoTime(ctx, m.Repo, "original_url")
}

func pruneBrokenReferences(ctx context.Context, m *repo_model.Mirror, gitRepo gitrepo.Repository, timeout time.Duration) error {
	cmd := gitcmd.NewCommand("remote", "prune").AddDynamicArguments(m.GetRemoteName()).WithTimeout(timeout)
	stdout, _, pruneErr := gitrepo.RunCmdString(ctx, gitRepo, cmd)
	if pruneErr != nil {
		// sanitize the output, since it may contain the remote address, which may contain a password
		stderrMessage := util.SanitizeCredentialURLs(pruneErr.Stderr())
		stdoutMessage := util.SanitizeCredentialURLs(stdout)

		log.Error("Failed to prune mirror repository %s references:\nStdout: %s\nStderr: %s\nErr: %v", gitRepo.RelativePath(), stdoutMessage, stderrMessage, pruneErr)
		desc := fmt.Sprintf("Failed to prune mirror repository (%s) references: %s", m.Repo.FullName(), stderrMessage)
		if err := system_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return pruneErr
}

// checkRecoverableSyncError takes an error message from a git fetch command and returns false if it should be a fatal/blocking error
func checkRecoverableSyncError(stderrMessage string) bool {
	switch {
	case strings.Contains(stderrMessage, "unable to resolve reference") && strings.Contains(stderrMessage, "reference broken"):
		return true
	case strings.Contains(stderrMessage, "remote error") && strings.Contains(stderrMessage, "not our ref"):
		return true
	case strings.Contains(stderrMessage, "cannot lock ref") && strings.Contains(stderrMessage, "but expected"):
		return true
	case strings.Contains(stderrMessage, "cannot lock ref") && strings.Contains(stderrMessage, "unable to resolve reference"):
		return true
	case strings.Contains(stderrMessage, "Unable to create") && strings.Contains(stderrMessage, ".lock"):
		return true
	default:
		return false
	}
}

const (
	temporaryFetchBranchPrefix  = "refs/mirror-fetch/heads/"
	temporaryFetchTagPrefix     = "refs/mirror-fetch/tags/"
	temporaryFetchPrefix        = "refs/mirror-fetch/"
	mirrorBackupBranchPrefix    = git.BranchPrefix + "mirror-backup/branch/"
	mirrorBackupTagPrefix       = git.TagPrefix + "mirror-backup/tag/"
	mirrorBackupTimestampLayout = "20060102-150405"
)

// runSync returns true if sync finished without error.
func runSync(ctx context.Context, m *repo_model.Mirror) ([]*repo_module.SyncResult, bool) {
	log.Trace("SyncMirrors [repo: %-v]: running git remote update...", m.Repo)

	remoteURL, remoteErr := gitrepo.GitRemoteGetURL(ctx, m.Repo, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("SyncMirrors [repo: %-v]: GetRemoteURL Error %v", m.Repo, remoteErr)
		return nil, false
	}
	envs := proxy.EnvWithProxy(remoteURL.URL)
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	// use fetch but not remote update because git fetch support --tags but remote update doesn't
	cmdFetch := func() *gitcmd.Command {
		cmd := gitcmd.NewCommand("fetch")
		if m.ForcePushBackup {
			cmd.AddArguments("--no-tags", "--refmap=")
			return cmd.AddDynamicArguments(m.GetRemoteName(),
				"+refs/heads/*:"+temporaryFetchBranchPrefix+"*",
				"+refs/tags/*:"+temporaryFetchTagPrefix+"*",
			).WithTimeout(timeout).WithEnv(envs)
		}
		cmd.AddArguments("--tags")
		if m.EnablePrune {
			cmd.AddArguments("--prune")
		}
		return cmd.AddDynamicArguments(m.GetRemoteName()).WithTimeout(timeout).WithEnv(envs)
	}

	if m.ForcePushBackup {
		if err := removeRefsWithPrefix(ctx, m.Repo, temporaryFetchPrefix); err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to clear temporary fetch refs: %v", m.Repo, err)
			return nil, false
		}
		defer func() {
			if err := removeRefsWithPrefix(ctx, m.Repo, temporaryFetchPrefix); err != nil {
				log.Error("SyncMirrors [repo: %-v]: failed to clean temporary fetch refs: %v", m.Repo, err)
			}
		}()
	}

	var err error
	fetchStdout, fetchStderr, err := gitrepo.RunCmdString(ctx, m.Repo, cmdFetch())
	if err != nil {
		// sanitize the output, since it may contain the remote address, which may contain a password
		stderrMessage := util.SanitizeCredentialURLs(fetchStderr)
		stdoutMessage := util.SanitizeCredentialURLs(fetchStdout)

		// Now check if the error is a resolve reference due to broken reference
		if checkRecoverableSyncError(fetchStderr) {
			log.Warn("SyncMirrors [repo: %-v]: failed to update mirror repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
			err = nil
			// Attempt prune
			pruneErr := pruneBrokenReferences(ctx, m, m.Repo, timeout)
			if pruneErr == nil {
				// Successful prune - reattempt mirror
				fetchStdout, fetchStderr, err = gitrepo.RunCmdString(ctx, m.Repo, cmdFetch())
				if err != nil {
					// sanitize the output, since it may contain the remote address, which may contain a password
					stderrMessage = util.SanitizeCredentialURLs(fetchStderr)
					stdoutMessage = util.SanitizeCredentialURLs(fetchStdout)
				}
			}
		}

		// If there is still an error (or there always was an error)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to update mirror repository:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository (%s): %s", m.Repo.FullName(), stderrMessage)
			if err := system_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}
	if err := gitrepo.WriteCommitGraph(ctx, m.Repo); err != nil {
		log.Error("SyncMirrors [repo: %-v]: %v", m.Repo, err)
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, m.Repo)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to OpenRepository: %v", m.Repo, err)
		return nil, false
	}

	if m.ForcePushBackup {
		if err := applyFetchedMirrorRefs(ctx, m); err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to apply fetched refs with backup: %v", m.Repo, err)
			gitRepo.Close()
			return nil, false
		}
	}

	if m.LFS && setting.LFS.StartServer {
		log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)
		endpoint := lfs.DetermineEndpoint(remoteURL.String(), m.LFSEndpoint)
		lfsClient := lfs.NewClient(endpoint, migrations.NewMigrationHTTPTransport())
		if err = repo_module.StoreMissingLfsObjectsInRepository(ctx, m.Repo, gitRepo, lfsClient); err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to synchronize LFS objects for repository: %v", m.Repo.FullName(), err)
		}
	}

	log.Trace("SyncMirrors [repo: %-v]: syncing branches...", m.Repo)
	_, results, err := repo_module.SyncRepoBranchesWithRepo(ctx, m.Repo, gitRepo, 0)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to synchronize branches: %v", m.Repo, err)
	}

	log.Trace("SyncMirrors [repo: %-v]: syncing releases with tags...", m.Repo)
	tagResults, err := repo_module.SyncReleasesWithTags(ctx, m.Repo, gitRepo)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to synchronize tags to releases: %v", m.Repo, err)
	}
	results = append(results, tagResults...)
	gitRepo.Close()

	log.Trace("SyncMirrors [repo: %-v]: updating size of repository", m.Repo)
	if err := repo_module.UpdateRepoSize(ctx, m.Repo); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to update size for mirror repository: %v", m.Repo.FullName(), err)
	}

	cmdRemoteUpdatePrune := func() *gitcmd.Command {
		return gitcmd.NewCommand("remote", "update", "--prune").
			AddDynamicArguments(m.GetRemoteName()).WithTimeout(timeout).WithEnv(envs)
	}

	if repo_service.HasWiki(ctx, m.Repo) {
		log.Trace("SyncMirrors [repo: %-v Wiki]: running git remote update...", m.Repo)
		// the result of "git remote update" is in stderr
		stdout, stderr, err := gitrepo.RunCmdString(ctx, m.Repo.WikiStorageRepo(), cmdRemoteUpdatePrune())
		if err != nil {
			// sanitize the output, since it may contain the remote address, which may contain a password
			stderrMessage := util.SanitizeCredentialURLs(stderr)
			stdoutMessage := util.SanitizeCredentialURLs(stdout)

			// Now check if the error is a resolve reference due to broken reference
			if checkRecoverableSyncError(stderrMessage) {
				log.Warn("SyncMirrors [repo: %-v Wiki]: failed to update mirror wiki repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
				err = nil

				// Attempt prune
				pruneErr := pruneBrokenReferences(ctx, m, m.Repo.WikiStorageRepo(), timeout)
				if pruneErr == nil {
					// Successful prune - reattempt mirror
					stdout, stderr, err = gitrepo.RunCmdString(ctx, m.Repo.WikiStorageRepo(), cmdRemoteUpdatePrune())
					if err != nil {
						stderrMessage = util.SanitizeCredentialURLs(stderr)
						stdoutMessage = util.SanitizeCredentialURLs(stdout)
					}
				}
			}

			// If there is still an error (or there always was an error)
			if err != nil {
				log.Error("SyncMirrors [repo: %-v Wiki]: failed to update mirror repository wiki:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
				desc := fmt.Sprintf("Failed to update mirror repository wiki (%s): %s", m.Repo.FullName(), stderrMessage)
				if err := system_model.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return nil, false
			}

			if err := gitrepo.WriteCommitGraph(ctx, m.Repo.WikiStorageRepo()); err != nil {
				log.Error("SyncMirrors [repo: %-v]: %v", m.Repo, err)
			}
		}
		log.Trace("SyncMirrors [repo: %-v Wiki]: git remote update complete", m.Repo)
	}

	log.Trace("SyncMirrors [repo: %-v]: invalidating mirror branch caches...", m.Repo)
	branches, _, err := gitrepo.GetBranchesByPath(ctx, m.Repo, 0, 0)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to GetBranches: %v", m.Repo, err)
		return nil, false
	}

	for _, branch := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branch, true))
	}

	m.UpdatedUnix = timeutil.TimeStampNow()
	return results, true
}

type mirrorRef struct {
	name     string
	objectID string
}

func listRefsWithPrefix(ctx context.Context, repo gitrepo.Repository, prefix string) ([]mirrorRef, error) {
	stdout, _, err := gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("for-each-ref", "--format=%(objectname) %(refname)").
		AddDynamicArguments(prefix))
	if err != nil {
		return nil, err
	}
	var refs []mirrorRef
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		refs = append(refs, mirrorRef{
			objectID: fields[0],
			name:     fields[1],
		})
	}
	return refs, nil
}

func removeRefsWithPrefix(ctx context.Context, repo gitrepo.Repository, prefix string) error {
	refs, err := listRefsWithPrefix(ctx, repo, prefix)
	if err != nil {
		return err
	}
	slices.SortFunc(refs, func(a, b mirrorRef) int {
		return strings.Compare(b.name, a.name)
	})
	for _, ref := range refs {
		if err := gitrepo.RemoveRef(ctx, repo, ref.name); err != nil {
			return err
		}
	}
	return nil
}

func applyFetchedMirrorRefs(ctx context.Context, m *repo_model.Mirror) error {
	remoteBranches, err := refsByName(ctx, m.Repo, temporaryFetchBranchPrefix, git.BranchPrefix, "")
	if err != nil {
		return err
	}
	remoteTags, err := refsByName(ctx, m.Repo, temporaryFetchTagPrefix, git.TagPrefix, "")
	if err != nil {
		return err
	}
	localBranches, err := refsByName(ctx, m.Repo, git.BranchPrefix, git.BranchPrefix, mirrorBackupBranchPrefix)
	if err != nil {
		return err
	}
	localTags, err := refsByName(ctx, m.Repo, git.TagPrefix, git.TagPrefix, mirrorBackupTagPrefix)
	if err != nil {
		return err
	}

	for refName, remoteRef := range remoteBranches {
		localRef, hasLocal := localBranches[refName]
		if !hasLocal {
			if err := gitrepo.UpdateRef(ctx, m.Repo, refName, remoteRef.objectID); err != nil {
				return err
			}
			continue
		}
		if localRef.objectID == remoteRef.objectID {
			continue
		}
		canReplace, err := canFastForwardRef(ctx, m.Repo, localRef.objectID, remoteRef.objectID)
		if err != nil {
			return err
		}
		if !canReplace {
			if err := backupRef(ctx, m, "branch", strings.TrimPrefix(refName, git.BranchPrefix), localRef.objectID); err != nil {
				return err
			}
		}
		if err := gitrepo.UpdateRef(ctx, m.Repo, refName, remoteRef.objectID); err != nil {
			return err
		}
	}

	for refName, remoteRef := range remoteTags {
		localRef, hasLocal := localTags[refName]
		if !hasLocal {
			if err := gitrepo.UpdateRef(ctx, m.Repo, refName, remoteRef.objectID); err != nil {
				return err
			}
			continue
		}
		if localRef.objectID == remoteRef.objectID {
			continue
		}
		if err := backupRef(ctx, m, "tag", strings.TrimPrefix(refName, git.TagPrefix), localRef.objectID); err != nil {
			return err
		}
		if err := gitrepo.UpdateRef(ctx, m.Repo, refName, remoteRef.objectID); err != nil {
			return err
		}
	}

	if m.EnablePrune {
		for refName, localRef := range localBranches {
			if _, ok := remoteBranches[refName]; ok {
				continue
			}
			if err := backupRef(ctx, m, "branch", strings.TrimPrefix(refName, git.BranchPrefix), localRef.objectID); err != nil {
				return err
			}
			if err := gitrepo.RemoveRef(ctx, m.Repo, refName); err != nil {
				return err
			}
		}
		for refName, localRef := range localTags {
			if _, ok := remoteTags[refName]; ok {
				continue
			}
			if err := backupRef(ctx, m, "tag", strings.TrimPrefix(refName, git.TagPrefix), localRef.objectID); err != nil {
				return err
			}
			if err := gitrepo.RemoveRef(ctx, m.Repo, refName); err != nil {
				return err
			}
		}
	}
	return nil
}

func refsByName(ctx context.Context, repo gitrepo.Repository, listPrefix, refPrefix, excludePrefix string) (map[string]mirrorRef, error) {
	refs, err := listRefsWithPrefix(ctx, repo, listPrefix)
	if err != nil {
		return nil, err
	}
	refsMap := make(map[string]mirrorRef, len(refs))
	for _, ref := range refs {
		if excludePrefix != "" && strings.HasPrefix(ref.name, excludePrefix) {
			continue
		}
		if !strings.HasPrefix(ref.name, listPrefix) {
			continue
		}
		refsMap[refPrefix+strings.TrimPrefix(ref.name, listPrefix)] = ref
	}
	return refsMap, nil
}

func canFastForwardRef(ctx context.Context, repo gitrepo.Repository, oldObjectID, newObjectID string) (bool, error) {
	_, _, err := gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base", "--is-ancestor").
		AddDynamicArguments(oldObjectID, newObjectID))
	if err == nil {
		return true, nil
	}
	if gitcmd.IsErrorExitCode(err, 1) {
		return false, nil
	}
	return false, err
}

func backupRef(ctx context.Context, m *repo_model.Mirror, refType, shortName, objectID string) error {
	backupRefName := generateBackupRefName(ctx, m.Repo, refType, shortName)
	if err := gitrepo.UpdateRef(ctx, m.Repo, backupRefName, objectID); err != nil {
		return err
	}
	log.Trace("SyncMirrors [repo: %-v]: created backup ref %s -> %s", m.Repo, backupRefName, objectID)
	originalRefName := git.RefNameFromBranch(shortName).String()
	if refType == "tag" {
		originalRefName = git.RefNameFromTag(shortName).String()
	}
	if err := feed_service.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: m.Repo.OwnerID,
		ActUser:   m.Repo.MustOwner(ctx),
		OpType:    activities_model.ActionMirrorSyncBackup,
		RepoID:    m.Repo.ID,
		Repo:      m.Repo,
		IsPrivate: m.Repo.IsPrivate,
		RefName:   backupRefName,
		Content:   originalRefName,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
	return pruneBackupRefs(ctx, m, refType, shortName)
}

func pruneBackupRefs(ctx context.Context, m *repo_model.Mirror, refType, shortName string) error {
	limit := setting.Mirror.DefaultForcePushBackupLimit
	refs, err := listBackupRefs(ctx, m.Repo, refType, shortName)
	if err != nil {
		return err
	}
	if len(refs) <= limit {
		return nil
	}
	slices.SortFunc(refs, func(a, b mirrorRef) int {
		return strings.Compare(b.name, a.name)
	})
	for _, ref := range refs[limit:] {
		if err := gitrepo.RemoveRef(ctx, m.Repo, ref.name); err != nil {
			return err
		}
		log.Trace("SyncMirrors [repo: %-v]: pruned backup ref %s", m.Repo, ref.name)
	}
	return nil
}

func generateBackupRefName(ctx context.Context, repo gitrepo.Repository, refType, shortName string) string {
	timestamp := time.Now().UTC().Format(mirrorBackupTimestampLayout)
	backupRefName := fmt.Sprintf("%s%s-%s", backupRefPrefix(refType), shortName, timestamp)

	if !gitrepo.IsReferenceExist(ctx, repo, backupRefName) {
		return backupRefName
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", backupRefName, i)
		if !gitrepo.IsReferenceExist(ctx, repo, candidate) {
			return candidate
		}
	}
}

func backupRefPrefix(refType string) string {
	if refType == "tag" {
		return mirrorBackupTagPrefix
	}
	return mirrorBackupBranchPrefix
}

func listBackupRefs(ctx context.Context, repo gitrepo.Repository, refType, shortName string) ([]mirrorRef, error) {
	prefix := backupRefPrefix(refType) + shortName + "-"
	listPrefix := prefix
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		listPrefix = prefix[:idx+1]
	}
	refs, err := listRefsWithPrefix(ctx, repo, listPrefix)
	if err != nil {
		return nil, err
	}
	matchingRefs := make([]mirrorRef, 0, len(refs))
	for _, ref := range refs {
		if isBackupRefForPrefix(ref.name, prefix) {
			matchingRefs = append(matchingRefs, ref)
		}
	}
	return matchingRefs, nil
}

func isBackupRefForPrefix(refName, prefix string) bool {
	suffix, ok := strings.CutPrefix(refName, prefix)
	if !ok {
		return false
	}
	if len(suffix) < len(mirrorBackupTimestampLayout) {
		return false
	}
	if _, err := time.Parse(mirrorBackupTimestampLayout, suffix[:len(mirrorBackupTimestampLayout)]); err != nil {
		return false
	}
	if len(suffix) == len(mirrorBackupTimestampLayout) {
		return true
	}
	if suffix[len(mirrorBackupTimestampLayout)] != '-' {
		return false
	}
	_, err := strconv.Atoi(suffix[len(mirrorBackupTimestampLayout)+1:])
	return err == nil
}

func getRepoPullMirrorLockKey(repoID int64) string {
	return fmt.Sprintf("repo_pull_mirror_%d", repoID)
}

// SyncPullMirror starts the sync of the pull mirror and schedules the next run.
func SyncPullMirror(ctx context.Context, repoID int64) bool {
	log.Trace("SyncMirrors [repo_id: %v]", repoID)
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst syncMirrors...
		log.Error("PANIC whilst SyncMirrors[repo_id: %d] Panic: %v\nStacktrace: %s", repoID, err, log.Stack(2))
	}()

	releaser, err := globallock.Lock(ctx, getRepoPullMirrorLockKey(repoID))
	if err != nil {
		log.Error("globallock.Lock(): %v", err)
		return false
	}
	defer releaser()

	m, err := repo_model.GetMirrorByRepoID(ctx, repoID)
	if err != nil {
		log.Error("SyncMirrors [repo_id: %v]: unable to GetMirrorByRepoID: %v", repoID, err)
		return false
	}
	repo := m.GetRepository(ctx) // force load repository of mirror

	ctx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("Syncing Mirror %s/%s", m.Repo.OwnerName, m.Repo.Name))
	defer finished()

	log.Trace("SyncMirrors [repo: %-v]: Running Sync", m.Repo)
	results, ok := runSync(ctx, m)
	if !ok {
		if err = repo_model.TouchMirror(ctx, m); err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to TouchMirror: %v", m.Repo, err)
		}
		return false
	}

	log.Trace("SyncMirrors [repo: %-v]: Scheduling next update", m.Repo)
	m.ScheduleNextUpdate()
	m.LastSyncUnix = m.UpdatedUnix
	if err = repo_model.UpdateMirror(ctx, m); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to UpdateMirror with next update date: %v", m.Repo, err)
		return false
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, m.Repo)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: unable to OpenRepository: %v", m.Repo, err)
		return false
	}
	defer gitRepo.Close()

	log.Trace("SyncMirrors [repo: %-v]: %d branches updated", m.Repo, len(results))
	if len(results) > 0 {
		if ok := checkAndUpdateEmptyRepository(ctx, m, results); !ok {
			log.Error("SyncMirrors [repo: %-v]: checkAndUpdateEmptyRepository: %v", m.Repo, err)
			return false
		}
	}

	for _, result := range results {
		// Discard GitHub pull requests, i.e. refs/pull/*
		if result.RefName.IsPull() {
			continue
		}

		// Create reference
		if result.OldCommitID == "" {
			commitID, err := gitRepo.GetRefCommitID(result.RefName.String())
			if err != nil {
				log.Error("SyncMirrors [repo: %-v]: unable to GetRefCommitID [ref_name: %s]: %v", m.Repo, result.RefName, err)
				continue
			}
			objectFormat := git.ObjectFormatFromName(m.Repo.ObjectFormatName)
			notify_service.SyncPushCommits(ctx, m.Repo.MustOwner(ctx), m.Repo, &repo_module.PushUpdateOptions{
				RefFullName: result.RefName,
				OldCommitID: objectFormat.EmptyObjectID().String(),
				NewCommitID: commitID,
			}, repo_module.NewPushCommits())
			notify_service.SyncCreateRef(ctx, m.Repo.MustOwner(ctx), m.Repo, result.RefName, commitID)
			continue
		}

		// Delete reference
		if result.NewCommitID == "" {
			notify_service.SyncDeleteRef(ctx, m.Repo.MustOwner(ctx), m.Repo, result.RefName)
			continue
		}

		// Push commits
		oldCommitID, err := gitrepo.GetFullCommitID(ctx, repo, result.OldCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get GetFullCommitID[%s]: %v", m.Repo, result.OldCommitID, err)
			continue
		}
		newCommitID, err := gitrepo.GetFullCommitID(ctx, repo, result.NewCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get GetFullCommitID [%s]: %v", m.Repo, result.NewCommitID, err)
			continue
		}
		commits, err := gitRepo.CommitsBetweenIDs(newCommitID, oldCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get CommitsBetweenIDs [new_commit_id: %s, old_commit_id: %s]: %v", m.Repo, newCommitID, oldCommitID, err)
			continue
		}

		theCommits := repo_module.GitToPushCommits(commits)
		if len(theCommits.Commits) > setting.UI.FeedMaxCommitNum {
			theCommits.Commits = theCommits.Commits[:setting.UI.FeedMaxCommitNum]
		}

		newCommit, err := gitRepo.GetCommit(newCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get commit %s: %v", m.Repo, newCommitID, err)
			continue
		}

		theCommits.HeadCommit = repo_module.CommitToPushCommit(newCommit)
		theCommits.CompareURL = m.Repo.ComposeCompareURL(oldCommitID, newCommitID)

		notify_service.SyncPushCommits(ctx, m.Repo.MustOwner(ctx), m.Repo, &repo_module.PushUpdateOptions{
			RefFullName: result.RefName,
			OldCommitID: oldCommitID,
			NewCommitID: newCommitID,
		}, theCommits)
	}
	log.Trace("SyncMirrors [repo: %-v]: done notifying updated branches/tags - now updating last commit time", m.Repo)

	isEmpty, err := gitRepo.IsEmpty()
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: unable to check empty git repo: %v", m.Repo, err)
		return false
	}
	if !isEmpty {
		// Get latest commit date and update to current repository updated time
		commitDate, err := gitrepo.GetLatestCommitTime(ctx, m.Repo)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to GetLatestCommitDate: %v", m.Repo, err)
			return false
		}

		if err = repo_model.UpdateRepositoryUpdatedTime(ctx, m.RepoID, commitDate); err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to update repository 'updated_unix': %v", m.Repo, err)
			return false
		}
	}

	// Update License
	if err = repo_service.AddRepoToLicenseUpdaterQueue(&repo_service.LicenseUpdaterOptions{
		RepoID: m.Repo.ID,
	}); err != nil {
		log.Error("SyncMirrors [repo: %-v]: unable to add repo to license updater queue: %v", m.Repo, err)
		return false
	}

	log.Trace("SyncMirrors [repo: %-v]: Successfully updated", m.Repo)

	return true
}

func checkAndUpdateEmptyRepository(ctx context.Context, m *repo_model.Mirror, results []*repo_module.SyncResult) bool {
	if !m.Repo.IsEmpty {
		return true
	}

	hasDefault := false
	hasMaster := false
	hasMain := false
	defaultBranchName := m.Repo.DefaultBranch
	if len(defaultBranchName) == 0 {
		defaultBranchName = setting.Repository.DefaultBranch
	}
	firstName := ""
	for _, result := range results {
		if !result.RefName.IsBranch() {
			continue
		}

		name := result.RefName.BranchName()
		if len(firstName) == 0 {
			firstName = name
		}

		hasDefault = hasDefault || name == defaultBranchName
		hasMaster = hasMaster || name == "master"
		hasMain = hasMain || name == "main"
	}

	if len(firstName) > 0 {
		if hasDefault {
			m.Repo.DefaultBranch = defaultBranchName
		} else if hasMaster {
			m.Repo.DefaultBranch = "master"
		} else if hasMain {
			m.Repo.DefaultBranch = "main"
		} else {
			m.Repo.DefaultBranch = firstName
		}
		// Update the git repository default branch
		if err := gitrepo.SetDefaultBranch(ctx, m.Repo, m.Repo.DefaultBranch); err != nil {
			log.Error("Failed to update default branch of underlying git repository %-v. Error: %v", m.Repo, err)
			return false
		}
		m.Repo.IsEmpty = false
		// Update the is empty and default_branch columns
		if err := repo_model.UpdateRepositoryColsWithAutoTime(ctx, m.Repo, "default_branch", "is_empty"); err != nil {
			log.Error("Failed to update default branch of repository %-v. Error: %v", m.Repo, err)
			desc := fmt.Sprintf("Failed to update default branch of repository (%s): %v", m.Repo.FullName(), err)
			if err = system_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return false
		}
	}
	return true
}
