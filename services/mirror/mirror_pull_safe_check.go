// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	admin_model "code.gitea.io/gitea/models/admin"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// get git command running stdout and stderr
func getGitCommandStdoutStderr(ctx context.Context, m *repo_model.Mirror, gitArgs []string, newRepoPath string) (string, string, error) {
	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
		return "", "", remoteErr
	}

	if err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.getMirrorCanUpdate: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()
		sanitizer := util.NewURLSanitizer(remoteAddr, true)
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)
		log.Error("CreateRepositoryNotice: %v", err)
		log.Error("getGitCommandStdoutStderr [repo: %-v]: failed to check if mirror can be updated:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
		desc := fmt.Sprintf("Failed to check if mirror '%s' can be updated: %s", newRepoPath, stderrMessage)
		if err = admin_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	stdoutRepoCommitCount := stdoutBuilder.String()
	stderrRepoCommitCount := stdoutBuilder.String()
	stderrBuilder.Reset()
	stdoutBuilder.Reset()

	return stdoutRepoCommitCount, stderrRepoCommitCount, nil
}

// sync new repo mirror
func syncRepoMirror(ctx context.Context, m *repo_model.Mirror, gitArgs []string, newRepoPath string) (bool, error) {
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second
	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
	}
	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		})
	sanitizer := util.NewURLSanitizer(remoteAddr, true)
	var stdout, stderr string
	if err != nil {
		stdout = stdoutBuilder.String()
		stderr = stderrBuilder.String()

		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)

		// Now check if the error is a resolve reference due to broken reference
		if strings.Contains(stderr, "unable to resolve reference") && strings.Contains(stderr, "reference broken") {
			log.Warn("SyncMirrors [repo: %-v]: failed to update mirror repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
			err = nil

			// Attempt prune
			pruneErr := pruneBrokenReferences(ctx, m, newRepoPath, timeout, &stdoutBuilder, &stderrBuilder, sanitizer, false)
			if pruneErr == nil {
				// Successful prune - reattempt mirror
				stderrBuilder.Reset()
				stdoutBuilder.Reset()
				if err = git.NewCommand(ctx, gitArgs...).
					SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
					RunWithContext(&git.RunContext{
						Timeout: timeout,
						Dir:     newRepoPath,
						Stdout:  &stdoutBuilder,
						Stderr:  &stderrBuilder,
					}); err != nil {
					stdout := stdoutBuilder.String()
					stderr := stderrBuilder.String()

					// sanitize the output, since it may contain the remote address, which may
					// contain a password
					stderrMessage = sanitizer.Replace(stderr)
					stdoutMessage = sanitizer.Replace(stdout)
				}
			}
		}

		// If there is still an error (or there always was an error)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to update mirror repository:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", newRepoPath, stderrMessage)
			if err = admin_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return false, nil
		}
	}
	return false, nil
}

// detect user can update the mirror
func detectCanUpdateMirror(ctx context.Context, m *repo_model.Mirror, gitArgs []string) (bool, error) {
	repoPath := m.Repo.RepoPath()
	newRepoPath := fmt.Sprintf("%s_update", repoPath)

	// do copy directory recursive
	err := util.CopyDir(repoPath, newRepoPath)
	defer func() {
		// delete the temp directory
		errDelete := util.RemoveAll(newRepoPath)
		if errDelete != nil {
			log.Error("DeleteRepositoryTempDirectoryError: %v", errDelete)
		}
	}()
	if err != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: CopyDirectory Error %v", m.Repo, err)
		return false, err
	}
	syncStatus, err := syncRepoMirror(ctx, m, gitArgs, newRepoPath)
	if err != nil {
		return false, err
	}
	if !syncStatus {
		return false, nil
	}
	gitCommitCountArgs := []string{"rev-list", "HEAD", "--count"}
	stdoutNewRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, newRepoPath)
	if err != nil {
		return false, err
	}
	stdoutNewRepoCommitCount = strings.TrimSpace(stdoutNewRepoCommitCount)
	stdoutRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, repoPath)
	if err != nil {
		return false, err
	}
	stdoutRepoCommitCount = strings.TrimSpace(stdoutRepoCommitCount)
	var repoCommitCount, newRepoCommitCount int64
	if i, err := strconv.ParseInt(stdoutRepoCommitCount, 10, 64); err == nil {
		repoCommitCount = i
	} else {
		return false, err
	}
	if i, err := strconv.ParseInt(stdoutNewRepoCommitCount, 10, 64); err == nil {
		newRepoCommitCount = i
	} else {
		return false, err
	}
	if repoCommitCount > newRepoCommitCount {
		return false, nil
	} else if repoCommitCount == newRepoCommitCount {
		// noting to happen
		return true, nil
	} else {
		// compare commit id
		skipcout := newRepoCommitCount - repoCommitCount
		gitNewRepoLastCommitIDArgs := []string{"log", "-1", fmt.Sprintf("--skip=%d", skipcout), "--format='%H'"}
		stdoutNewRepoCommitID, _, err := getGitCommandStdoutStderr(ctx, m, gitNewRepoLastCommitIDArgs, newRepoPath)
		if err != nil {
			return false, err
		}
		gitRepoLastCommitIDArgs := []string{"log", "--format='%H'", "-n", "1"}
		stdoutRepoCommitID, _, err := getGitCommandStdoutStderr(ctx, m, gitRepoLastCommitIDArgs, repoPath)
		if err != nil {
			return false, err
		}
		if stdoutNewRepoCommitID != stdoutRepoCommitID {
			return false, fmt.Errorf("Old repo commit id: %s not match new repo id: %s", stdoutRepoCommitID, stdoutNewRepoCommitID)
		}
	}
	return true, nil
}
