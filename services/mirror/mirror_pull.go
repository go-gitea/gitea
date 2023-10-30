// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"fmt"
	"strings"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/proxy"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

// gitShortEmptySha Git short empty SHA
const gitShortEmptySha = "0000000"

// UpdateAddress writes new address to Git repository and database
func UpdateAddress(ctx context.Context, m *repo_model.Mirror, addr string) error {
	remoteName := m.GetRemoteName()
	repoPath := m.GetRepository(ctx).RepoPath()
	// Remove old remote
	_, _, err := git.NewCommand(ctx, "remote", "rm").AddDynamicArguments(remoteName).RunStdString(&git.RunOpts{Dir: repoPath})
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	cmd := git.NewCommand(ctx, "remote", "add").AddDynamicArguments(remoteName).AddArguments("--mirror=fetch").AddDynamicArguments(addr)
	if strings.Contains(addr, "://") && strings.Contains(addr, "@") {
		cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=fetch %s [repo_path: %s]", remoteName, util.SanitizeCredentialURLs(addr), repoPath))
	} else {
		cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=fetch %s [repo_path: %s]", remoteName, addr, repoPath))
	}
	_, _, err = cmd.RunStdString(&git.RunOpts{Dir: repoPath})
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	if m.Repo.HasWiki() {
		wikiPath := m.Repo.WikiPath()
		wikiRemotePath := repo_module.WikiRemoteURL(ctx, addr)
		// Remove old remote of wiki
		_, _, err = git.NewCommand(ctx, "remote", "rm").AddDynamicArguments(remoteName).RunStdString(&git.RunOpts{Dir: wikiPath})
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}

		cmd = git.NewCommand(ctx, "remote", "add").AddDynamicArguments(remoteName).AddArguments("--mirror=fetch").AddDynamicArguments(wikiRemotePath)
		if strings.Contains(wikiRemotePath, "://") && strings.Contains(wikiRemotePath, "@") {
			cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=fetch %s [repo_path: %s]", remoteName, util.SanitizeCredentialURLs(wikiRemotePath), wikiPath))
		} else {
			cmd.SetDescription(fmt.Sprintf("remote add %s --mirror=fetch %s [repo_path: %s]", remoteName, wikiRemotePath, wikiPath))
		}
		_, _, err = cmd.RunStdString(&git.RunOpts{Dir: wikiPath})
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}
	}

	m.Repo.OriginalURL = addr
	return repo_model.UpdateRepositoryCols(ctx, m.Repo, "original_url")
}

// mirrorSyncResult contains information of a updated reference.
// If the oldCommitID is "0000000", it means a new reference, the value of newCommitID is empty.
// If the newCommitID is "0000000", it means the reference is deleted, the value of oldCommitID is empty.
type mirrorSyncResult struct {
	refName     git.RefName
	oldCommitID string
	newCommitID string
}

// parseRemoteUpdateOutput detects create, update and delete operations of references from upstream.
// possible output example:
/*
// * [new tag]         v0.1.8     -> v0.1.8
// * [new branch]      master     -> origin/master
// - [deleted]         (none)     -> origin/test // delete a branch
// - [deleted]         (none)     -> 1 // delete a tag
//   957a993..a87ba5f  test       -> origin/test
// + f895a1e...957a993 test       -> origin/test  (forced update)
*/
// TODO: return whether it's a force update
func parseRemoteUpdateOutput(output, remoteName string) []*mirrorSyncResult {
	results := make([]*mirrorSyncResult, 0, 3)
	lines := strings.Split(output, "\n")
	for i := range lines {
		// Make sure reference name is presented before continue
		idx := strings.Index(lines[i], "-> ")
		if idx == -1 {
			continue
		}

		refName := strings.TrimSpace(lines[i][idx+3:])

		switch {
		case strings.HasPrefix(lines[i], " * [new tag]"): // new tag
			results = append(results, &mirrorSyncResult{
				refName:     git.RefNameFromTag(refName),
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " * [new branch]"): // new branch
			refName = strings.TrimPrefix(refName, remoteName+"/")
			results = append(results, &mirrorSyncResult{
				refName:     git.RefNameFromBranch(refName),
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " - "): // Delete reference
			isTag := !strings.HasPrefix(refName, remoteName+"/")
			var refFullName git.RefName
			if isTag {
				refFullName = git.RefNameFromTag(refName)
			} else {
				refFullName = git.RefNameFromBranch(strings.TrimPrefix(refName, remoteName+"/"))
			}
			results = append(results, &mirrorSyncResult{
				refName:     refFullName,
				newCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " + "): // Force update
			if idx := strings.Index(refName, " "); idx > -1 {
				refName = refName[:idx]
			}
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error("SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "...")
			if len(shas) != 2 {
				log.Error("Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     git.RefNameFromBranch(strings.TrimPrefix(refName, remoteName+"/")),
				oldCommitID: shas[0],
				newCommitID: shas[1],
			})
		case strings.HasPrefix(lines[i], "   "): // New commits of a reference
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error("SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "..")
			if len(shas) != 2 {
				log.Error("Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     git.RefNameFromBranch(strings.TrimPrefix(refName, remoteName+"/")),
				oldCommitID: shas[0],
				newCommitID: shas[1],
			})

		default:
			log.Warn("parseRemoteUpdateOutput: unexpected update line %q", lines[i])
		}
	}
	return results
}

func pruneBrokenReferences(ctx context.Context,
	m *repo_model.Mirror,
	repoPath string,
	timeout time.Duration,
	stdoutBuilder, stderrBuilder *strings.Builder,
	isWiki bool,
) error {
	wiki := ""
	if isWiki {
		wiki = "Wiki "
	}

	stderrBuilder.Reset()
	stdoutBuilder.Reset()
	pruneErr := git.NewCommand(ctx, "remote", "prune").AddDynamicArguments(m.GetRemoteName()).
		SetDescription(fmt.Sprintf("Mirror.runSync %ssPrune references: %s ", wiki, m.Repo.FullName())).
		Run(&git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath,
			Stdout:  stdoutBuilder,
			Stderr:  stderrBuilder,
		})
	if pruneErr != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()

		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		stderrMessage := util.SanitizeCredentialURLs(stderr)
		stdoutMessage := util.SanitizeCredentialURLs(stdout)

		log.Error("Failed to prune mirror repository %s%-v references:\nStdout: %s\nStderr: %s\nErr: %v", wiki, m.Repo, stdoutMessage, stderrMessage, pruneErr)
		desc := fmt.Sprintf("Failed to prune mirror repository %s'%s' references: %s", wiki, repoPath, stderrMessage)
		if err := system_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
		// this if will only be reached on a successful prune so try to get the mirror again
	}
	return pruneErr
}

// runSync returns true if sync finished without error.
func runSync(ctx context.Context, m *repo_model.Mirror) ([]*mirrorSyncResult, bool) {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	log.Trace("SyncMirrors [repo: %-v]: running git remote update...", m.Repo)

	// use fetch but not remote update because git fetch support --tags but remote update doesn't
	cmd := git.NewCommand(ctx, "fetch")
	if m.EnablePrune {
		cmd.AddArguments("--prune")
	}
	cmd.AddArguments("--tags").AddDynamicArguments(m.GetRemoteName())

	remoteURL, remoteErr := git.GetRemoteURL(ctx, repoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("SyncMirrors [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
		return nil, false
	}

	envs := proxy.EnvWithProxy(remoteURL.URL)

	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	if err := cmd.
		SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
		Run(&git.RunOpts{
			Timeout: timeout,
			Dir:     repoPath,
			Env:     envs,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()

		// sanitize the output, since it may contain the remote address, which may contain a password
		stderrMessage := util.SanitizeCredentialURLs(stderr)
		stdoutMessage := util.SanitizeCredentialURLs(stdout)

		// Now check if the error is a resolve reference due to broken reference
		if strings.Contains(stderr, "unable to resolve reference") && strings.Contains(stderr, "reference broken") {
			log.Warn("SyncMirrors [repo: %-v]: failed to update mirror repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
			err = nil

			// Attempt prune
			pruneErr := pruneBrokenReferences(ctx, m, repoPath, timeout, &stdoutBuilder, &stderrBuilder, false)
			if pruneErr == nil {
				// Successful prune - reattempt mirror
				stderrBuilder.Reset()
				stdoutBuilder.Reset()
				if err = cmd.
					SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
					Run(&git.RunOpts{
						Timeout: timeout,
						Dir:     repoPath,
						Stdout:  &stdoutBuilder,
						Stderr:  &stderrBuilder,
					}); err != nil {
					stdout := stdoutBuilder.String()
					stderr := stderrBuilder.String()

					// sanitize the output, since it may contain the remote address, which may
					// contain a password
					stderrMessage = util.SanitizeCredentialURLs(stderr)
					stdoutMessage = util.SanitizeCredentialURLs(stdout)
				}
			}
		}

		// If there is still an error (or there always was an error)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to update mirror repository:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, stderrMessage)
			if err = system_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}
	output := stderrBuilder.String()

	if err := git.WriteCommitGraph(ctx, repoPath); err != nil {
		log.Error("SyncMirrors [repo: %-v]: %v", m.Repo, err)
	}

	gitRepo, err := git.OpenRepository(ctx, repoPath)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to OpenRepository: %v", m.Repo, err)
		return nil, false
	}

	log.Trace("SyncMirrors [repo: %-v]: syncing branches...", m.Repo)
	if _, err = repo_module.SyncRepoBranchesWithRepo(ctx, m.Repo, gitRepo, 0); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to synchronize branches: %v", m.Repo, err)
	}

	log.Trace("SyncMirrors [repo: %-v]: syncing releases with tags...", m.Repo)
	if err = repo_module.SyncReleasesWithTags(ctx, m.Repo, gitRepo); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to synchronize tags to releases: %v", m.Repo, err)
	}

	if m.LFS && setting.LFS.StartServer {
		log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)
		endpoint := lfs.DetermineEndpoint(remoteURL.String(), m.LFSEndpoint)
		lfsClient := lfs.NewClient(endpoint, nil)
		if err = repo_module.StoreMissingLfsObjectsInRepository(ctx, m.Repo, gitRepo, lfsClient); err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to synchronize LFS objects for repository: %v", m.Repo, err)
		}
	}
	gitRepo.Close()

	log.Trace("SyncMirrors [repo: %-v]: updating size of repository", m.Repo)
	if err := repo_module.UpdateRepoSize(ctx, m.Repo); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to update size for mirror repository: %v", m.Repo, err)
	}

	if m.Repo.HasWiki() {
		log.Trace("SyncMirrors [repo: %-v Wiki]: running git remote update...", m.Repo)
		stderrBuilder.Reset()
		stdoutBuilder.Reset()
		if err := git.NewCommand(ctx, "remote", "update", "--prune").AddDynamicArguments(m.GetRemoteName()).
			SetDescription(fmt.Sprintf("Mirror.runSync Wiki: %s ", m.Repo.FullName())).
			Run(&git.RunOpts{
				Timeout: timeout,
				Dir:     wikiPath,
				Stdout:  &stdoutBuilder,
				Stderr:  &stderrBuilder,
			}); err != nil {
			stdout := stdoutBuilder.String()
			stderr := stderrBuilder.String()

			// sanitize the output, since it may contain the remote address, which may contain a password
			stderrMessage := util.SanitizeCredentialURLs(stderr)
			stdoutMessage := util.SanitizeCredentialURLs(stdout)

			// Now check if the error is a resolve reference due to broken reference
			if strings.Contains(stderrMessage, "unable to resolve reference") && strings.Contains(stderrMessage, "reference broken") {
				log.Warn("SyncMirrors [repo: %-v Wiki]: failed to update mirror wiki repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
				err = nil

				// Attempt prune
				pruneErr := pruneBrokenReferences(ctx, m, repoPath, timeout, &stdoutBuilder, &stderrBuilder, true)
				if pruneErr == nil {
					// Successful prune - reattempt mirror
					stderrBuilder.Reset()
					stdoutBuilder.Reset()

					if err = git.NewCommand(ctx, "remote", "update", "--prune").AddDynamicArguments(m.GetRemoteName()).
						SetDescription(fmt.Sprintf("Mirror.runSync Wiki: %s ", m.Repo.FullName())).
						Run(&git.RunOpts{
							Timeout: timeout,
							Dir:     wikiPath,
							Stdout:  &stdoutBuilder,
							Stderr:  &stderrBuilder,
						}); err != nil {
						stdout := stdoutBuilder.String()
						stderr := stderrBuilder.String()
						stderrMessage = util.SanitizeCredentialURLs(stderr)
						stdoutMessage = util.SanitizeCredentialURLs(stdout)
					}
				}
			}

			// If there is still an error (or there always was an error)
			if err != nil {
				log.Error("SyncMirrors [repo: %-v Wiki]: failed to update mirror repository wiki:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
				desc := fmt.Sprintf("Failed to update mirror repository wiki '%s': %s", wikiPath, stderrMessage)
				if err = system_model.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return nil, false
			}

			if err := git.WriteCommitGraph(ctx, wikiPath); err != nil {
				log.Error("SyncMirrors [repo: %-v]: %v", m.Repo, err)
			}
		}
		log.Trace("SyncMirrors [repo: %-v Wiki]: git remote update complete", m.Repo)
	}

	log.Trace("SyncMirrors [repo: %-v]: invalidating mirror branch caches...", m.Repo)
	branches, _, err := git.GetBranchesByPath(ctx, m.Repo.RepoPath(), 0, 0)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to GetBranches: %v", m.Repo, err)
		return nil, false
	}

	for _, branch := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branch.Name, true))
	}

	m.UpdatedUnix = timeutil.TimeStampNow()
	return parseRemoteUpdateOutput(output, m.GetRemoteName()), true
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

	m, err := repo_model.GetMirrorByRepoID(ctx, repoID)
	if err != nil {
		log.Error("SyncMirrors [repo_id: %v]: unable to GetMirrorByRepoID: %v", repoID, err)
		return false
	}
	_ = m.GetRepository(ctx) // force load repository of mirror

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
	if err = repo_model.UpdateMirror(ctx, m); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to UpdateMirror with next update date: %v", m.Repo, err)
		return false
	}

	var gitRepo *git.Repository
	if len(results) == 0 {
		log.Trace("SyncMirrors [repo: %-v]: no branches updated", m.Repo)
	} else {
		log.Trace("SyncMirrors [repo: %-v]: %d branches updated", m.Repo, len(results))
		gitRepo, err = git.OpenRepository(ctx, m.Repo.RepoPath())
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to OpenRepository: %v", m.Repo, err)
			return false
		}
		defer gitRepo.Close()

		if ok := checkAndUpdateEmptyRepository(ctx, m, gitRepo, results); !ok {
			return false
		}
	}

	for _, result := range results {
		// Discard GitHub pull requests, i.e. refs/pull/*
		if result.refName.IsPull() {
			continue
		}

		// Create reference
		if result.oldCommitID == gitShortEmptySha {
			commitID, err := gitRepo.GetRefCommitID(result.refName.String())
			if err != nil {
				log.Error("SyncMirrors [repo: %-v]: unable to GetRefCommitID [ref_name: %s]: %v", m.Repo, result.refName, err)
				continue
			}
			notify_service.SyncPushCommits(ctx, m.Repo.MustOwner(ctx), m.Repo, &repo_module.PushUpdateOptions{
				RefFullName: result.refName,
				OldCommitID: git.EmptySHA,
				NewCommitID: commitID,
			}, repo_module.NewPushCommits())
			notify_service.SyncCreateRef(ctx, m.Repo.MustOwner(ctx), m.Repo, result.refName, commitID)
			continue
		}

		// Delete reference
		if result.newCommitID == gitShortEmptySha {
			notify_service.SyncDeleteRef(ctx, m.Repo.MustOwner(ctx), m.Repo, result.refName)
			continue
		}

		// Push commits
		oldCommitID, err := git.GetFullCommitID(gitRepo.Ctx, gitRepo.Path, result.oldCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get GetFullCommitID[%s]: %v", m.Repo, result.oldCommitID, err)
			continue
		}
		newCommitID, err := git.GetFullCommitID(gitRepo.Ctx, gitRepo.Path, result.newCommitID)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get GetFullCommitID [%s]: %v", m.Repo, result.newCommitID, err)
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

		if newCommit, err := gitRepo.GetCommit(newCommitID); err != nil {
			log.Error("SyncMirrors [repo: %-v]: unable to get commit %s: %v", m.Repo, newCommitID, err)
			continue
		} else {
			theCommits.HeadCommit = repo_module.CommitToPushCommit(newCommit)
		}

		theCommits.CompareURL = m.Repo.ComposeCompareURL(oldCommitID, newCommitID)

		notify_service.SyncPushCommits(ctx, m.Repo.MustOwner(ctx), m.Repo, &repo_module.PushUpdateOptions{
			RefFullName: result.refName,
			OldCommitID: oldCommitID,
			NewCommitID: newCommitID,
		}, theCommits)
	}
	log.Trace("SyncMirrors [repo: %-v]: done notifying updated branches/tags - now updating last commit time", m.Repo)

	// Get latest commit date and update to current repository updated time
	commitDate, err := git.GetLatestCommitTime(ctx, m.Repo.RepoPath())
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: unable to GetLatestCommitDate: %v", m.Repo, err)
		return false
	}

	if err = repo_model.UpdateRepositoryUpdatedTime(ctx, m.RepoID, commitDate); err != nil {
		log.Error("SyncMirrors [repo: %-v]: unable to update repository 'updated_unix': %v", m.Repo, err)
		return false
	}

	log.Trace("SyncMirrors [repo: %-v]: Successfully updated", m.Repo)

	return true
}

func checkAndUpdateEmptyRepository(ctx context.Context, m *repo_model.Mirror, gitRepo *git.Repository, results []*mirrorSyncResult) bool {
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
		if !result.refName.IsBranch() {
			continue
		}

		name := result.refName.BranchName()
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
		if err := gitRepo.SetDefaultBranch(m.Repo.DefaultBranch); err != nil {
			if !git.IsErrUnsupportedVersion(err) {
				log.Error("Failed to update default branch of underlying git repository %-v. Error: %v", m.Repo, err)
				desc := fmt.Sprintf("Failed to update default branch of underlying git repository '%s': %v", m.Repo.RepoPath(), err)
				if err = system_model.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return false
			}
		}
		m.Repo.IsEmpty = false
		// Update the is empty and default_branch columns
		if err := repo_model.UpdateRepositoryCols(ctx, m.Repo, "default_branch", "is_empty"); err != nil {
			log.Error("Failed to update default branch of repository %-v. Error: %v", m.Repo, err)
			desc := fmt.Sprintf("Failed to update default branch of repository '%s': %v", m.Repo.RepoPath(), err)
			if err = system_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return false
		}
	}
	return true
}
